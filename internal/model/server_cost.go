package model

import "time"

type ServerCost struct {
	ID               string    `gorm:"column:id;primaryKey"`
	NodeID           string    `gorm:"column:node_id"`
	PurchaseCost     int64     `gorm:"column:purchase_cost"`
	Notes            *string   `gorm:"column:notes"`
	RecordedAt       time.Time `gorm:"column:recorded_at"`
	CreatedByAdminID *string   `gorm:"column:created_by_admin_id"`
	CreatedAt        time.Time `gorm:"column:created_at"`

	Node           *Node      `gorm:"foreignKey:NodeID;references:ID"`
	CreatedByAdmin *AdminUser `gorm:"foreignKey:CreatedByAdminID;references:ID"`
}

func (ServerCost) TableName() string {
	return "server_costs"
}
