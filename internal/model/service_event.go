package model

import "time"

type ServiceEvent struct {
	ID        string         `gorm:"column:id;primaryKey"`
	ServiceID string         `gorm:"column:service_id"`
	EventType string         `gorm:"column:event_type"`
	ActorType string         `gorm:"column:actor_type"`
	ActorID   *string        `gorm:"column:actor_id"`
	Summary   string         `gorm:"column:summary"`
	Payload   map[string]any `gorm:"column:payload;serializer:json"`
	CreatedAt time.Time      `gorm:"column:created_at"`

	Service *Service `gorm:"foreignKey:ServiceID;references:ID"`
}

func (ServiceEvent) TableName() string {
	return "service_events"
}
