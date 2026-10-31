// OAuth scope helpers shared by the consent page and Connected apps settings
// (plans 015 and 022).

// Human-readable labels for the granular scopes Hydra may request.
const SCOPE_LABELS: Record<string, string> = {
  "read:accounts": "View your accounts",
  "read:transactions": "View your transactions",
  "read:budgets": "View your budgets",
  "read:categories": "View your categories",
  "read:investments": "View your investments",
  "read:portfolio": "View your portfolio",
  "read:snapshots": "View your net-worth history",
  "write:transactions": "Add and edit transactions, transfers and repayments",
  "write:categories": "Add and edit categories",
  "write:budgets": "Add and edit budgets",
  openid: "Verify your identity",
  offline: "Maintain access when you're away",
  offline_access: "Maintain access when you're away",
};

// OIDC / refresh scopes: always granted, never user-selectable.
// Keep in sync with protocolScopes in apps/api/internal/handlers/oauth_handler.go.
const PROTOCOL_SCOPES = new Set(["openid", "offline", "offline_access"]);

export function scopeLabel(scope: string): string {
  return SCOPE_LABELS[scope] ?? scope;
}

export function isProtocolScope(scope: string): boolean {
  return PROTOCOL_SCOPES.has(scope);
}

// Write scopes let a client change the user's data and always need an
// explicit opt-in. Keep in sync with writeScopePrefix in
// apps/api/internal/handlers/oauth_handler.go.
export function isWriteScope(scope: string): boolean {
  return scope.startsWith("write:");
}
