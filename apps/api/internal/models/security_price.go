package models

import "time"

// SecurityPrice represents a historical price entry for a security.
// This is immutable time-series data — no Base embed, no soft deletes.
type SecurityPrice struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	SecurityID uint      `gorm:"not null" json:"security_id"`
	Price      int64     `gorm:"type:bigint;not null" json:"price"`
	RecordedAt time.Time `gorm:"not null" json:"recorded_at"`
	Security   Security  `gorm:"foreignKey:SecurityID" json:"security,omitempty"`
}
