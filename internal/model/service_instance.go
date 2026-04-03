package model

import "time"

type ServiceInstance struct {
	ID                   string    `gorm:"column:id;primaryKey"`
	ServiceID            string    `gorm:"column:service_id"`
	NodeID               string    `gorm:"column:node_id"`
	IncusInstanceName    string    `gorm:"column:incus_instance_name"`
	ImageAlias           string    `gorm:"column:image_alias"`
	OSFamily             *string   `gorm:"column:os_family"`
	InternalIP           *string   `gorm:"column:internal_ip"`
	MainPublicIP         *string   `gorm:"column:main_public_ip"`
	SSHPort              *int      `gorm:"column:ssh_port"`
	Status               string    `gorm:"column:status"`
	LastIncusOperationID *string   `gorm:"column:last_incus_operation_id"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`

	Service *Service `gorm:"foreignKey:ServiceID;references:ID"`
	Node    *Node    `gorm:"foreignKey:NodeID;references:ID"`
}

func (ServiceInstance) TableName() string {
	return "service_instances"
}
