package adminops

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidRequest      = errors.New("invalid admin request")
	ErrNotFound            = errors.New("resource not found")
	ErrForbidden           = errors.New("forbidden")
	ErrInsufficientBalance = errors.New("insufficient wallet balance")
	ErrInvalidAdjustment   = errors.New("invalid wallet adjustment type")
	ErrInvalidServerCost   = errors.New("invalid server cost input")
	ErrMetricsUnavailable  = errors.New("dashboard metrics unavailable")
)

type DashboardMetricsProvider interface {
	Snapshot(ctx context.Context) DashboardLiveSnapshot
}

type DashboardOverviewResult struct {
	Users         DashboardUserStats     `json:"users"`
	Services      DashboardServiceStats  `json:"services"`
	ResourceUsage DashboardResourceUsage `json:"resource_usage"`
	Revenue       DashboardRevenueStats  `json:"revenue"`
	Alerts        DashboardAlertStats    `json:"alerts"`
	Incus         DashboardIncusStats    `json:"incus"`
}

type DashboardUserStats struct {
	Total  int64 `json:"total"`
	Active int64 `json:"active"`
}

type DashboardServiceStats struct {
	Active              int64 `json:"active"`
	RunningContainers   int64 `json:"running_containers"`
	StoppedContainers   int64 `json:"stopped_containers"`
	ProvisioningPending int64 `json:"provisioning_pending"`
}

type DashboardResourceUsage struct {
	Allocated DashboardAllocatedResources `json:"allocated"`
	Live      DashboardLiveResources      `json:"live"`
}

type DashboardAllocatedResources struct {
	CPUCores int64 `json:"cpu_cores"`
	RAMMB    int64 `json:"ram_mb"`
	DiskGB   int64 `json:"disk_gb"`
}

type DashboardLiveResources struct {
	CPUUsagePercent *float64   `json:"cpu_usage_percent"`
	RAMUsageBytes   int64      `json:"ram_usage_bytes"`
	DiskUsageBytes  int64      `json:"disk_usage_bytes"`
	WarmingUp       bool       `json:"warming_up"`
	SampledAt       *time.Time `json:"sampled_at,omitempty"`
	InstanceCount   int64      `json:"instance_count"`
}

type DashboardRevenueStats struct {
	GrossRevenue   int64 `json:"gross_revenue"`
	RefundedAmount int64 `json:"refunded_amount"`
	NetRevenue     int64 `json:"net_revenue"`
}

type DashboardAlertStats struct {
	OpenCount       int64              `json:"open_count"`
	LatestOpenItems []AlertSummaryItem `json:"latest_open_items"`
}

type DashboardIncusStats struct {
	LiveAvailable bool       `json:"live_available"`
	LastSampleAt  *time.Time `json:"last_sample_at,omitempty"`
	Warning       *string    `json:"warning,omitempty"`
}

type DashboardLiveSnapshot struct {
	LiveAvailable   bool
	LastSampleAt    *time.Time
	Warning         *string
	CPUUsagePercent *float64
	RAMUsageBytes   int64
	DiskUsageBytes  int64
	WarmingUp       bool
	InstanceCount   int64
}

type FinanceSummaryResult struct {
	GrossRevenue          int64      `json:"gross_revenue"`
	RefundedAmount        int64      `json:"refunded_amount"`
	NetRevenue            int64      `json:"net_revenue"`
	TotalServerCost       int64      `json:"total_server_cost"`
	DifferenceToBreakEven int64      `json:"difference_to_break_even"`
	IsBreakEven           bool       `json:"is_break_even"`
	ServerCostCount       int64      `json:"server_cost_count"`
	LastRecordedAt        *time.Time `json:"last_recorded_at,omitempty"`
}

type ServerCostItem struct {
	ID             string            `json:"id"`
	NodeID         string            `json:"node_id"`
	PurchaseCost   int64             `json:"purchase_cost"`
	Notes          *string           `json:"notes,omitempty"`
	RecordedAt     time.Time         `json:"recorded_at"`
	CreatedAt      time.Time         `json:"created_at"`
	Node           AdminNodeSummary  `json:"node"`
	CreatedByAdmin *AdminUserSummary `json:"created_by_admin,omitempty"`
}

type AdminNodeSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	PublicIP    string `json:"public_ip"`
	Status      string `json:"status"`
}

type ServerCostListResult struct {
	Items []ServerCostItem `json:"items"`
}

type CreateServerCostInput struct {
	NodeID       string
	PurchaseCost int64
	Notes        *string
	RecordedAt   *time.Time
}

type WalletAdjustmentInput struct {
	UserID         string
	AdjustmentType string
	Amount         int64
	Reason         string
}

type WalletAdjustmentResult struct {
	User        WalletUserSummary       `json:"user"`
	Wallet      AdminWalletSummary      `json:"wallet"`
	Adjustment  WalletAdjustmentSummary `json:"adjustment"`
	PerformedBy AdminUserSummary        `json:"performed_by"`
}

type WalletAdjustmentSummary struct {
	TransactionID string    `json:"transaction_id"`
	Direction     string    `json:"direction"`
	Amount        int64     `json:"amount"`
	BalanceBefore int64     `json:"balance_before"`
	BalanceAfter  int64     `json:"balance_after"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
}

type AdminUserSummary struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type WalletUserSummary struct {
	ID               string    `json:"id"`
	TelegramID       int64     `json:"telegram_id"`
	TelegramUsername *string   `json:"telegram_username,omitempty"`
	DisplayName      string    `json:"display_name"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

type AdminWalletSummary struct {
	ID      string `json:"id"`
	Balance int64  `json:"balance"`
}

type ListActivityLogsInput struct {
	Page       int
	Limit      int
	ActorType  string
	Action     string
	TargetType string
	TargetID   string
}

type ActivityLogListResult struct {
	Items      []ActivityLogItem `json:"items"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	TotalItems int64             `json:"total_items"`
	TotalPages int               `json:"total_pages"`
}

type ActivityLogItem struct {
	ID           string            `json:"id"`
	ActorType    string            `json:"actor_type"`
	ActorID      *string           `json:"actor_id,omitempty"`
	Action       string            `json:"action"`
	TargetType   string            `json:"target_type"`
	TargetID     *string           `json:"target_id,omitempty"`
	Metadata     map[string]any    `json:"metadata"`
	CreatedAt    time.Time         `json:"created_at"`
	ActorSummary *ActivityActorRef `json:"actor_summary,omitempty"`
}

type ActivityActorRef struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	Email            *string   `json:"email,omitempty"`
	Role             *string   `json:"role,omitempty"`
	TelegramID       *int64    `json:"telegram_id,omitempty"`
	TelegramUsername *string   `json:"telegram_username,omitempty"`
	DisplayName      *string   `json:"display_name,omitempty"`
	Status           *string   `json:"status,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type ListAlertsInput struct {
	Page      int
	Limit     int
	Status    string
	AlertType string
	ServiceID string
	NodeID    string
}

type AlertListResult struct {
	Items      []AlertSummaryItem `json:"items"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalItems int64              `json:"total_items"`
	TotalPages int                `json:"total_pages"`
}

type AlertSummaryItem struct {
	ID               string               `json:"id"`
	AlertType        string               `json:"alert_type"`
	ThresholdPercent int                  `json:"threshold_percent"`
	DurationMinutes  int                  `json:"duration_minutes"`
	Status           string               `json:"status"`
	OpenedAt         time.Time            `json:"opened_at"`
	ResolvedAt       *time.Time           `json:"resolved_at,omitempty"`
	ServiceSummary   *AlertServiceSummary `json:"service_summary,omitempty"`
	NodeSummary      *AdminNodeSummary    `json:"node_summary,omitempty"`
	Metadata         map[string]any       `json:"metadata"`
}

type AlertServiceSummary struct {
	ID                string  `json:"id"`
	OwnerUserID       string  `json:"owner_user_id"`
	OwnerDisplayName  *string `json:"owner_display_name,omitempty"`
	OwnerTelegramID   *int64  `json:"owner_telegram_id,omitempty"`
	PackageName       string  `json:"package_name"`
	ServiceStatus     string  `json:"service_status"`
	ContainerID       *string `json:"container_id,omitempty"`
	IncusInstanceName *string `json:"incus_instance_name,omitempty"`
}

type AlertMonitorConfig struct {
	Interval         time.Duration
	ThresholdPercent int
	Duration         time.Duration
}

type AlertNotifier interface {
	NotifyAlert(ctx context.Context, message string) error
}

type AlertNotifierFunc func(ctx context.Context, message string) error

func (f AlertNotifierFunc) NotifyAlert(ctx context.Context, message string) error {
	return f(ctx, message)
}
