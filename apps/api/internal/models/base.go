package models

import (
	"time"

	"kuberan/internal/uuid"

	"gorm.io/gorm"
)

// Base contains common columns for all tables
type Base struct {
	ID        string         `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BeforeCreate hook generates a UUIDv7 for new records
func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New()
	}
	return nil
}
