package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
)

// ConnectedAppResponse is one OAuth client the user has granted access to.
type ConnectedAppResponse struct {
	ClientID   string     `json:"client_id"`
	ClientName string     `json:"client_name"`
	Scopes     []string   `json:"scopes"`
	AutoAccept bool       `json:"auto_accept"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// ConnectedAppsResponse wraps the user's connected apps.
type ConnectedAppsResponse struct {
	Clients []ConnectedAppResponse `json:"clients"`
}

// ListConnectedApps returns the OAuth clients (e.g. Claude connectors) the
// authenticated user has granted access to.
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
	for i := range grants {
		g := &grants[i]
		out = append(out, ConnectedAppResponse{
			ClientID:   g.ClientID,
			ClientName: g.ClientName,
			Scopes:     g.Approved(),
			AutoAccept: g.AutoAccept,
			CreatedAt:  g.CreatedAt,
			LastUsedAt: g.LastUsedAt,
		})
	}
	c.JSON(http.StatusOK, ConnectedAppsResponse{Clients: out})
}

// DisconnectApp revokes the authenticated user's grant for an OAuth client.
// Hydra revokes the client's consent sessions first, which invalidates its
// refresh tokens; the local grant is then marked revoked, which blocks MCP
// writes at once. Access tokens already issued keep working for reads until
// they expire (at most 15 minutes). If Hydra fails nothing is changed, so the
// user can simply retry.
//
// @Summary     Disconnect an app
// @Description Revoke an OAuth client's access and stop it from auto-connecting
// @Tags        oauth
// @Security    BearerAuth
// @Param       client_id path string true "OAuth client ID"
// @Success     204 "Disconnected"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Connected app not found"
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
	if err != nil {
		respondWithError(c, err)
		return
	}
	if grant.IsRevoked() {
		respondWithError(c, apperrors.ErrOAuthClientNotFound)
		return
	}

	if err := h.hydra.RevokeConsentSessions(c.Request.Context(), userID, clientID); err != nil {
		respondWithError(c, err)
		return
	}
	if err := h.grants.Revoke(userID, clientID); err != nil {
		respondWithError(c, err)
		return
	}

	h.auditService.Log(userID, "OAUTH_CLIENT_REVOKED", "oauth_client", clientID, c.ClientIP(),
		map[string]interface{}{"client_name": grant.ClientName})
	c.Status(http.StatusNoContent)
}
