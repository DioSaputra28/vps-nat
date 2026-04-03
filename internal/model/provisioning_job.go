package model

import "time"

type ProvisioningJob struct {
	ID               string         `gorm:"column:id;primaryKey"`
	ServiceID        *string        `gorm:"column:service_id"`
	OrderID          *string        `gorm:"column:order_id"`
	JobType          string         `gorm:"column:job_type"`
	Status           string         `gorm:"column:status"`
	IncusOperationID *string        `gorm:"column:incus_operation_id"`
	RequestedByType  string         `gorm:"column:requested_by_type"`
	RequestedByID    *string        `gorm:"column:requested_by_id"`
	AttemptCount     int            `gorm:"column:attempt_count"`
	ErrorMessage     *string        `gorm:"column:error_message"`
	Payload          map[string]any `gorm:"column:payload;serializer:json"`
	StartedAt        *time.Time     `gorm:"column:started_at"`
	FinishedAt       *time.Time     `gorm:"column:finished_at"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`

	Service *Service `gorm:"foreignKey:ServiceID;references:ID"`
	Order   *Order   `gorm:"foreignKey:OrderID;references:ID"`
}

func (ProvisioningJob) TableName() string {
	return "provisioning_jobs"
}
