package model

import "time"

type Wallet struct {
	ID        string    `gorm:"column:id;primaryKey"`
	UserID    string    `gorm:"column:user_id"`
	Balance   int64     `gorm:"column:balance"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`

	User         *User               `gorm:"foreignKey:UserID;references:ID"`
	Transactions []WalletTransaction `gorm:"foreignKey:WalletID;references:ID"`
}

func (Wallet) TableName() string {
	return "wallets"
}
