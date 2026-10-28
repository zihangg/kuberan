package handlers

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/logger"
	"kuberan/internal/models"
)

// ConnectedAppResponse is one OAuth client the user has granted access to.
// Hydra-only entries (Legacy) are clients whose access predates per-scope
// grants (plan 022): Hydra still holds their consent, but Kuberan has no
// grant row, so they can read but never write. CreatedAt and LastUsedAt are
// unknown for them.
type ConnectedAppResponse struct {
	ClientID   string     `json:"client_id"`
	ClientName string     `json:"client_name"`
	Scopes     []string   `json:"scopes"`
	AutoAccept bool       `json:"auto_accept"`
	Legacy     bool       `json:"legacy,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// ConnectedAppsResponse wraps the user's connected apps.
type ConnectedAppsResponse struct {
	Clients []ConnectedAppResponse `json:"clients"`
}

// ListConnectedApps returns the OAuth clients (e.g. Claude connectors) the
// authenticated user has granted access to: their grants, plus any client
// Hydra still holds a consent session for without an active grant (one
// connected before grants existed), so every connection can be disconnected.
// If Hydra cannot be reached the grants alone are returned.
//
// @Summary     List connected apps
// @Description List the OAuth clients the user has granted access to, with their scopes
// @Tags        oauth
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} ConnectedAppsResponse
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Router      /oauth/clients [get]
func (h *OAuthHandler) ListConnectedApps(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	grants, err := h.grants.List(userID)
	if err != nil {
		respondWithError(c, err)
		return
	}

	out := make([]ConnectedAppResponse, 0, len(grants))
	listed := make(map[string]int, len(grants))
	for i := range grants {
		g := &grants[i]
		listed[g.ClientID] = len(out)
		out = append(out, ConnectedAppResponse{
			ClientID:   g.ClientID,
			ClientName: g.ClientName,
			Scopes:     g.Approved(),
			AutoAccept: g.AutoAccept,
			CreatedAt:  &g.CreatedAt,
			LastUsedAt: g.LastUsedAt,
		})
	}

	sessions, err := h.hydra.ListConsentSessions(c.Request.Context(), userID)
	if err != nil {
		logger.Get().Warnw("connected apps: could not list Hydra consent sessions", "error", err)
	}
	for i := range sessions {
		s := &sessions[i]
		client := s.ConsentRequest.Client
		if client.ClientID == "" {
			continue
		}
		if i, ok := listed[client.ClientID]; ok {
			if out[i].Legacy {
				out[i].Scopes = mergeScopes(out[i].Scopes, apiScopes(s.GrantScope))
			}
			continue
		}
		listed[client.ClientID] = len(out)
		out = append(out, ConnectedAppResponse{
			ClientID:   client.ClientID,
			ClientName: client.ClientName,
			Scopes:     apiScopes(s.GrantScope),
			Legacy:     true,
		})
	}

	c.JSON(http.StatusOK, ConnectedAppsResponse{Clients: out})
}

// mergeScopes returns a with the members of b it lacks appended.
func mergeScopes(a, b []string) []string {
	for _, s := range b {
		if !slices.Contains(a, s) {
			a = append(a, s)
		}
	}
	return a
}

// DisconnectApp revokes the authenticated user's access for an OAuth client.
// Hydra revokes the client's consent sessions first, which invalidates its
// refresh tokens; the local grant (if any) is then marked revoked, which
// blocks MCP writes at once. Access tokens already issued keep working for
// reads until they expire (at most 15 minutes). If Hydra fails nothing is
// changed, so the user can simply retry.
//
// Hydra is called even without a grant row: a client connected before grants
// existed (migration 000033 dropped the old trust table) has a live Hydra
// session and nothing else. Revoking only ever affects the caller's own
// sessions, so the call is idempotent and safe for any client_id.
//
// @Summary     Disconnect an app
// @Description Revoke an OAuth client's access and stop it from auto-connecting
// @Tags        oauth
// @Security    BearerAuth
// @Param       client_id path string true "OAuth client ID"
// @Success     204 "Disconnected"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Router      /oauth/clients/{client_id} [delete]
func (h *OAuthHandler) DisconnectApp(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}
	clientID := c.Param("client_id")
	if clientID == "" {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, "client_id is required"))
		return
	}

	grant, err := h.grants.Get(userID, clientID)
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Code == apperrors.ErrOAuthClientNotFound.Code {
		grant, err = nil, nil
	}
	if err != nil {
		respondWithError(c, err)
		return
	}

	if err := h.hydra.RevokeConsentSessions(c.Request.Context(), userID, clientID); err != nil {
		respondWithError(c, err)
		return
	}
	// The handler checks IsRevoked itself (Revoke would also refuse) so an
	// already-revoked grant still gets the Hydra revoke above, as a retry.
	if grant != nil && !grant.IsRevoked() {
		if err := h.grants.Revoke(userID, clientID); err != nil {
			respondWithError(c, err)
			return
		}
	}

	h.auditService.Log(userID, "OAUTH_CLIENT_REVOKED", "oauth_client", clientID, c.ClientIP(),
		map[string]interface{}{"client_name": grantClientName(grant), "had_grant": grant != nil})
	c.Status(http.StatusNoContent)
}

// grantClientName returns the grant's client name, or "" without a grant.
func grantClientName(g *models.OAuthClientGrant) string {
	if g == nil {
		return ""
	}
	return g.ClientName
}
