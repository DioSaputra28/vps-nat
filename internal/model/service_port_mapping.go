package model

import "time"

type ServicePortMapping struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	ServiceID   string    `gorm:"column:service_id" json:"service_id"`
	MappingType string    `gorm:"column:mapping_type" json:"mapping_type"`
	PublicIP    string    `gorm:"column:public_ip" json:"public_ip"`
	PublicPort  int       `gorm:"column:public_port" json:"public_port"`
	Protocol    string    `gorm:"column:protocol" json:"protocol"`
	TargetIP    string    `gorm:"column:target_ip" json:"target_ip"`
	TargetPort  int       `gorm:"column:target_port" json:"target_port"`
	Description *string   `gorm:"column:description" json:"description,omitempty"`
	IsActive    bool      `gorm:"column:is_active" json:"is_active"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`

	Service *Service `gorm:"foreignKey:ServiceID;references:ID"`
}

func (ServicePortMapping) TableName() string {
	return "service_port_mappings"
}
