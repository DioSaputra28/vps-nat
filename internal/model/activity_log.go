package model

import "time"

type ActivityLog struct {
	ID         string         `gorm:"column:id;primaryKey"`
	ActorType  string         `gorm:"column:actor_type"`
	ActorID    *string        `gorm:"column:actor_id"`
	Action     string         `gorm:"column:action"`
	TargetType string         `gorm:"column:target_type"`
	TargetID   *string        `gorm:"column:target_id"`
	Metadata   map[string]any `gorm:"column:metadata;serializer:json"`
	CreatedAt  time.Time      `gorm:"column:created_at"`
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}
