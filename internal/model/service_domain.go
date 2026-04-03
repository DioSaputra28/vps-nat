package model

import "time"

type ServiceDomain struct {
	ID         string    `gorm:"column:id;primaryKey"`
	ServiceID  string    `gorm:"column:service_id"`
	Domain     string    `gorm:"column:domain"`
	TargetPort int       `gorm:"column:target_port"`
	ProxyMode  string    `gorm:"column:proxy_mode"`
	Status     string    `gorm:"column:status"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`

	Service *Service `gorm:"foreignKey:ServiceID;references:ID"`
}

func (ServiceDomain) TableName() string {
	return "service_domains"
}
