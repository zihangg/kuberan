package models

// AuditLog records sensitive user operations for security and compliance.
type AuditLog struct {
	Base
	UserID       uint   `gorm:"not null;index" json:"user_id"`
	Action       string `gorm:"not null" json:"action"`
	ResourceType string `gorm:"not null" json:"resource_type"`
	ResourceID   uint   `json:"resource_id"`
	IPAddress    string `json:"ip_address"`
	Changes      string `json:"changes,omitempty"`
}
