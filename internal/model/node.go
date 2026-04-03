package model

import "time"

type Node struct {
	ID          string    `gorm:"column:id;primaryKey"`
	Name        string    `gorm:"column:name"`
	DisplayName string    `gorm:"column:display_name"`
	PublicIP    string    `gorm:"column:public_ip"`
	Status      string    `gorm:"column:status"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (Node) TableName() string {
	return "nodes"
}
