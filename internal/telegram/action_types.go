package telegram

import (
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
)

type RuntimeActionInput struct {
	TelegramID  int64
	ContainerID string
	Action      string
}

type ActionAcceptedResult struct {
	ContainerID string  `json:"container_id"`
	ServiceID   string  `json:"service_id"`
	Action      string  `json:"action"`
	Accepted    bool    `json:"accepted"`
	OperationID *string `json:"operation_id,omitempty"`
	JobID       *string `json:"job_id,omitempty"`
	Status      string  `json:"status"`
}

type ChangePasswordInput struct {
	TelegramID  int64
	ContainerID string
	NewPassword string
}

type ChangePasswordResult struct {
	ActionAcceptedResult
}

type ResetSSHInput struct {
	TelegramID  int64
	ContainerID string
}

type ResetSSHResult struct {
	ActionAcceptedResult
	Username    string `json:"username"`
	NewPassword string `json:"new_password"`
}

type RenewPreviewInput struct {
	TelegramID  int64
	ContainerID string
}

type PackageQuote struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CPU          int    `json:"cpu"`
	RAMMB        int    `json:"ram_mb"`
	DiskGB       int    `json:"disk_gb"`
	Price        int64  `json:"price"`
	DurationDays int    `json:"duration_days"`
}

type RenewPreviewResult struct {
	ContainerID      string       `json:"container_id"`
	ServiceID        string       `json:"service_id"`
	Package          PackageQuote `json:"package"`
	Price            int64        `json:"price"`
	CurrentExpiresAt *time.Time   `json:"current_expires_at,omitempty"`
	NextExpiresAt    *time.Time   `json:"next_expires_at,omitempty"`
	PaymentMethods   []string     `json:"payment_methods"`
}

type RenewSubmitInput struct {
	TelegramID    int64
	ContainerID   string
	PaymentMethod string
}

type BillingSubmitResult struct {
	ContainerID string     `json:"container_id"`
	ServiceID   string     `json:"service_id"`
	OrderID     string     `json:"order_id"`
	PaymentID   string     `json:"payment_id"`
	Action      string     `json:"action"`
	Amount      int64      `json:"amount"`
	Applied     bool       `json:"applied"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type UpgradeOptionsInput struct {
	TelegramID  int64
	ContainerID string
}

type UpgradeOptionsResult struct {
	ContainerID string         `json:"container_id"`
	ServiceID   string         `json:"service_id"`
	Current     PackageQuote   `json:"current"`
	Packages    []PackageQuote `json:"packages"`
}

type UpgradePreviewInput struct {
	TelegramID      int64
	ContainerID     string
	TargetPackageID string
}

type UpgradePreviewResult struct {
	ContainerID      string       `json:"container_id"`
	ServiceID        string       `json:"service_id"`
	Current          PackageQuote `json:"current"`
	Target           PackageQuote `json:"target"`
	CurrentExpiresAt *time.Time   `json:"current_expires_at,omitempty"`
	RemainingSeconds int64        `json:"remaining_seconds"`
	PriceDifference  int64        `json:"price_difference"`
	PaymentMethods   []string     `json:"payment_methods"`
}

type UpgradeSubmitInput struct {
	TelegramID      int64
	ContainerID     string
	TargetPackageID string
	PaymentMethod   string
}

type ImageOption struct {
	Label string `json:"label"`
	Alias string `json:"alias"`
}

type ReinstallOptionsInput struct {
	TelegramID  int64
	ContainerID string
}

type ReinstallOptionsResult struct {
	ContainerID string        `json:"container_id"`
	ServiceID   string        `json:"service_id"`
	Images      []ImageOption `json:"images"`
}

type ReinstallPreviewInput struct {
	TelegramID  int64
	ContainerID string
	ImageAlias  string
}

type ReinstallPreviewResult struct {
	ContainerID string      `json:"container_id"`
	ServiceID   string      `json:"service_id"`
	Image       ImageOption `json:"image"`
	Warning     string      `json:"warning"`
}

type ReinstallSubmitInput struct {
	TelegramID  int64
	ContainerID string
	ImageAlias  string
}

type ReinstallSubmitResult struct {
	ActionAcceptedResult
	Username    string `json:"username"`
	NewPassword string `json:"new_password"`
	ImageAlias  string `json:"image_alias"`
}

type DomainPreviewInput struct {
	TelegramID  int64
	ContainerID string
	Domain      string
	TargetPort  int
	ProxyMode   string
}

type DomainPreviewResult struct {
	ContainerID      string  `json:"container_id"`
	ServiceID        string  `json:"service_id"`
	Domain           string  `json:"domain"`
	TargetPort       int     `json:"target_port"`
	ProxyMode        string  `json:"proxy_mode"`
	Available        bool    `json:"available"`
	ExpectedPublicIP *string `json:"expected_public_ip,omitempty"`
	DNSReady         bool    `json:"dns_ready"`
	UpstreamTarget   string  `json:"upstream_target"`
}

type DomainSubmitInput struct {
	TelegramID  int64
	ContainerID string
	Domain      string
	TargetPort  int
	ProxyMode   string
}

type DomainSubmitResult struct {
	ContainerID string              `json:"container_id"`
	ServiceID   string              `json:"service_id"`
	Domain      model.ServiceDomain `json:"domain"`
	Status      string              `json:"status"`
}

type ReconfigPortMapping struct {
	MappingType string  `json:"mapping_type"`
	PublicIP    string  `json:"public_ip"`
	PublicPort  int     `json:"public_port"`
	Protocol    string  `json:"protocol"`
	TargetIP    string  `json:"target_ip"`
	TargetPort  int     `json:"target_port"`
	Description *string `json:"description,omitempty"`
	IsActive    bool    `json:"is_active"`
}

type ReconfigIPPreviewInput struct {
	TelegramID  int64
	ContainerID string
}

type ReconfigIPPreviewResult struct {
	ContainerID      string                `json:"container_id"`
	ServiceID        string                `json:"service_id"`
	MainPublicIP     *string               `json:"main_public_ip,omitempty"`
	CurrentSSHPort   *int                  `json:"current_ssh_port,omitempty"`
	ProposedSSHPort  int                   `json:"proposed_ssh_port"`
	CurrentMappings  []ReconfigPortMapping `json:"current_mappings"`
	ProposedMappings []ReconfigPortMapping `json:"proposed_mappings"`
}

type ReconfigIPSubmitInput struct {
	TelegramID       int64
	ContainerID      string
	RequestedSSHPort *int
}

type ReconfigIPSubmitResult struct {
	ActionAcceptedResult
	MainPublicIP *string               `json:"main_public_ip,omitempty"`
	SSHPort      *int                  `json:"ssh_port,omitempty"`
	PortMappings []ReconfigPortMapping `json:"port_mappings"`
}

type TransferPreviewInput struct {
	TelegramID       int64
	ContainerID      string
	TargetTelegramID int64
}

type TransferUser struct {
	ID               string  `json:"id"`
	TelegramID       int64   `json:"telegram_id"`
	TelegramUsername *string `json:"telegram_username,omitempty"`
	DisplayName      string  `json:"display_name"`
}

type TransferPreviewResult struct {
	ContainerID string       `json:"container_id"`
	ServiceID   string       `json:"service_id"`
	FromUser    TransferUser `json:"from_user"`
	ToUser      TransferUser `json:"to_user"`
}

type TransferSubmitInput struct {
	TelegramID       int64
	ContainerID      string
	TargetTelegramID int64
	Reason           *string
}

type TransferSubmitResult struct {
	ContainerID string       `json:"container_id"`
	ServiceID   string       `json:"service_id"`
	FromUser    TransferUser `json:"from_user"`
	ToUser      TransferUser `json:"to_user"`
}

type CancelPreviewInput struct {
	TelegramID  int64
	ContainerID string
}

type CancelPreviewResult struct {
	ContainerID      string     `json:"container_id"`
	ServiceID        string     `json:"service_id"`
	PackageName      string     `json:"package_name"`
	CurrentExpiresAt *time.Time `json:"current_expires_at,omitempty"`
	RemainingSeconds int64      `json:"remaining_seconds"`
	RefundAmount     int64      `json:"refund_amount"`
}

type CancelSubmitInput struct {
	TelegramID  int64
	ContainerID string
}

type CancelSubmitResult struct {
	ActionAcceptedResult
	RefundAmount int64 `json:"refund_amount"`
}
