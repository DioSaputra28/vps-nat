package model

import "time"

type ServicePortMapping struct {
	ID          string    `gorm:"column:id;primaryKey"`
	ServiceID   string    `gorm:"column:service_id"`
	MappingType string    `gorm:"column:mapping_type"`
	PublicIP    string    `gorm:"column:public_ip"`
	PublicPort  int       `gorm:"column:public_port"`
	Protocol    string    `gorm:"column:protocol"`
	TargetIP    string    `gorm:"column:target_ip"`
	TargetPort  int       `gorm:"column:target_port"`
	Description *string   `gorm:"column:description"`
	IsActive    bool      `gorm:"column:is_active"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`

	Service *Service `gorm:"foreignKey:ServiceID;references:ID"`
}

func (ServicePortMapping) TableName() string {
	return "service_port_mappings"
}
