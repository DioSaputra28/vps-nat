package model

import "time"

type User struct {
	ID               string    `gorm:"column:id;primaryKey"`
	TelegramID       int64     `gorm:"column:telegram_id"`
	TelegramUsername *string   `gorm:"column:telegram_username"`
	DisplayName      string    `gorm:"column:display_name"`
	Status           string    `gorm:"column:status"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`

	Wallet *Wallet `gorm:"foreignKey:UserID;references:ID"`
}

func (User) TableName() string {
	return "users"
}
