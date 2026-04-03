package model

import "time"

type WalletTopup struct {
	ID          string     `gorm:"column:id;primaryKey"`
	UserID      string     `gorm:"column:user_id"`
	Status      string     `gorm:"column:status"`
	Amount      int64      `gorm:"column:amount"`
	RequestedAt time.Time  `gorm:"column:requested_at"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	ExpiredAt   *time.Time `gorm:"column:expired_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`

	User     *User     `gorm:"foreignKey:UserID;references:ID"`
	Payments []Payment `gorm:"foreignKey:WalletTopupID;references:ID"`
}

func (WalletTopup) TableName() string {
	return "wallet_topups"
}
