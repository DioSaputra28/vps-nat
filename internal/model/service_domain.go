package model

import "time"

type ServiceDomain struct {
	ID         string    `gorm:"column:id;primaryKey" json:"id"`
	ServiceID  string    `gorm:"column:service_id" json:"service_id"`
	Domain     string    `gorm:"column:domain" json:"domain"`
	TargetPort int       `gorm:"column:target_port" json:"target_port"`
	ProxyMode  string    `gorm:"column:proxy_mode" json:"proxy_mode"`
	Status     string    `gorm:"column:status" json:"status"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`

	Service *Service `gorm:"foreignKey:ServiceID;references:ID"`
}

func (ServiceDomain) TableName() string {
	return "service_domains"
}
