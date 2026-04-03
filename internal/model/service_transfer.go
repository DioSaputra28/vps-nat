package model

import "time"

type ServiceTransfer struct {
	ID            string    `gorm:"column:id;primaryKey"`
	ServiceID     string    `gorm:"column:service_id"`
	FromUserID    string    `gorm:"column:from_user_id"`
	ToUserID      string    `gorm:"column:to_user_id"`
	Reason        *string   `gorm:"column:reason"`
	CreatedByType string    `gorm:"column:created_by_type"`
	CreatedByID   *string   `gorm:"column:created_by_id"`
	CreatedAt     time.Time `gorm:"column:created_at"`

	Service  *Service `gorm:"foreignKey:ServiceID;references:ID"`
	FromUser *User    `gorm:"foreignKey:FromUserID;references:ID"`
	ToUser   *User    `gorm:"foreignKey:ToUserID;references:ID"`
}

func (ServiceTransfer) TableName() string {
	return "service_transfers"
}
