package model

import "time"

type Order struct {
	ID                   string     `gorm:"column:id;primaryKey"`
	UserID               string     `gorm:"column:user_id"`
	PackageID            string     `gorm:"column:package_id"`
	TargetServiceID      *string    `gorm:"column:target_service_id"`
	OrderType            string     `gorm:"column:order_type"`
	Status               string     `gorm:"column:status"`
	PaymentStatus        string     `gorm:"column:payment_status"`
	PaymentMethod        *string    `gorm:"column:payment_method"`
	SelectedImageAlias   *string    `gorm:"column:selected_image_alias"`
	PackageNameSnapshot  string     `gorm:"column:package_name_snapshot"`
	CPUSnapshot          int        `gorm:"column:cpu_snapshot"`
	RAMMBSnapshot        int        `gorm:"column:ram_mb_snapshot"`
	DiskGBSnapshot       int        `gorm:"column:disk_gb_snapshot"`
	PriceSnapshot        int64      `gorm:"column:price_snapshot"`
	DurationDaysSnapshot int        `gorm:"column:duration_days_snapshot"`
	TotalAmount          int64      `gorm:"column:total_amount"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
	PaidAt               *time.Time `gorm:"column:paid_at"`
	CanceledAt           *time.Time `gorm:"column:canceled_at"`
	FailedAt             *time.Time `gorm:"column:failed_at"`

	User          *User     `gorm:"foreignKey:UserID;references:ID"`
	Package       *Package  `gorm:"foreignKey:PackageID;references:ID"`
	TargetService *Service  `gorm:"foreignKey:TargetServiceID;references:ID"`
	Payments      []Payment `gorm:"foreignKey:OrderID;references:ID"`
	Invoice       *Invoice  `gorm:"foreignKey:OrderID;references:ID"`
	Service       *Service  `gorm:"foreignKey:OrderID;references:ID"`
}

func (Order) TableName() string {
	return "orders"
}
