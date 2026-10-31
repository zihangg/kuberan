package services

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/logger"
	"kuberan/internal/models"
)

// lastUsedInterval throttles last_used_at writes from Authorize so a burst of
// MCP writes costs one extra UPDATE per minute at most.
const lastUsedInterval = time.Minute

// oauthGrantService handles per-user OAuth consent grants (plan 022).
type oauthGrantService struct {
	db  *gorm.DB
	now func() time.Time
}

// NewOAuthGrantService creates a new OAuthGrantServicer.
func NewOAuthGrantService(db *gorm.DB) OAuthGrantServicer {
	return &oauthGrantService{db: db, now: time.Now}
}

// Get returns the user's grant for clientID, revoked or not.
func (s *oauthGrantService) Get(userID, clientID string) (*models.OAuthClientGrant, error) {
	if userID == "" || clientID == "" {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "user_id and client_id are required")
	}
	var grant models.OAuthClientGrant
	err := s.db.Where("user_id = ? AND client_id = ?", userID, clientID).First(&grant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrOAuthClientNotFound
	}
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &grant, nil
}

// maxClientNameLen matches oauth_client_grants.client_name (VARCHAR(255)).
// Client names come from self-registration, so they are truncated rather
// than allowed to fail every consent.
const maxClientNameLen = 255

// RecordConsent upserts the grant for (user, client) with the latest decision,
// in one statement so two simultaneous first consents cannot collide on the
// unique index.
func (s *oauthGrantService) RecordConsent(in RecordConsentInput) (*models.OAuthClientGrant, error) {
	if in.UserID == "" || in.ClientID == "" {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "user_id and client_id are required")
	}

	name := truncateRunes(in.ClientName, maxClientNameLen)
	approved := strings.Join(in.Approved, " ")
	reviewed := strings.Join(in.Reviewed, " ")
	grant := &models.OAuthClientGrant{
		UserID:         in.UserID,
		ClientID:       in.ClientID,
		ClientName:     name,
		ApprovedScopes: approved,
		ReviewedScopes: reviewed,
		AutoAccept:     in.AutoAccept,
	}
	err := s.db.Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "user_id"}, {Name: "client_id"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "deleted_at IS NULL"}}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"client_name":     name,
			"approved_scopes": approved,
			"reviewed_scopes": reviewed,
			"auto_accept":     in.AutoAccept,
			"revoked_at":      nil,
			"updated_at":      s.now(),
		}),
	}).Create(grant).Error
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return s.Get(in.UserID, in.ClientID)
}

// List returns the user's active grants, most recently created first.
func (s *oauthGrantService) List(userID string) ([]models.OAuthClientGrant, error) {
	var grants []models.OAuthClientGrant
	if err := s.db.Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC").Find(&grants).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return grants, nil
}

// Revoke marks the user's active grant for clientID revoked and stops it
// auto-accepting.
func (s *oauthGrantService) Revoke(userID, clientID string) error {
	grant, err := s.Get(userID, clientID)
	if err != nil {
		return err
	}
	if grant.IsRevoked() {
		return apperrors.ErrOAuthClientNotFound
	}
	now := s.now()
	if err := s.db.Model(grant).Updates(map[string]interface{}{
		"revoked_at":  now,
		"auto_accept": false,
	}).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return nil
}

// Authorize checks the user is active and holds an active grant for clientID
// that includes scope. It is the MCP write guard's source of truth: a revoked
// grant or deactivated user blocks writes immediately, without waiting for
// the access token to expire.
func (s *oauthGrantService) Authorize(userID, clientID, scope string) error {
	if userID == "" || clientID == "" || scope == "" {
		return apperrors.ErrOAuthScopeNotGranted
	}

	var user models.User
	if err := s.db.Select("id", "is_active").Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrOAuthScopeNotGranted
		}
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	if !user.IsActive {
		return apperrors.ErrOAuthScopeNotGranted
	}

	grant, err := s.Get(userID, clientID)
	if errors.Is(err, apperrors.ErrOAuthClientNotFound) {
		return apperrors.ErrOAuthScopeNotGranted
	}
	if err != nil {
		return err
	}
	if grant.IsRevoked() || !grant.HasApproved(scope) {
		return apperrors.ErrOAuthScopeNotGranted
	}

	// last_used_at is bookkeeping for the Connected apps page: a failure to
	// record it must not block an authorized write.
	now := s.now()
	if grant.LastUsedAt == nil || now.Sub(*grant.LastUsedAt) >= lastUsedInterval {
		if err := s.db.Model(grant).UpdateColumn("last_used_at", now).Error; err != nil {
			logger.Get().Warnw("failed to record OAuth grant last use", "error", err, "client_id", clientID)
		}
	}
	return nil
}
