package model

import "time"

type Invoice struct {
	ID          string     `gorm:"column:id;primaryKey"`
	OrderID     string     `gorm:"column:order_id"`
	InvoiceCode string     `gorm:"column:invoice_code"`
	Amount      int64      `gorm:"column:amount"`
	Status      string     `gorm:"column:status"`
	ExpiresAt   *time.Time `gorm:"column:expires_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`

	Order *Order `gorm:"foreignKey:OrderID;references:ID"`
}

func (Invoice) TableName() string {
	return "invoices"
}
