package services

import (
	"encoding/json"

	"gorm.io/gorm"

	"kuberan/internal/logger"
	"kuberan/internal/models"
)

// auditService handles audit log recording.
type auditService struct {
	db *gorm.DB
}

// NewAuditService creates a new AuditServicer.
func NewAuditService(db *gorm.DB) AuditServicer {
	return &auditService{db: db}
}

// Log records an audit event. Errors are logged but never propagated
// to avoid disrupting the main operation.
func (s *auditService) Log(userID uint, action, resourceType string, resourceID uint, ipAddress string, changes map[string]interface{}) {
	var changesJSON string
	if changes != nil {
		data, err := json.Marshal(changes)
		if err != nil {
			logger.Get().Errorw("failed to marshal audit log changes", "error", err, "action", action)
			changesJSON = "{}"
		} else {
			changesJSON = string(data)
		}
	}

	entry := &models.AuditLog{
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    ipAddress,
		Changes:      changesJSON,
	}

	if err := s.db.Create(entry).Error; err != nil {
		logger.Get().Errorw("failed to create audit log entry",
			"error", err,
			"user_id", userID,
			"action", action,
			"resource_type", resourceType,
			"resource_id", resourceID,
		)
	}
}
