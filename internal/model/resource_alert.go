package model

import "time"

type ResourceAlert struct {
	ID               string         `gorm:"column:id;primaryKey"`
	ServiceID        *string        `gorm:"column:service_id"`
	NodeID           *string        `gorm:"column:node_id"`
	AlertType        string         `gorm:"column:alert_type"`
	ThresholdPercent int            `gorm:"column:threshold_percent"`
	DurationMinutes  int            `gorm:"column:duration_minutes"`
	Status           string         `gorm:"column:status"`
	OpenedAt         time.Time      `gorm:"column:opened_at"`
	ResolvedAt       *time.Time     `gorm:"column:resolved_at"`
	Metadata         map[string]any `gorm:"column:metadata;serializer:json"`

	Service *Service `gorm:"foreignKey:ServiceID;references:ID"`
	Node    *Node    `gorm:"foreignKey:NodeID;references:ID"`
}

func (ResourceAlert) TableName() string {
	return "resource_alerts"
}
