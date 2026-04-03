package model

import "time"

type AdminSession struct {
	ID          string     `gorm:"column:id;primaryKey"`
	AdminUserID string     `gorm:"column:admin_user_id"`
	TokenHash   string     `gorm:"column:token_hash"`
	UserAgent   *string    `gorm:"column:user_agent"`
	IPAddress   *string    `gorm:"column:ip_address"`
	ExpiresAt   time.Time  `gorm:"column:expires_at"`
	LastUsedAt  *time.Time `gorm:"column:last_used_at"`
	RevokedAt   *time.Time `gorm:"column:revoked_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`

	AdminUser *AdminUser `gorm:"foreignKey:AdminUserID;references:ID"`
}

func (AdminSession) TableName() string {
	return "admin_sessions"
}
