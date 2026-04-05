package telegram

import (
	"context"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
)

type BuyVPSSubmitInput struct {
	TelegramID    int64
	PackageID     string
	ImageAlias    string
	Hostname      string
	PaymentMethod string
}

type BuyVPSSubmitResult struct {
	OrderID   string                `json:"order_id"`
	PaymentID string                `json:"payment_id"`
	Status    string                `json:"status"`
	Service   *BuyVPSServicePayload `json:"service,omitempty"`
	Payment   *BuyVPSPaymentPayload `json:"payment,omitempty"`
}

type BuyVPSStatusInput struct {
	TelegramID int64
	OrderID    string
}

type BuyVPSStatusResult struct {
	OrderID string                `json:"order_id"`
	Status  string                `json:"status"`
	Service *BuyVPSServicePayload `json:"service,omitempty"`
	Payment *BuyVPSPaymentPayload `json:"payment,omitempty"`
	Error   *string               `json:"error,omitempty"`
}

type WalletTopupSubmitInput struct {
	TelegramID int64
	Amount     int64
}

type WalletTopupSubmitResult struct {
	TopupID   string                `json:"topup_id"`
	PaymentID string                `json:"payment_id"`
	Status    string                `json:"status"`
	Payment   *BuyVPSPaymentPayload `json:"payment,omitempty"`
}

type WalletTopupStatusInput struct {
	TelegramID int64
	TopupID    string
}

type WalletTopupStatusResult struct {
	TopupID string                `json:"topup_id"`
	Status  string                `json:"status"`
	Amount  int64                 `json:"amount"`
	Balance *int64                `json:"balance,omitempty"`
	Payment *BuyVPSPaymentPayload `json:"payment,omitempty"`
	PaidAt  *time.Time            `json:"paid_at,omitempty"`
	Error   *string               `json:"error,omitempty"`
}

type BuyVPSServicePayload struct {
	OrderID      string                     `json:"order_id"`
	ServiceID    string                     `json:"service_id"`
	ContainerID  string                     `json:"container_id"`
	Package      BuyVPSPackagePayload       `json:"package"`
	Hostname     string                     `json:"hostname"`
	PublicIP     string                     `json:"public_ip"`
	PrivateIP    string                     `json:"private_ip"`
	SSHUsername  string                     `json:"ssh_username"`
	SSHPassword  string                     `json:"ssh_password,omitempty"`
	SSHPort      int                        `json:"ssh_port"`
	ExpiresAt    *time.Time                 `json:"expires_at,omitempty"`
	PortMappings []BuyVPSPortMappingPayload `json:"port_mappings"`
}

type BuyVPSPackagePayload struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CPU          int    `json:"cpu"`
	RAMMB        int    `json:"ram_mb"`
	DiskGB       int    `json:"disk_gb"`
	Price        int64  `json:"price"`
	DurationDays int    `json:"duration_days"`
}

type BuyVPSPortMappingPayload struct {
	PublicIP    string `json:"public_ip"`
	PublicPort  int    `json:"public_port"`
	Protocol    string `json:"protocol"`
	TargetIP    string `json:"target_ip"`
	TargetPort  int    `json:"target_port"`
	MappingType string `json:"mapping_type"`
}

type BuyVPSPaymentPayload struct {
	Provider      string     `json:"provider"`
	PaymentMethod string     `json:"payment_method"`
	Amount        int64      `json:"amount"`
	Fee           int64      `json:"fee"`
	TotalPayment  int64      `json:"total_payment"`
	QRString      string     `json:"qr_string"`
	ExpiredAt     *time.Time `json:"expired_at,omitempty"`
}

type PakasirWebhookInput struct {
	Amount        int64  `json:"amount"`
	OrderID       string `json:"order_id"`
	Project       string `json:"project"`
	Status        string `json:"status"`
	PaymentMethod string `json:"payment_method"`
	CompletedAt   string `json:"completed_at"`
}

type RequestedPortMapping struct {
	MappingType string
	Protocol    string
	TargetPort  int
	PublicPort  int
	Description string
}

type ProvisionedPortMapping struct {
	MappingType string
	PublicIP    string
	PublicPort  int
	Protocol    string
	TargetIP    string
	TargetPort  int
}

type PurchaseProvisionRequest struct {
	OrderID      string
	Hostname     string
	ImageAlias   string
	Node         model.Node
	Package      BuyVPSPackagePayload
	PortMappings []RequestedPortMapping
}

type PurchaseProvisionResult struct {
	OperationID  string
	Hostname     string
	PublicIP     string
	PrivateIP    string
	SSHUsername  string
	SSHPassword  string
	SSHPort      int
	OSFamily     *string
	PortMappings []ProvisionedPortMapping
}

type PakasirCreateTransactionRequest struct {
	OrderID string
	Amount  int64
	Method  string
}

type PakasirTransaction struct {
	Provider     string
	OrderID      string
	Amount       int64
	Fee          int64
	TotalPayment int64
	QRString     string
	ExpiredAt    time.Time
	Raw          map[string]any
}

type PakasirVerifyTransactionRequest struct {
	OrderID string
	Amount  int64
}

type PakasirVerifiedTransaction struct {
	OrderID       string
	Amount        int64
	Status        string
	PaymentMethod string
	CompletedAt   time.Time
	Raw           map[string]any
}

type PurchaseProvisioner interface {
	HostnameExists(ctx context.Context, hostname string) (bool, error)
	Provision(ctx context.Context, req PurchaseProvisionRequest) (*PurchaseProvisionResult, error)
}

type PaymentGateway interface {
	CreateQris(ctx context.Context, req PakasirCreateTransactionRequest) (*PakasirTransaction, error)
	VerifyTransaction(ctx context.Context, req PakasirVerifyTransactionRequest) (*PakasirVerifiedTransaction, error)
}

type AdminNotifier interface {
	NotifyProvisionFailure(ctx context.Context, message string) error
}
