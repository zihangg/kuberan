package models

import "time"

// RefreshSession is one signed-in device's refresh-token chain. Each login
// creates a new session, so signing in on another device never invalidates an
// existing one. Only SHA-256 hashes of refresh tokens are stored.
//
// PreviousTokenHash and RotatedAt implement a short rotation grace period: the
// token that was just rotated out stays usable for a moment, so two tabs racing
// to refresh (or a refresh response lost on a flaky network) does not log the
// device out.
type RefreshSession struct {
	Base
	UserID            string     `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash         string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	PreviousTokenHash string     `gorm:"size:64;not null;default:'';index" json:"-"`
	RotatedAt         *time.Time `json:"rotated_at,omitempty"`
	ExpiresAt         time.Time  `gorm:"not null" json:"expires_at"`
	UserAgent         string     `gorm:"size:255;not null;default:''" json:"user_agent"`
	IPAddress         string     `gorm:"size:45;not null;default:''" json:"ip_address"`
}
