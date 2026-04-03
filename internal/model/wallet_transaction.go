package model

import "time"

type WalletTransaction struct {
	ID              string    `gorm:"column:id;primaryKey"`
	WalletID        string    `gorm:"column:wallet_id"`
	UserID          string    `gorm:"column:user_id"`
	Direction       string    `gorm:"column:direction"`
	TransactionType string    `gorm:"column:transaction_type"`
	Amount          int64     `gorm:"column:amount"`
	BalanceBefore   int64     `gorm:"column:balance_before"`
	BalanceAfter    int64     `gorm:"column:balance_after"`
	SourceType      string    `gorm:"column:source_type"`
	SourceID        *string   `gorm:"column:source_id"`
	Note            *string   `gorm:"column:note"`
	CreatedAt       time.Time `gorm:"column:created_at"`

	Wallet *Wallet `gorm:"foreignKey:WalletID;references:ID"`
	User   *User   `gorm:"foreignKey:UserID;references:ID"`
}

func (WalletTransaction) TableName() string {
	return "wallet_transactions"
}
