package model

import "time"

type Service struct {
	ID                  string     `gorm:"column:id;primaryKey"`
	OrderID             string     `gorm:"column:order_id"`
	OwnerUserID         string     `gorm:"column:owner_user_id"`
	CurrentPackageID    string     `gorm:"column:current_package_id"`
	Status              string     `gorm:"column:status"`
	BillingCycleDays    int        `gorm:"column:billing_cycle_days"`
	PackageNameSnapshot string     `gorm:"column:package_name_snapshot"`
	CPUSnapshot         int        `gorm:"column:cpu_snapshot"`
	RAMMBSnapshot       int        `gorm:"column:ram_mb_snapshot"`
	DiskGBSnapshot      int        `gorm:"column:disk_gb_snapshot"`
	PriceSnapshot       int64      `gorm:"column:price_snapshot"`
	StartedAt           *time.Time `gorm:"column:started_at"`
	ExpiresAt           *time.Time `gorm:"column:expires_at"`
	CanceledAt          *time.Time `gorm:"column:canceled_at"`
	SuspendedAt         *time.Time `gorm:"column:suspended_at"`
	TerminatedAt        *time.Time `gorm:"column:terminated_at"`
	CreatedAt           time.Time  `gorm:"column:created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at"`

	Order        *Order               `gorm:"foreignKey:OrderID;references:ID"`
	OwnerUser    *User                `gorm:"foreignKey:OwnerUserID;references:ID"`
	CurrentPack  *Package             `gorm:"foreignKey:CurrentPackageID;references:ID"`
	Instance     *ServiceInstance     `gorm:"foreignKey:ServiceID;references:ID"`
	PortMappings []ServicePortMapping `gorm:"foreignKey:ServiceID;references:ID"`
	Domains      []ServiceDomain      `gorm:"foreignKey:ServiceID;references:ID"`
	Events       []ServiceEvent       `gorm:"foreignKey:ServiceID;references:ID"`
	Transfers    []ServiceTransfer    `gorm:"foreignKey:ServiceID;references:ID"`
	Jobs         []ProvisioningJob    `gorm:"foreignKey:ServiceID;references:ID"`
	Alerts       []ResourceAlert      `gorm:"foreignKey:ServiceID;references:ID"`
}

func (Service) TableName() string {
	return "services"
}
