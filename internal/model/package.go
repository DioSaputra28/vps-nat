package model

import "time"

type Package struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Name         string    `gorm:"column:name"`
	Description  *string   `gorm:"column:description"`
	CPU          int       `gorm:"column:cpu"`
	RAMMB        int       `gorm:"column:ram_mb"`
	DiskGB       int       `gorm:"column:disk_gb"`
	Price        int64     `gorm:"column:price"`
	DurationDays int       `gorm:"column:duration_days"`
	IsActive     bool      `gorm:"column:is_active"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (Package) TableName() string {
	return "packages"
}
