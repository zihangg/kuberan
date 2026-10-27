package mcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/logger"
)

// WriteConfig controls the MCP write tools (plan 022).
type WriteConfig struct {
	// Enabled registers the write tools. When false the server is read-only.
	Enabled bool
	// MaxAmount is the per-call ceiling for any amount, in cents.
	MaxAmount int64
	// RatePerMinute and RatePerDay cap writes per (user, client); 0 disables
	// that window.
	RatePerMinute int
	RatePerDay    int
}

// maxFutureDays bounds how far ahead a write may date a record.
const maxFutureDays = 7

// writeNotice ends every write tool description. Clients surface tool
// descriptions to the model; this steers it to confirm with the user and to
// check before retrying, which matters because a create is not idempotent.
const writeNotice = " This changes the user's financial records: confirm the details with the user before calling. " +
	"If a call may have failed, check with list_transactions before retrying."

// caller identifies who is making an MCP write, for the grant check, the rate
// limit and the audit log.
type caller struct {
	UserID   string
	ClientID string
	IP       string
}

// authorizeWrite resolves the caller and enforces, in order: the token scope,
// the user's consent grant in the database (not revoked, scope approved, user
// active), and the per-(user, client) rate limit, counting n writes. It returns
// the caller, or an MCP error result the handler should return directly.
//
// The grant check is what makes revocation immediate for writes: access
// tokens are validated offline and stay valid for up to 15 minutes after the
// user disconnects the client in Settings.
func (s *Server) authorizeWrite(ctx context.Context, scope string, n int) (caller, *mcp.CallToolResult) {
	userID, err := getUserID(ctx)
	if err != nil {
		return caller{}, errUnauthorized()
	}
	if denied := requireScope(ctx, scope); denied != nil {
		return caller{}, denied
	}

	c := caller{UserID: userID}
	c.ClientID, _ = ctx.Value(clientIDKey{}).(string)
	c.IP, _ = ctx.Value(clientIPKey{}).(string)
	if c.ClientID == "" {
		return caller{}, mcp.NewToolResultError("forbidden: access token has no client_id")
	}

	if err := s.services.Grants.Authorize(userID, c.ClientID, scope); err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.ErrOAuthScopeNotGranted.Code {
			return caller{}, mcp.NewToolResultError(fmt.Sprintf(
				"forbidden: this connection is not allowed to make changes (%s). "+
					"Reconnect and allow changes, or check Settings > Connected apps in Kuberan.", scope))
		}
		logger.Get().Errorf("MCP write: grant check failed: %v", err)
		return caller{}, mcp.NewToolResultError("failed to verify access")
	}

	if ok, retry := s.limiter.allow(userID+"|"+c.ClientID, n); !ok {
		return caller{}, mcp.NewToolResultError(fmt.Sprintf(
			"rate limited: too many changes, try again in %ds", int(math.Ceil(retry.Seconds()))))
	}
	return c, nil
}

// audit records an MCP write in audit_logs, tagged with the OAuth client and
// tool so every change can be traced to the connection that made it.
func (s *Server) audit(c caller, tool, action, resourceType, resourceID string, changes map[string]interface{}) {
	if changes == nil {
		changes = map[string]interface{}{}
	}
	changes["via"] = "mcp"
	changes["client_id"] = c.ClientID
	changes["tool"] = tool
	s.services.Audit.Log(c.UserID, action, resourceType, resourceID, c.IP, changes)
}

// serviceErrorMessage returns a client-safe description of a service error.
// AppError messages are written for clients (the REST API returns them as-is);
// anything else is logged and reported generically.
func serviceErrorMessage(err error) string {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.StatusCode < 500 {
		return appErr.Message
	}
	logger.Get().Errorf("MCP write failed: %v", err)
	return "internal error"
}

// serviceError turns a service error into a tool error for action.
func serviceError(action string, err error) *mcp.CallToolResult {
	return mcp.NewToolResultError("failed to " + action + ": " + serviceErrorMessage(err))
}

// writeTool returns the options shared by every write tool: the description
// with the confirmation notice, and annotations telling clients the tool
// changes data. destructive marks tools that overwrite existing values.
func writeTool(name, description string, destructive bool, opts ...mcp.ToolOption) mcp.Tool {
	all := append([]mcp.ToolOption{
		mcp.WithDescription(description + writeNotice),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(destructive),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	}, opts...)
	return mcp.NewTool(name, all...)
}

// readTool returns a tool annotated as read-only, so clients can tell reads
// from writes and skip confirmation for them.
func readTool(name string, opts ...mcp.ToolOption) mcp.Tool {
	all := append([]mcp.ToolOption{
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	}, opts...)
	return mcp.NewTool(name, all...)
}

// stringArg returns a trimmed string argument, or "" when absent.
func stringArg(args map[string]interface{}, name string) string {
	v, _ := args[name].(string)
	return strings.TrimSpace(v)
}

// requiredString returns a non-empty string argument or an error result.
func requiredString(args map[string]interface{}, name string) (string, *mcp.CallToolResult) {
	v := stringArg(args, name)
	if v == "" {
		return "", mcp.NewToolResultError(name + " is required")
	}
	return v, nil
}

// optionalString returns a pointer to a string argument when it was supplied
// (even if empty, which callers use to clear a value), or nil.
func optionalString(args map[string]interface{}, name string) *string {
	v, ok := args[name].(string)
	if !ok {
		return nil
	}
	v = strings.TrimSpace(v)
	return &v
}

// maxDescriptionLen matches the REST API's binding:"max=500".
const maxDescriptionLen = 500

// descriptionArg returns the description argument, bounded like the REST API.
func descriptionArg(args map[string]interface{}) (string, *mcp.CallToolResult) {
	v := stringArg(args, "description")
	if len(v) > maxDescriptionLen {
		return "", mcp.NewToolResultError(fmt.Sprintf("description must be at most %d characters", maxDescriptionLen))
	}
	return v, nil
}

// amountArg reads the "amount" argument (dollars) and returns it in cents. It
// must be positive and at most the configured ceiling. present is false when
// the argument was omitted.
func (s *Server) amountArg(args map[string]interface{}) (cents int64, present bool, res *mcp.CallToolResult) {
	raw, ok := args["amount"]
	if !ok || raw == nil {
		return 0, false, nil
	}
	dollars, ok := raw.(float64)
	if !ok || math.IsNaN(dollars) || math.IsInf(dollars, 0) {
		return 0, true, mcp.NewToolResultError("amount must be a number of dollars")
	}
	cents = dollarsToCents(dollars)
	if cents <= 0 {
		return 0, true, mcp.NewToolResultError("amount must be greater than zero")
	}
	if s.write.MaxAmount > 0 && cents > s.write.MaxAmount {
		return 0, true, mcp.NewToolResultError(fmt.Sprintf(
			"amount exceeds the per-change limit of %s; record it in the Kuberan app instead",
			formatCents(s.write.MaxAmount)))
	}
	return cents, true, nil
}

// requiredAmount reads the required "amount" argument (dollars), in cents.
func (s *Server) requiredAmount(args map[string]interface{}) (int64, *mcp.CallToolResult) {
	cents, present, res := s.amountArg(args)
	if res != nil {
		return 0, res
	}
	if !present {
		return 0, mcp.NewToolResultError("amount is required")
	}
	return cents, nil
}

// dateArg reads the optional "date" argument (YYYY-MM-DD) as UTC midnight, the
// same instant the web app sends for a date. It rejects dates more than
// maxFutureDays ahead. present is false when the argument was omitted.
func dateArg(args map[string]interface{}, now time.Time) (t time.Time, present bool, res *mcp.CallToolResult) {
	v := stringArg(args, "date")
	if v == "" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return time.Time{}, true, mcp.NewToolResultError("date must be in YYYY-MM-DD format")
	}
	if t.After(now.AddDate(0, 0, maxFutureDays)) {
		return time.Time{}, true, mcp.NewToolResultError(fmt.Sprintf("date cannot be more than %d days in the future", maxFutureDays))
	}
	return t, true, nil
}

// rateLimiter is a fixed-window counter per key, with a one-minute and a
// one-day window. The MCP server runs as a single instance, so in-memory state
// is enough; a restart resets the windows.
type rateLimiter struct {
	mu        sync.Mutex
	perMinute int
	perDay    int
	now       func() time.Time
	buckets   map[string]*rateBucket
}

type rateBucket struct {
	minuteStart time.Time
	minuteCount int
	dayStart    time.Time
	dayCount    int
}

func newRateLimiter(perMinute, perDay int, now func() time.Time) *rateLimiter {
	return &rateLimiter{perMinute: perMinute, perDay: perDay, now: now, buckets: map[string]*rateBucket{}}
}

// allow counts n events for key and reports whether they fit in both windows.
// When they do not, nothing is counted and retry is how long until the
// blocking window resets.
func (l *rateLimiter) allow(key string, n int) (ok bool, retry time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b := l.buckets[key]
	if b == nil {
		b = &rateBucket{minuteStart: now, dayStart: now}
		l.buckets[key] = b
	}
	if now.Sub(b.minuteStart) >= time.Minute {
		b.minuteStart, b.minuteCount = now, 0
	}
	if now.Sub(b.dayStart) >= 24*time.Hour {
		b.dayStart, b.dayCount = now, 0
	}

	if l.perDay > 0 && b.dayCount+n > l.perDay {
		return false, b.dayStart.Add(24 * time.Hour).Sub(now)
	}
	if l.perMinute > 0 && b.minuteCount+n > l.perMinute {
		return false, b.minuteStart.Add(time.Minute).Sub(now)
	}
	b.minuteCount += n
	b.dayCount += n
	return true, 0
}
