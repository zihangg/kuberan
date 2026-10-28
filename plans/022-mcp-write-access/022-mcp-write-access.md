# Plan 022 — MCP Write Access

**Status: implemented** (all phases). Where the code differs from the plan below:

- **Amounts are in dollars**, not cents. The read tools' amount filters already take
  dollars, and the model is less likely to get dollars wrong. Tools convert to cents with
  `dollarsToCents`. `MCP_MAX_WRITE_AMOUNT` is still in cents.
- **`categorize_transactions` takes at most 25 IDs**, not 50, so a full batch always fits
  the default 30-per-minute rate limit. Each ID counts as one write.
- **Category ownership**: implementing this surfaced that the transaction service never
  checked that a `category_id` belongs to the user (REST was affected too). That's now
  fixed in the service (`ensureCategoryOwned`). The MCP tools additionally reject a
  category whose type doesn't match the transaction type.
- **Disconnect calls Hydra first, then revokes locally**, so a Hydra failure leaves
  everything unchanged and the user can retry.
- **Files:** write tools live in `internal/mcp/tools_*_write.go`, with the guard, argument
  helpers and rate limiter in `internal/mcp/write.go`. The connected-apps endpoints are
  methods on `OAuthHandler` (`internal/handlers/oauth_clients_handler.go`). The settings UI
  is a new "Connected apps" tab.
- **After review** (five reviewers on the finished branch), all findings were fixed:
  - Every write tool now runs through one wrapper, `addWriteTool`: grant check, then
    argument validation, then the rate limit (charged only for valid writes, by records
    changed), then the write and its audit rows.
  - The ceiling now also covers implicit amounts: a full write-off, and the existing
    amount of a transaction whose type is flipped.
  - Creates require the user's local `date`. Dates must be within 10 years back.
  - Text limits count characters, as REST does.
  - Disconnect also revokes Hydra-only (pre-grant) connections, and Connected apps
    lists them.
  - `OAUTH_SCOPES` can't add `write:*` while the flag is off.
  - Audit IPs trust `CF-Connecting-IP` only from `MCP_TRUSTED_PROXY_CIDRS`.
  - Long client names are truncated.
  - Consent recording is a single upsert.
  - The web app sends anti-framing headers.
- **Follow-ups**, also done:
  - `AppError` has a code-based `Is`, so `errors.Is` is used everywhere.
  - `OAUTH_CLIENT_TRUSTED` folded into `OAUTH_CONSENT_GRANTED` (`auto_accept`).
  - Text limits moved into the services.
  - The MCP server depends on a narrow `GrantAuthorizer`.
  - A type flip clears a deleted category.
  - A new consent merges with the client's earlier choices.
  - The runbook (section 6) has the cross-user category check.
- **Verified** with unit, handler and integration tests (SQLite), and a local smoke run
  against Postgres 16 with the real `api` and `mcp` binaries and a stub JWKS/admin server
  standing in for Hydra. The real-Claude E2E in Phase 5 is still to be done on a
  deployment.

## Context

The MCP server (`apps/api/cmd/mcp`, `internal/mcp/`) is read-only. Plan 015 chose that on purpose
(decision **Q6: granular `read:*`**), and later plans kept it (plan 021's "MCP and BOT (read-only)"
section). One config value enforces it in four places:

| Layer | Where | What it does |
|---|---|---|
| Scope set | `config.DefaultOAuthScopes` (`internal/config/config.go:72`), `OAUTH_SCOPES` | Lists the API scopes that exist |
| DCR proxy | `handlers/registration_handler.go:96` (`capScopes`) | A self-registering client can register only these scopes |
| Consent bridge | `handlers/oauth_handler.go` (`grantableScopes`) | A grant can never exceed these scopes |
| Resource server | `requireScope` (`internal/mcp/server.go:63`), called by every tool | Each tool needs its `read:*` scope |

Adding write tools is mechanically simple: the service layer already has user-scoped, transactional
create/update methods. The work is in the parts of the auth design that are fine for reads and
unsafe for writes:

1. **Trusted clients silently get any new scope.** `GetConsent` auto-accepts a trusted client with
   whatever it requests, capped only by the global allowed set. `trusted_oauth_clients` stores only
   `client_id`: no user, no approved scopes (migration `trusted_oauth_clients`,
   `trusted_client_service.go:23`). Adding `write:*` to the allowed set would give write access to
   every trusted client on its next authorization, with no consent screen.
2. **There's no revocation.** Tokens are checked offline against Hydra's JWKS (15-minute access
   tokens, rotating refresh tokens with a 90-day idle timeout). `hydra.AdminClient` has no call to
   revoke consent or tokens, and no UI lists or disconnects clients.
3. **MCP calls aren't audited.** Audit logging lives in the REST handlers
   (`transaction_handler.go:98`), not the services, so service calls from MCP leave no audit row.
   `AccessClaims` also doesn't carry `client_id`, so a write couldn't be traced to a client anyway.
4. **Account deactivation is only checked at login.** `IsActive` is checked when a token is
   issued, not on each call.

There's also a risk that exists only because the client is an AI: **prompt injection**. The model
reads text the user didn't write (transaction descriptions, receipt OCR, counterparty names,
anything else in the conversation), and planted text could steer it into calling a write tool.

## Goal

Let a connected MCP client (Claude) **create and edit** the user's records, opt in per client:

- Write access is **never granted implicitly**. Every write scope needs an explicit opt-in on the
  consent screen, even for a client that is already trusted.
- The user can **see and disconnect** connected apps. Disconnecting blocks writes **immediately**
  and reads within one access-token lifetime (≤15 min).
- Every MCP write is **audited** with the client that made it.
- The first tool set is **additive or reversible**. Deletes and investment trades stay out of v1.
- A single env flag turns the whole feature off, as a rollback.

## Decisions (for review)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | Write scope granularity | **Per domain:** `write:transactions`, `write:categories`, `write:budgets` | Same shape as `read:*`. Receivable writes (repayments, write-offs) move money through transactions, so they use `write:transactions`, as `read:transactions` covers receivable reads. |
| D2 | Feature flag | **`MCP_WRITE_ENABLED`, default `false`** | When off, write scopes aren't advertised, registrable or grantable, and write tools aren't registered. Rollback is an env flip with no code change. |
| D3 | Consent model | **Per-user, per-scope grants** replacing `trusted_oauth_clients` | Fixes problem 1. A trusted client is auto-accepted only for scopes the user has already reviewed. A newly requested scope brings the consent screen back. |
| D4 | Write scopes on the consent screen | **Separate "Allow changes" section, unchecked by default** | Clients usually request everything in `scopes_supported`. Read access works as today; write access needs a deliberate click. |
| D5 | Instant revocation for writes | **Each write tool re-checks the grant row in the DB** | One indexed query per write: the grant must exist, not be revoked, include the write scope, and the user must be active. Revocation and deactivation then block writes immediately while reads stay offline. Fixes problems 2 and 4 for writes. |
| D6 | Revocation mechanism | **Hydra admin `DELETE /admin/oauth2/auth/sessions/consent?subject=&client=` plus marking the grant row revoked** | Hydra revokes the consent and its refresh tokens. Any access JWT still out there stops working for writes at once (D5) and for reads within 15 minutes. |
| D7 | Audit | **Every MCP write is logged** as `MCP_<ACTION>`, with `client_id`, the tool name and the key arguments | Same `audit_logs` table and `AuditServicer` as REST. MCP gets an `Audit` service and the request IP. Fixes problem 3. |
| D8 | Rate limiting | **In-process, per (user, client): 30 writes/min and 500/day** (configurable) | Caps damage from a looping or hijacked model. The MCP server runs as a single instance, so in-memory is enough. |
| D9 | Input bounds | Amount `> 0` and `≤ MCP_MAX_WRITE_AMOUNT` (default 1,000,000 cents); date no more than 7 days ahead; description ≤ 500 characters; IDs only, no name lookups | Same limits as the REST handlers, plus a ceiling. Accepting IDs only avoids ambiguous "which account?" guesses. |
| D10 | Tool annotations | Read tools: `readOnlyHint: true`. Create tools: `readOnlyHint: false, destructiveHint: false`. Update and write-off tools: `destructiveHint: true` | Clients like Claude use these hints to decide when to ask the user before running a tool. |
| D11 | v1 tool scope | **Tier 1** (additive) and **Tier 2** (edits, reversible). No deletes, investments, accounts or fixed deposits | Limits the damage a prompt injection can do, and anything v1 does can be undone in the web UI. |
| D12 | Existing trust rows | **Dropped by the migration** (users see the consent screen once more) | The old rows have no `user_id` or scopes to carry over. Re-consenting once is cheap, and it's the safe default. |

## Threat model

| Threat | Mitigation |
|---|---|
| A trusted client silently gains write access | D3, D4: grants are per scope. A new scope means a new consent screen, and write scopes start unchecked |
| A malicious client registers itself through DCR and phishes consent | Existing controls stay in place (PKCE, scope cap, new-client alert, consent screen showing the redirect host). Write access also needs a deliberate extra click (D4) |
| Prompt injection makes the model write | Annotations make the client ask the user (D10). No deletes in v1 (D11). Bounds (D9), rate limits (D8), audit trail (D7), one-click disconnect (D6) |
| A stolen access token is used to write | It lasts ≤15 min. Disconnecting blocks writes immediately (D5). Rate limits (D8) |
| A model retry creates duplicate transactions | Tool output includes the new ID and the resulting balance, and the tool description tells the model to check with `list_transactions` before retrying. Idempotency keys are a follow-up |
| A deactivated user keeps write access | Write tools check `IsActive` with the grant (D5) |
| Cross-user IDs (writing to another user's account) | The services already scope every query by `user_id`. Each tool gets a test passing another user's ID |

## Phase 1: Per-user, scope-aware grants and revocation (no write tools yet)

This phase ships on its own and improves today's read-only setup: users get a list of connected
apps and a disconnect button.

1. **Migration `000033_create_oauth_client_grants`**

   ```sql
   CREATE TABLE oauth_client_grants (
       id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
       created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
       updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
       deleted_at TIMESTAMPTZ,
       user_id UUID NOT NULL REFERENCES users(id),
       client_id VARCHAR(255) NOT NULL,
       client_name VARCHAR(255) NOT NULL DEFAULT '',
       approved_scopes TEXT NOT NULL DEFAULT '',   -- space-delimited, granted
       reviewed_scopes TEXT NOT NULL DEFAULT '',   -- space-delimited, ever shown to the user
       auto_accept BOOLEAN NOT NULL DEFAULT FALSE, -- the "remember this application" box
       last_used_at TIMESTAMPTZ,
       revoked_at TIMESTAMPTZ
   );
   CREATE UNIQUE INDEX idx_oauth_client_grants_user_client
       ON oauth_client_grants (user_id, client_id) WHERE deleted_at IS NULL;
   DROP TABLE trusted_oauth_clients;  -- D12
   ```

   The down migration recreates `trusted_oauth_clients` with its old schema, empty, and drops the
   new table.

2. **Model and service.** Add `models.OAuthClientGrant` and replace `TrustedClientServicer` with
   `OAuthGrantServicer`:
   - `Get(userID, clientID) (*OAuthClientGrant, error)`
   - `RecordConsent(userID, clientID, name string, approved, reviewed []string, autoAccept bool)`:
     upserts the row and clears `revoked_at`.
   - `List(userID) ([]OAuthClientGrant, error)`: active grants only.
   - `Revoke(userID, clientID) error`: sets `revoked_at` and `auto_accept=false`.
   - `Authorize(userID, clientID, scope string) error`: used by D5. Fails if the row is missing or
     revoked, if `scope` isn't in `approved_scopes`, or if the user is inactive. Updates
     `last_used_at`, throttled to at most one write per minute.

   Remove `trusted_client_service.go` and its tests. Rewire `cmd/api/main.go`.

3. **Consent bridge changes** (`handlers/oauth_handler.go`):
   - `GetConsent`: look up the grant by `(consent.Subject, client_id)`. Auto-accept only if
     `auto_accept`, not revoked, and every requested scope is in `reviewed_scopes`. In that case
     grant `requested ∩ approved_scopes`. Otherwise return the consent details. Add
     `previously_approved` so the page can pre-check those boxes.
   - `AcceptConsent`: the request gains `granted_scopes []string`. The server grants
     `granted_scopes ∩ grantableScopes(requested)`, adding protocol scopes and `offline_access` as
     today, then calls `RecordConsent` with `reviewed = grantableScopes(requested)` and
     `autoAccept = remember_client`. If `granted_scopes` is missing, grant only the requested
     `read:*` scopes, so an old page can never grant write access.
   - Record a grant on every accept, not just when "remember" is ticked. D5 needs a row to check.
   - Audit `OAUTH_CONSENT_GRANTED` with the granted scopes (superseded: one event now also records `auto_accept`; `OAUTH_CLIENT_TRUSTED` is gone), and keep `OAUTH_CLIENT_TRUSTED` when
     `remember` is set.

4. **Hydra admin client** (`internal/hydra/`): add
   `RevokeConsentSessions(ctx, subject, clientID string) error` calling
   `DELETE /admin/oauth2/auth/sessions/consent?subject=…&client=…`. Map 404 to success
   (idempotent).

5. **Connected apps API** (protected routes, `cmd/api/main.go`):
   - `GET /api/v1/oauth/clients`: the user's active grants (name, client_id, approved scopes,
     created, last used).
   - `DELETE /api/v1/oauth/clients/:client_id`: calls Hydra revoke, then `Revoke` on the grant, and
     audits `OAUTH_CLIENT_REVOKED`. If Hydra fails, still revoke locally and return an error asking
     the user to retry. The local revoke already blocks writes through D5.
   - Regenerate Swagger.

6. **Web**:
   - `components/settings/connected-apps-settings.tsx` on the settings page: a list with scope
     badges, last used, and a "Disconnect" button with a confirmation dialog. Add a
     `use-oauth-clients.ts` hook with a query key factory and the matching `api-client.ts`
     methods.
   - The consent page gets per-scope checkboxes. It's still read-only in this phase, so the read
     scopes start checked. It sends `granted_scopes`.

7. **Docs**: update `docs/mcp-oauth.md` (the "Components" table, the TOFU description, a new
   "Revoking a client" section) and the scope list in `CLAUDE.md`.

**Verify:** the full check suite passes. In the browser:
- Connect Claude, untick one read scope on the consent screen, and confirm the matching tool
  returns "missing required scope".
- Reconnect with "remember" ticked: the consent screen is skipped.
- Disconnect from Settings: the next refresh fails, and tools stop working within 15 minutes.

## Phase 2: Write infrastructure (behind `MCP_WRITE_ENABLED=false`)

1. **Config** (`internal/config/config.go`):
   - Add `DefaultOAuthWriteScopes = "write:transactions write:categories write:budgets"`, plus
     `MCPWriteEnabled` (`MCP_WRITE_ENABLED`), `MCPMaxWriteAmount`, `MCPWriteRatePerMinute` and
     `MCPWriteRatePerDay`.
   - Add `OAuthScopeSet()`, which returns read scopes plus write scopes when enabled. Use it in the
     DCR proxy, the consent bridge and discovery instead of the raw `OAuthScopes` string.
   - `OAUTH_SCOPES` still overrides the read set only.

2. **Validator** (`internal/mcp/validator.go`): read the `client_id` claim (Hydra puts it in JWT
   access tokens) into `AccessClaims.ClientID`. Put it in the request context next to the user ID
   and scopes. Add a validator test with a Hydra-shaped token.

3. **Request IP**: `makeAuthFromRequest` stores the client IP in the context: the
   `CF-Connecting-IP` header if present (we run behind the Cloudflare tunnel), else `RemoteAddr`.

4. **Write guard** (`internal/mcp/write.go`): a single entry point for every write tool.

   ```go
   // authorizeWrite resolves the caller and enforces, in order: the token scope,
   // the DB grant (not revoked, scope approved, user active; D5), and the
   // per-(user, client) rate limit (D8). It returns the caller or an error result.
   func (s *Server) authorizeWrite(ctx context.Context, scope string) (caller, *mcp.CallToolResult)
   ```

   - `caller` holds `UserID`, `ClientID` and `IP`.
   - The guard sits on top of `requireScope`: the token must carry the scope **and** the DB grant
     must still include it.
   - `s.audit(ctx, c, action, resourceType, resourceID, changes)` wraps `AuditServicer.Log`. It
     always adds `client_id`, `tool` and `via: "mcp"` to `changes`.
   - The rate limiter is a small fixed-window counter keyed by `userID|clientID` behind a mutex,
     with a minute bucket and a day bucket. That avoids a new dependency. When the limit is hit,
     return "rate limited: try again in Ns".

5. **Services wiring**: add `Audit services.AuditServicer` and `Grants services.OAuthGrantServicer`
   to `mcp.Services`, wired in `cmd/mcp/main.go`. `Run` gets the write config. Write tools are
   registered only when `MCPWriteEnabled` is set.

6. **Annotations on existing tools**: add `readOnlyHint: true` (with `idempotentHint: true` and
   `openWorldHint: false`) to every read tool, so clients can tell reads from writes. Use mcp-go's
   annotation options; confirm the exact option names against v0.47.1 in `go.mod`.

7. **Consent page**: add an "Allow changes" section with labels such as "Add and edit
   transactions and repayments", "Add and edit categories" and "Add and edit budgets". The boxes
   start unchecked unless that scope is in `previously_approved`. Add a warning line: "Claude
   will be able to change your financial records. You can disconnect it in Settings at any
   time."

8. **Discovery**: `scopes_supported` comes from `OAuthScopeSet()`, so write scopes appear only
   when the flag is on.

**Verify:** the full check suite passes. Unit tests for `authorizeWrite`:
- no token scope
- token scope present but the grant is revoked
- scope not approved in the grant
- user inactive
- rate limit at N+1
- success, which audits the call

With the flag off, the discovery output and consent page are byte-identical to the end of
Phase 1.

## Phase 3: Tier 1 tools (additive)

Each tool lives in its domain's `tools_*.go` file. It calls `authorizeWrite`, validates against
D9, calls the existing service method, audits, and returns the result in the same text format as
the read tools, including the new ID and the resulting account balance.

| Tool | Scope | Service call | Notes |
|---|---|---|---|
| `create_transaction` | `write:transactions` | `Transactions.CreateTransaction` | `type` ∈ `income`, `expense`. Category rules still apply (the service runs `resolveRuleCategory`). Amount is in **cents**, and the description says so. |
| `create_transfer` | `write:transactions` | `Transactions.CreateTransfer` | The two accounts must differ. |
| `create_category` | `write:categories` | `Categories.CreateCategory` | `type`, plus an optional parent, icon and color. |
| `create_budget` | `write:budgets` | `Budgets.CreateBudget` | `period` validated against `models.BudgetPeriod`. |
| `record_repayment` | `write:transactions` | `Receivables.Settle` | `receivable_id`, `account_id`, `amount`, optional date and description. `write_off_remainder` defaults to false. |

Every tool description ends with: *"This changes the user's financial records. Confirm the
details with the user before calling. If a call may have failed, check with list_transactions
before retrying."*

Audit actions: `MCP_CREATE_TRANSACTION`, `MCP_CREATE_TRANSFER`, `MCP_CREATE_CATEGORY`,
`MCP_CREATE_BUDGET` and `MCP_RECORD_REPAYMENT`.

**Tests** (`tools_write_test.go`, using the same SQLite setup as `tools_receivables_test.go`). For
each tool:
- missing scope → forbidden
- revoked grant → forbidden
- another user's account/category ID → not found, with no row created
- amount 0, negative, or above the ceiling → invalid
- success → the row exists, the balance changed correctly (including the inverted credit card
  case for `create_transaction`), and exactly one audit row carries `client_id`

## Phase 4: Tier 2 tools (edits, reversible in the UI)

| Tool | Scope | Service call | Annotation |
|---|---|---|---|
| `update_transaction` | `write:transactions` | `Transactions.UpdateTransaction` | destructive |
| `categorize_transactions` | `write:transactions` | `UpdateTransaction` in a loop, at most 50 IDs | destructive. Each ID counts toward the rate limit, and each is audited. |
| `update_category` | `write:categories` | `Categories.UpdateCategory` | destructive |
| `update_budget` | `write:budgets` | `Budgets.UpdateBudget` (including pause/resume through `is_active`) | destructive |
| `write_off_receivable` | `write:transactions` | `Receivables.WriteOff` | destructive |

- `update_transaction` accepts only the fields being changed. To keep D9 bounds simple, it doesn't
  allow moving a transaction to a different account (`AccountID`) in v1.
- Updates audit the before and after values of every changed field.
- Tests follow the Phase 3 pattern, plus "a partial update leaves other fields alone".

## Phase 5: Rollout

1. Deploy Phases 1–4 with `MCP_WRITE_ENABLED=false`. Check that read-only behavior is unchanged,
   then use the "Verifying a deployment" curl checks in `docs/mcp-oauth.md`.
2. Set `MCP_WRITE_ENABLED=true` in `.env.prod` and restart `api` and `mcp`.
3. **Re-add the connector.** Existing clients registered through DCR with only `read:*`, and Hydra
   rejects an authorize request for scopes outside a client's registered set. Remove and re-add
   the connector in Claude so it registers again. This is the same step the `offline_access`
   change needed, as noted in `docs/mcp-oauth.md`. Document it in the runbook.
4. End-to-end test with the real Claude client, following `PHASE-3-5-RUNBOOK.md`:
   - Connect without ticking any write scope: write tools return forbidden.
   - Reconnect and tick `write:transactions`: Claude asks before calling `create_transaction`, the
     transaction appears in the web UI, and the audit row has the client ID.
   - Disconnect in Settings: the next write fails immediately.
   - Wait out the access-token lifetime: reads fail and refresh is rejected.
5. Rollback: set `MCP_WRITE_ENABLED=false`. Write tools disappear on restart. Grants keep their
   write scopes, but no tool accepts them.

## Out of scope (follow-ups)

- **Deletes** (`delete_transaction`, `delete_category`, deleting settlements and budgets). These
  should come only after v1 has been used for a while, possibly behind a separate
  `delete:transactions` scope.
- **Investments** (buy, sell, dividend, split), **accounts**, and **fixed deposits**: valuations
  and cost basis get complicated, and these are rarely done conversationally.
- **Idempotency keys** for create tools: an optional `idempotency_key` with a short-lived dedupe
  table.
- **Provenance on records**: a `source` column (`web`, `bot`, `mcp`) on transactions, so the UI
  can show and filter what Claude created. For now the audit log answers this.
- **Per-call introspection for reads**: reads still trust the JWT for up to 15 minutes after a
  disconnect.

## Files touched (summary)

- **API**:
  - `migrations/000033_*`
  - `internal/models/oauth_client_grant.go`
  - `internal/services/{interfaces.go, oauth_grant_service.go}` (delete `trusted_client_service.go`)
  - `internal/hydra/{client.go, types.go}`
  - `internal/handlers/{oauth_handler.go, registration_handler.go, oauth_clients_handler.go}`
  - `internal/config/config.go`
  - `internal/mcp/{server.go, validator.go, discovery.go, write.go, tools_*.go}`
  - `cmd/api/main.go`, `cmd/mcp/main.go`
  - Swagger docs
- **Web**:
  - `app/(oauth)/oauth/consent/page.tsx`
  - `app/(dashboard)/settings/page.tsx`
  - `components/settings/connected-apps-settings.tsx`
  - `hooks/use-oauth-clients.ts`
  - `lib/api-client.ts`
  - `types/{api,models}.ts`
- **Ops and docs**:
  - `.env.prod.example`
  - `docker-compose*.yml` (pass `MCP_WRITE_ENABLED` and the limits to `api` and `mcp`)
  - `docs/mcp-oauth.md`
  - `plans/015-mcp-oauth-hydra/PHASE-3-5-RUNBOOK.md`
  - `CLAUDE.md` (scopes, endpoints, env vars)
