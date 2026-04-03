package model

import "time"

type Payment struct {
	ID              string         `gorm:"column:id;primaryKey"`
	OrderID         *string        `gorm:"column:order_id"`
	WalletTopupID   *string        `gorm:"column:wallet_topup_id"`
	Purpose         string         `gorm:"column:purpose"`
	Method          string         `gorm:"column:method"`
	Provider        *string        `gorm:"column:provider"`
	ProviderRef     *string        `gorm:"column:provider_reference"`
	ProviderPayload map[string]any `gorm:"column:provider_payload;serializer:json"`
	Amount          int64          `gorm:"column:amount"`
	Status          string         `gorm:"column:status"`
	PaidAt          *time.Time     `gorm:"column:paid_at"`
	ExpiredAt       *time.Time     `gorm:"column:expired_at"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`

	Order       *Order       `gorm:"foreignKey:OrderID;references:ID"`
	WalletTopup *WalletTopup `gorm:"foreignKey:WalletTopupID;references:ID"`
}

func (Payment) TableName() string {
	return "payments"
}
