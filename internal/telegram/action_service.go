package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/incus"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"gorm.io/gorm"
)

const (
	actorTypeTelegramUser = "user"
	defaultSSHUsername    = "root"
)

var (
	ErrInvalidActionRequest     = errors.New("invalid action request")
	ErrIncusUnavailable         = errors.New("incus unavailable")
	ErrActionConflict           = errors.New("conflicting action in progress")
	ErrContainerNotRunning      = errors.New("container is not running")
	ErrUnsupportedPayment       = errors.New("unsupported payment method")
	ErrInsufficientBalance      = errors.New("insufficient wallet balance")
	ErrUpgradePackageNotFound   = errors.New("upgrade package not found")
	ErrUpgradePackageIneligible = errors.New("upgrade package is not eligible")
	ErrTransferTargetNotFound   = errors.New("transfer target not found")
	ErrTransferTargetSame       = errors.New("transfer target cannot be the same user")
	ErrInvalidDomain            = errors.New("invalid domain")
	ErrDomainAlreadyExists      = errors.New("domain already exists")
	ErrServiceNotOperable       = errors.New("service is not operable")
	ErrNoActiveSSHMapping       = errors.New("ssh mapping not found")
)

var domainPattern = regexp.MustCompile(`^(?i)[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

type actionExecutor interface {
	ChangeState(instanceName string, action string) (string, error)
	ChangePassword(instanceName string, password string) (string, error)
	ResetSSH(instanceName string) (string, string, error)
	Reinstall(instanceName string, imageAlias string) (string, error)
	ApplyResourceLimits(instanceName string, cpu int, ramMB int, diskGB int) (string, error)
	DeleteInstance(instanceName string) (string, error)
}

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
	ContainerID string `json:"container_id"`
	ServiceID   string `json:"service_id"`
	Domain      string `json:"domain"`
	TargetPort  int    `json:"target_port"`
	ProxyMode   string `json:"proxy_mode"`
	Available   bool   `json:"available"`
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
	JobID       string              `json:"job_id"`
	Status      string              `json:"status"`
}

type ReconfigIPPreviewInput struct {
	TelegramID  int64
	ContainerID string
}

type ReconfigIPPreviewResult struct {
	ContainerID     string  `json:"container_id"`
	ServiceID       string  `json:"service_id"`
	MainPublicIP    *string `json:"main_public_ip,omitempty"`
	CurrentSSHPort  *int    `json:"current_ssh_port,omitempty"`
	ProposedSSHPort int     `json:"proposed_ssh_port"`
}

type ReconfigIPSubmitInput struct {
	TelegramID       int64
	ContainerID      string
	RequestedSSHPort *int
}

type ReconfigIPSubmitResult struct {
	ActionAcceptedResult
	MainPublicIP *string `json:"main_public_ip,omitempty"`
	SSHPort      *int    `json:"ssh_port,omitempty"`
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

type managedService struct {
	User     *model.User
	Wallet   *model.Wallet
	Instance *model.ServiceInstance
	Service  *model.Service
	Package  *model.Package
}

func (s *Service) RuntimeAction(ctx context.Context, input RuntimeActionInput) (*ActionAcceptedResult, error) {
	if input.TelegramID <= 0 || strings.TrimSpace(input.ContainerID) == "" {
		return nil, ErrInvalidActionRequest
	}

	action := normalizeRuntimeAction(input.Action)
	if action == "" {
		return nil, ErrInvalidActionRequest
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := ensureServiceOperable(ms.Service); err != nil {
		return nil, err
	}

	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}

	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	opID, err := s.actions.ChangeState(ms.Instance.IncusInstanceName, action)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	instanceStatus, serviceStatus := runtimeStatuses(action)
	job := successJob(ms.Service.ID, nil, actionToJobType(action), actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
		"action":              action,
		"container_id":        ms.Instance.ID,
		"incus_instance_name": ms.Instance.IncusInstanceName,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: runtimeEventType(action),
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   runtimeSummary(action, ms.Instance.IncusInstanceName),
		Payload: map[string]any{
			"action":       action,
			"container_id": ms.Instance.ID,
			"operation_id": opID,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", ms.Instance.ID).
			Updates(map[string]any{
				"status":                  instanceStatus,
				"last_incus_operation_id": opID,
				"updated_at":              now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.Service{}).
			Where("id = ?", ms.Service.ID).
			Updates(map[string]any{
				"status":       serviceStatus,
				"suspended_at": nil,
				"updated_at":   now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Create(job).Error; err != nil {
			return err
		}

		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	return &ActionAcceptedResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Action:      action,
		Accepted:    true,
		OperationID: &opID,
		JobID:       &job.ID,
		Status:      "completed",
	}, nil
}

func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) (*ChangePasswordResult, error) {
	if input.TelegramID <= 0 || strings.TrimSpace(input.ContainerID) == "" || len(strings.TrimSpace(input.NewPassword)) < 8 {
		return nil, ErrInvalidActionRequest
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if !isRunningStatus(ms.Instance.Status) {
		return nil, ErrContainerNotRunning
	}

	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	opID, err := s.actions.ChangePassword(ms.Instance.IncusInstanceName, strings.TrimSpace(input.NewPassword))
	if err != nil {
		return nil, err
	}
	ms.Instance.LastIncusOperationID = &opID

	job := successJob(ms.Service.ID, nil, "change_password", actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
		"container_id": ms.Instance.ID,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "password_changed",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "password layanan berhasil diubah",
		Payload: map[string]any{
			"container_id": ms.Instance.ID,
			"operation_id": opID,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.persistActionArtifacts(ctx, ms.Instance, nil, job, event); err != nil {
		return nil, err
	}

	return &ChangePasswordResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "change_password",
			Accepted:    true,
			OperationID: &opID,
			JobID:       &job.ID,
			Status:      "completed",
		},
	}, nil
}

func (s *Service) ResetSSH(ctx context.Context, input ResetSSHInput) (*ResetSSHResult, error) {
	if input.TelegramID <= 0 || strings.TrimSpace(input.ContainerID) == "" {
		return nil, ErrInvalidActionRequest
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if !isRunningStatus(ms.Instance.Status) {
		return nil, ErrContainerNotRunning
	}

	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	opID, password, err := s.actions.ResetSSH(ms.Instance.IncusInstanceName)
	if err != nil {
		return nil, err
	}
	ms.Instance.LastIncusOperationID = &opID

	job := successJob(ms.Service.ID, nil, "change_password", actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
		"container_id": ms.Instance.ID,
		"kind":         "ssh_reset",
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "ssh_reset",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "akses SSH berhasil direset",
		Payload: map[string]any{
			"container_id": ms.Instance.ID,
			"operation_id": opID,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.persistActionArtifacts(ctx, ms.Instance, nil, job, event); err != nil {
		return nil, err
	}

	return &ResetSSHResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "ssh_reset",
			Accepted:    true,
			OperationID: &opID,
			JobID:       &job.ID,
			Status:      "completed",
		},
		Username:    defaultSSHUsername,
		NewPassword: password,
	}, nil
}

func (s *Service) RenewPreview(ctx context.Context, input RenewPreviewInput) (*RenewPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := ensureServiceOperable(ms.Service); err != nil {
		return nil, err
	}

	nextExpiresAt := extendExpiry(ms.Service.ExpiresAt, ms.Package.DurationDays)

	return &RenewPreviewResult{
		ContainerID:      ms.Instance.ID,
		ServiceID:        ms.Service.ID,
		Package:          packageQuote(ms.Package),
		Price:            ms.Package.Price,
		CurrentExpiresAt: ms.Service.ExpiresAt,
		NextExpiresAt:    nextExpiresAt,
		PaymentMethods:   []string{"wallet", "qris"},
	}, nil
}

func (s *Service) RenewSubmit(ctx context.Context, input RenewSubmitInput) (*BillingSubmitResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := ensureServiceOperable(ms.Service); err != nil {
		return nil, err
	}

	if err := validatePaymentMethod(input.PaymentMethod); err != nil {
		return nil, err
	}

	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}

	nextExpiresAt := extendExpiry(ms.Service.ExpiresAt, ms.Package.DurationDays)
	if nextExpiresAt == nil {
		return nil, ErrInvalidActionRequest
	}

	now := time.Now().UTC()
	order := buildOrder(ms.User.ID, ms.Package, "renewal", &ms.Service.ID, ms.Package.Price, nil, now)
	order.PaymentMethod = &input.PaymentMethod
	payment := buildPayment(&order.ID, "order", input.PaymentMethod, ms.Package.Price, now)

	result := &BillingSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		OrderID:     order.ID,
		PaymentID:   payment.ID,
		Action:      "renew",
		Amount:      ms.Package.Price,
		Applied:     false,
		Status:      "awaiting_payment",
		ExpiresAt:   ms.Service.ExpiresAt,
	}

	switch input.PaymentMethod {
	case "wallet":
		if ms.Wallet == nil || ms.Wallet.Balance < ms.Package.Price {
			return nil, ErrInsufficientBalance
		}

		balanceBefore := ms.Wallet.Balance
		balanceAfter := balanceBefore - ms.Package.Price
		order.Status = "completed"
		order.PaymentStatus = "paid"
		order.PaidAt = &now
		order.UpdatedAt = now
		payment.Status = "paid"
		payment.PaidAt = &now
		payment.UpdatedAt = now

		sourceOrderID := order.ID
		note := "renewal payment"
		walletTxn := &model.WalletTransaction{
			ID:              uuid.NewString(),
			WalletID:        ms.Wallet.ID,
			UserID:          ms.User.ID,
			Direction:       "debit",
			TransactionType: "payment",
			Amount:          ms.Package.Price,
			BalanceBefore:   balanceBefore,
			BalanceAfter:    balanceAfter,
			SourceType:      "order",
			SourceID:        &sourceOrderID,
			Note:            &note,
			CreatedAt:       now,
		}
		event := &model.ServiceEvent{
			ID:        uuid.NewString(),
			ServiceID: ms.Service.ID,
			EventType: "service_renewed",
			ActorType: actorTypeTelegramUser,
			ActorID:   &ms.User.ID,
			Summary:   "layanan berhasil diperpanjang",
			Payload: map[string]any{
				"order_id":   order.ID,
				"payment_id": payment.ID,
				"old_expiry": ms.Service.ExpiresAt,
				"new_expiry": nextExpiresAt,
				"amount":     ms.Package.Price,
			},
			CreatedAt: now,
		}

		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Wallet{}).
				Where("id = ?", ms.Wallet.ID).
				Update("balance", balanceAfter).Error; err != nil {
				return err
			}
			if err := tx.Create(walletTxn).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Service{}).
				Where("id = ?", ms.Service.ID).
				Updates(map[string]any{
					"expires_at": nextExpiresAt,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
			return tx.Create(event).Error
		}); err != nil {
			return nil, err
		}

		result.Applied = true
		result.Status = "completed"
		result.ExpiresAt = nextExpiresAt
	case "qris":
		order.Status = "awaiting_payment"
		order.UpdatedAt = now
		payment.Status = "pending"
		payment.UpdatedAt = now

		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			return tx.Create(&payment).Error
		}); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (s *Service) UpgradeOptions(ctx context.Context, input UpgradeOptionsInput) (*UpgradeOptionsResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	packages, err := s.repo.FindActivePackages(ctx)
	if err != nil {
		return nil, err
	}

	result := &UpgradeOptionsResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Current:     packageQuote(ms.Package),
		Packages:    make([]PackageQuote, 0),
	}

	for i := range packages {
		if isEligibleUpgrade(ms.Service, &packages[i]) {
			result.Packages = append(result.Packages, packageQuote(&packages[i]))
		}
	}

	return result, nil
}

func (s *Service) UpgradePreview(ctx context.Context, input UpgradePreviewInput) (*UpgradePreviewResult, error) {
	if strings.TrimSpace(input.TargetPackageID) == "" {
		return nil, ErrInvalidActionRequest
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	targetPackage, err := s.repo.FindPackageByID(ctx, input.TargetPackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpgradePackageNotFound
		}
		return nil, err
	}

	if !targetPackage.IsActive || !isEligibleUpgrade(ms.Service, targetPackage) {
		return nil, ErrUpgradePackageIneligible
	}

	remainingSeconds := remainingDurationSeconds(ms.Service.ExpiresAt)
	amount := proratedUpgradeCost(ms.Service.PriceSnapshot, targetPackage.Price, remainingSeconds, ms.Service.BillingCycleDays)
	if amount <= 0 {
		return nil, ErrUpgradePackageIneligible
	}

	return &UpgradePreviewResult{
		ContainerID:      ms.Instance.ID,
		ServiceID:        ms.Service.ID,
		Current:          packageQuote(ms.Package),
		Target:           packageQuote(targetPackage),
		CurrentExpiresAt: ms.Service.ExpiresAt,
		RemainingSeconds: remainingSeconds,
		PriceDifference:  amount,
		PaymentMethods:   []string{"wallet", "qris"},
	}, nil
}

func (s *Service) UpgradeSubmit(ctx context.Context, input UpgradeSubmitInput) (*BillingSubmitResult, error) {
	if strings.TrimSpace(input.TargetPackageID) == "" {
		return nil, ErrInvalidActionRequest
	}

	if err := validatePaymentMethod(input.PaymentMethod); err != nil {
		return nil, err
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}

	targetPackage, err := s.repo.FindPackageByID(ctx, input.TargetPackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpgradePackageNotFound
		}
		return nil, err
	}

	if !targetPackage.IsActive || !isEligibleUpgrade(ms.Service, targetPackage) {
		return nil, ErrUpgradePackageIneligible
	}

	remainingSeconds := remainingDurationSeconds(ms.Service.ExpiresAt)
	amount := proratedUpgradeCost(ms.Service.PriceSnapshot, targetPackage.Price, remainingSeconds, ms.Service.BillingCycleDays)
	if amount <= 0 {
		return nil, ErrUpgradePackageIneligible
	}

	now := time.Now().UTC()
	order := buildOrder(ms.User.ID, targetPackage, "upgrade", &ms.Service.ID, amount, nil, now)
	order.PaymentMethod = &input.PaymentMethod
	payment := buildPayment(&order.ID, "order", input.PaymentMethod, amount, now)

	result := &BillingSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		OrderID:     order.ID,
		PaymentID:   payment.ID,
		Action:      "upgrade",
		Amount:      amount,
		Applied:     false,
		Status:      "awaiting_payment",
		ExpiresAt:   ms.Service.ExpiresAt,
	}

	switch input.PaymentMethod {
	case "wallet":
		if ms.Wallet == nil || ms.Wallet.Balance < amount {
			return nil, ErrInsufficientBalance
		}
		if s.actions == nil {
			return nil, ErrIncusUnavailable
		}

		opID, err := s.actions.ApplyResourceLimits(ms.Instance.IncusInstanceName, targetPackage.CPU, targetPackage.RAMMB, targetPackage.DiskGB)
		if err != nil {
			return nil, err
		}

		order.Status = "completed"
		order.PaymentStatus = "paid"
		order.PaidAt = &now
		payment.Status = "paid"
		payment.PaidAt = &now
		sourceOrderID := order.ID
		note := "upgrade payment"
		walletTxn := &model.WalletTransaction{
			ID:              uuid.NewString(),
			WalletID:        ms.Wallet.ID,
			UserID:          ms.User.ID,
			Direction:       "debit",
			TransactionType: "payment",
			Amount:          amount,
			BalanceBefore:   ms.Wallet.Balance,
			BalanceAfter:    ms.Wallet.Balance - amount,
			SourceType:      "order",
			SourceID:        &sourceOrderID,
			Note:            &note,
			CreatedAt:       now,
		}
		job := successJob(ms.Service.ID, &order.ID, "upgrade", actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
			"container_id":      ms.Instance.ID,
			"target_package_id": targetPackage.ID,
		})
		event := &model.ServiceEvent{
			ID:        uuid.NewString(),
			ServiceID: ms.Service.ID,
			EventType: "service_upgraded",
			ActorType: actorTypeTelegramUser,
			ActorID:   &ms.User.ID,
			Summary:   "resource layanan berhasil di-upgrade",
			Payload: map[string]any{
				"order_id":            order.ID,
				"payment_id":          payment.ID,
				"target_package_id":   targetPackage.ID,
				"target_package_name": targetPackage.Name,
				"operation_id":        opID,
			},
			CreatedAt: now,
		}

		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Wallet{}).Where("id = ?", ms.Wallet.ID).Update("balance", ms.Wallet.Balance-amount).Error; err != nil {
				return err
			}
			if err := tx.Create(walletTxn).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Service{}).
				Where("id = ?", ms.Service.ID).
				Updates(map[string]any{
					"current_package_id":    targetPackage.ID,
					"package_name_snapshot": targetPackage.Name,
					"cpu_snapshot":          targetPackage.CPU,
					"ram_mb_snapshot":       targetPackage.RAMMB,
					"disk_gb_snapshot":      targetPackage.DiskGB,
					"price_snapshot":        targetPackage.Price,
					"updated_at":            now,
				}).Error; err != nil {
				return err
			}
			if err := tx.Create(job).Error; err != nil {
				return err
			}
			return tx.Create(event).Error
		}); err != nil {
			return nil, err
		}

		result.Applied = true
		result.Status = "completed"
	case "qris":
		order.Status = "awaiting_payment"
		payment.Status = "pending"
		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			return tx.Create(&payment).Error
		}); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (s *Service) ReinstallOptions(ctx context.Context, input ReinstallOptionsInput) (*ReinstallOptionsResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	return &ReinstallOptionsResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Images:      defaultImageOptions(),
	}, nil
}

func (s *Service) ReinstallPreview(ctx context.Context, input ReinstallPreviewInput) (*ReinstallPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	image, ok := findImageOption(input.ImageAlias)
	if !ok {
		return nil, ErrInvalidActionRequest
	}

	return &ReinstallPreviewResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Image:       image,
		Warning:     "reinstall akan menghapus data di dalam container",
	}, nil
}

func (s *Service) ReinstallSubmit(ctx context.Context, input ReinstallSubmitInput) (*ReinstallSubmitResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}

	if _, ok := findImageOption(input.ImageAlias); !ok {
		return nil, ErrInvalidActionRequest
	}

	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	reinstallOpID, err := s.actions.Reinstall(ms.Instance.IncusInstanceName, input.ImageAlias)
	if err != nil {
		return nil, err
	}

	_, _ = s.actions.ChangeState(ms.Instance.IncusInstanceName, "start")
	passOpID, newPassword, err := s.actions.ResetSSH(ms.Instance.IncusInstanceName)
	if err != nil {
		return nil, err
	}

	job := successJob(ms.Service.ID, nil, "reinstall", actorTypeTelegramUser, ms.User.ID, reinstallOpID, map[string]any{
		"container_id":       ms.Instance.ID,
		"image_alias":        input.ImageAlias,
		"password_operation": passOpID,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "service_reinstalled",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "layanan berhasil di-reinstall",
		Payload: map[string]any{
			"container_id":       ms.Instance.ID,
			"image_alias":        input.ImageAlias,
			"operation_id":       reinstallOpID,
			"password_operation": passOpID,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", ms.Instance.ID).
			Updates(map[string]any{
				"image_alias": input.ImageAlias,
				"status":      "running",
				"updated_at":  time.Now().UTC(),
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Service{}).Where("id = ?", ms.Service.ID).Update("status", "active").Error; err != nil {
			return err
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	return &ReinstallSubmitResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "reinstall",
			Accepted:    true,
			OperationID: &reinstallOpID,
			JobID:       &job.ID,
			Status:      "completed",
		},
		Username:    defaultSSHUsername,
		NewPassword: newPassword,
		ImageAlias:  input.ImageAlias,
	}, nil
}

func (s *Service) DomainPreview(ctx context.Context, input DomainPreviewInput) (*DomainPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	domain, err := normalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	if !isValidProxyMode(input.ProxyMode) || input.TargetPort < 1 || input.TargetPort > 65535 {
		return nil, ErrInvalidActionRequest
	}

	exists, err := s.repo.DomainExists(ctx, domain)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDomainAlreadyExists
	}

	return &DomainPreviewResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Domain:      domain,
		TargetPort:  input.TargetPort,
		ProxyMode:   input.ProxyMode,
		Available:   true,
	}, nil
}

func (s *Service) DomainSubmit(ctx context.Context, input DomainSubmitInput) (*DomainSubmitResult, error) {
	preview, err := s.DomainPreview(ctx, DomainPreviewInput(input))
	if err != nil {
		return nil, err
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	job := queuedJob(ms.Service.ID, nil, "reassign_port", actorTypeTelegramUser, ms.User.ID, map[string]any{
		"kind":         "setup_domain",
		"container_id": ms.Instance.ID,
		"domain":       preview.Domain,
		"target_port":  preview.TargetPort,
		"proxy_mode":   preview.ProxyMode,
	})
	record := model.ServiceDomain{
		ID:         uuid.NewString(),
		ServiceID:  ms.Service.ID,
		Domain:     preview.Domain,
		TargetPort: preview.TargetPort,
		ProxyMode:  preview.ProxyMode,
		Status:     "pending",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	event := model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "domain_setup_requested",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "permintaan setup domain dibuat",
		Payload: map[string]any{
			"domain":      record.Domain,
			"target_port": record.TargetPort,
			"proxy_mode":  record.ProxyMode,
			"job_id":      job.ID,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(&event).Error
	}); err != nil {
		return nil, err
	}

	return &DomainSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Domain:      record,
		JobID:       job.ID,
		Status:      "queued",
	}, nil
}

func (s *Service) ReconfigIPPreview(ctx context.Context, input ReconfigIPPreviewInput) (*ReconfigIPPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	port, err := s.repo.FindNextAvailableSSHPort(ctx)
	if err != nil {
		return nil, err
	}

	return &ReconfigIPPreviewResult{
		ContainerID:     ms.Instance.ID,
		ServiceID:       ms.Service.ID,
		MainPublicIP:    ms.Instance.MainPublicIP,
		CurrentSSHPort:  ms.Instance.SSHPort,
		ProposedSSHPort: port,
	}, nil
}

func (s *Service) ReconfigIPSubmit(ctx context.Context, input ReconfigIPSubmitInput) (*ReconfigIPSubmitResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if ms.Instance.MainPublicIP == nil || ms.Instance.InternalIP == nil {
		return nil, ErrInvalidActionRequest
	}

	newPort := 0
	if input.RequestedSSHPort != nil {
		newPort = *input.RequestedSSHPort
	} else {
		newPort, err = s.repo.FindNextAvailableSSHPort(ctx)
		if err != nil {
			return nil, err
		}
	}
	if newPort < 1 || newPort > 65535 {
		return nil, ErrInvalidActionRequest
	}

	now := time.Now().UTC()
	job := successJob(ms.Service.ID, nil, "reassign_port", actorTypeTelegramUser, ms.User.ID, "", map[string]any{
		"container_id":   ms.Instance.ID,
		"old_ssh_port":   ms.Instance.SSHPort,
		"new_ssh_port":   newPort,
		"main_public_ip": ms.Instance.MainPublicIP,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "ip_reconfigured",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "akses NAT utama berhasil diubah",
		Payload: map[string]any{
			"container_id": ms.Instance.ID,
			"new_ssh_port": newPort,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", ms.Instance.ID).
			Updates(map[string]any{
				"ssh_port":   newPort,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		var mapping model.ServicePortMapping
		err := tx.Where("service_id = ? AND mapping_type = ? AND is_active = ?", ms.Service.ID, "ssh", true).First(&mapping).Error
		if err == nil {
			if err := tx.Model(&model.ServicePortMapping{}).
				Where("id = ?", mapping.ID).
				Updates(map[string]any{
					"public_port": newPort,
					"updated_at":  now,
				}).Error; err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	return &ReconfigIPSubmitResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "reconfig_ip",
			Accepted:    true,
			JobID:       &job.ID,
			Status:      "completed",
		},
		MainPublicIP: ms.Instance.MainPublicIP,
		SSHPort:      &newPort,
	}, nil
}

func (s *Service) TransferPreview(ctx context.Context, input TransferPreviewInput) (*TransferPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	target, err := s.repo.FindUserByTelegramID(ctx, input.TargetTelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransferTargetNotFound
		}
		return nil, err
	}
	if target.ID == ms.User.ID {
		return nil, ErrTransferTargetSame
	}

	return &TransferPreviewResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		FromUser:    transferUser(ms.User),
		ToUser:      transferUser(target),
	}, nil
}

func (s *Service) TransferSubmit(ctx context.Context, input TransferSubmitInput) (*TransferSubmitResult, error) {
	preview, err := s.TransferPreview(ctx, TransferPreviewInput{
		TelegramID:       input.TelegramID,
		ContainerID:      input.ContainerID,
		TargetTelegramID: input.TargetTelegramID,
	})
	if err != nil {
		return nil, err
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}
	target, err := s.repo.FindUserByTelegramID(ctx, input.TargetTelegramID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	transfer := &model.ServiceTransfer{
		ID:            uuid.NewString(),
		ServiceID:     ms.Service.ID,
		FromUserID:    ms.User.ID,
		ToUserID:      target.ID,
		Reason:        normalizeOptionalString(input.Reason),
		CreatedByType: actorTypeTelegramUser,
		CreatedByID:   &ms.User.ID,
		CreatedAt:     now,
	}
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "service_transferred",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "kepemilikan layanan berhasil dipindahkan",
		Payload: map[string]any{
			"from_user_id": ms.User.ID,
			"to_user_id":   target.ID,
			"reason":       transfer.Reason,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Service{}).
			Where("id = ?", ms.Service.ID).
			Updates(map[string]any{
				"owner_user_id": target.ID,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Create(transfer).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	return &TransferSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		FromUser:    preview.FromUser,
		ToUser:      preview.ToUser,
	}, nil
}

func (s *Service) CancelPreview(ctx context.Context, input CancelPreviewInput) (*CancelPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	remaining := remainingDurationSeconds(ms.Service.ExpiresAt)
	return &CancelPreviewResult{
		ContainerID:      ms.Instance.ID,
		ServiceID:        ms.Service.ID,
		PackageName:      ms.Service.PackageNameSnapshot,
		CurrentExpiresAt: ms.Service.ExpiresAt,
		RemainingSeconds: remaining,
		RefundAmount:     proratedRefund(ms.Service.PriceSnapshot, remaining, ms.Service.BillingCycleDays),
	}, nil
}

func (s *Service) CancelSubmit(ctx context.Context, input CancelSubmitInput) (*CancelSubmitResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}

	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	refund := proratedRefund(ms.Service.PriceSnapshot, remainingDurationSeconds(ms.Service.ExpiresAt), ms.Service.BillingCycleDays)
	opID, err := s.actions.DeleteInstance(ms.Instance.IncusInstanceName)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	sourceServiceID := ms.Service.ID
	note := "service cancellation refund"
	job := successJob(ms.Service.ID, nil, "cancel", actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
		"container_id": ms.Instance.ID,
		"refund":       refund,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "service_canceled",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "layanan berhasil dibatalkan",
		Payload: map[string]any{
			"container_id": ms.Instance.ID,
			"refund":       refund,
			"operation_id": opID,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if refund > 0 && ms.Wallet != nil {
			if err := tx.Model(&model.Wallet{}).
				Where("id = ?", ms.Wallet.ID).
				Update("balance", ms.Wallet.Balance+refund).Error; err != nil {
				return err
			}
			txRecord := &model.WalletTransaction{
				ID:              uuid.NewString(),
				WalletID:        ms.Wallet.ID,
				UserID:          ms.User.ID,
				Direction:       "credit",
				TransactionType: "refund",
				Amount:          refund,
				BalanceBefore:   ms.Wallet.Balance,
				BalanceAfter:    ms.Wallet.Balance + refund,
				SourceType:      "service",
				SourceID:        &sourceServiceID,
				Note:            &note,
				CreatedAt:       now,
			}
			if err := tx.Create(txRecord).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&model.Service{}).
			Where("id = ?", ms.Service.ID).
			Updates(map[string]any{
				"status":        "terminated",
				"canceled_at":   now,
				"terminated_at": now,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", ms.Instance.ID).
			Updates(map[string]any{
				"status":                  "deleted",
				"last_incus_operation_id": opID,
				"updated_at":              now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	return &CancelSubmitResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "cancel",
			Accepted:    true,
			OperationID: &opID,
			JobID:       &job.ID,
			Status:      "completed",
		},
		RefundAmount: refund,
	}, nil
}

func (s *Service) loadManagedService(ctx context.Context, telegramID int64, containerID string) (*managedService, error) {
	if telegramID <= 0 || strings.TrimSpace(containerID) == "" {
		return nil, ErrInvalidActionRequest
	}

	user, err := s.repo.FindUserByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}
		return nil, err
	}

	instance, err := s.repo.FindUserServiceInstanceByID(ctx, telegramID, containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMyVPSNotFound
		}
		return nil, err
	}

	if instance.Service == nil {
		return nil, ErrMyVPSNotFound
	}

	pkg := instance.Service.CurrentPack
	if pkg == nil {
		return nil, ErrInvalidActionRequest
	}

	return &managedService{
		User:     user,
		Wallet:   user.Wallet,
		Instance: instance,
		Service:  instance.Service,
		Package:  pkg,
	}, nil
}

func (s *Service) ensureNoConflictingJobs(ctx context.Context, serviceID string) error {
	hasConflict, err := s.repo.HasConflictingJob(ctx, serviceID)
	if err != nil {
		return err
	}
	if hasConflict {
		return ErrActionConflict
	}
	return nil
}

func ensureServiceOperable(service *model.Service) error {
	if service == nil {
		return ErrServiceNotOperable
	}

	switch service.Status {
	case "active", "stopped", "suspended":
		return nil
	default:
		return ErrServiceNotOperable
	}
}

func normalizeRuntimeAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start":
		return "start"
	case "stop":
		return "stop"
	case "reboot", "restart":
		return "restart"
	default:
		return ""
	}
}

func runtimeStatuses(action string) (string, string) {
	switch action {
	case "start":
		return "running", "active"
	case "stop":
		return "stopped", "stopped"
	case "restart":
		return "running", "active"
	default:
		return "pending", "pending"
	}
}

func runtimeEventType(action string) string {
	switch action {
	case "start":
		return "container_started"
	case "stop":
		return "container_stopped"
	default:
		return "container_restarted"
	}
}

func runtimeSummary(action string, instanceName string) string {
	switch action {
	case "start":
		return fmt.Sprintf("container %s berhasil dijalankan", instanceName)
	case "stop":
		return fmt.Sprintf("container %s berhasil dihentikan", instanceName)
	default:
		return fmt.Sprintf("container %s berhasil direstart", instanceName)
	}
}

func actionToJobType(action string) string {
	switch action {
	case "restart":
		return "restart"
	default:
		return action
	}
}

func packageQuote(pkg *model.Package) PackageQuote {
	return PackageQuote{
		ID:           pkg.ID,
		Name:         pkg.Name,
		CPU:          pkg.CPU,
		RAMMB:        pkg.RAMMB,
		DiskGB:       pkg.DiskGB,
		Price:        pkg.Price,
		DurationDays: pkg.DurationDays,
	}
}

func extendExpiry(current *time.Time, durationDays int) *time.Time {
	base := time.Now().UTC()
	if current != nil && current.After(base) {
		base = *current
	}

	next := base.Add(time.Duration(durationDays) * 24 * time.Hour)
	return &next
}

func validatePaymentMethod(method string) error {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "wallet", "qris":
		return nil
	default:
		return ErrUnsupportedPayment
	}
}

func buildOrder(userID string, pkg *model.Package, orderType string, targetServiceID *string, totalAmount int64, selectedImage *string, now time.Time) model.Order {
	return model.Order{
		ID:                   uuid.NewString(),
		UserID:               userID,
		PackageID:            pkg.ID,
		TargetServiceID:      targetServiceID,
		OrderType:            orderType,
		Status:               "pending",
		PaymentStatus:        "pending",
		SelectedImageAlias:   selectedImage,
		PackageNameSnapshot:  pkg.Name,
		CPUSnapshot:          pkg.CPU,
		RAMMBSnapshot:        pkg.RAMMB,
		DiskGBSnapshot:       pkg.DiskGB,
		PriceSnapshot:        pkg.Price,
		DurationDaysSnapshot: pkg.DurationDays,
		TotalAmount:          totalAmount,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func buildPayment(orderID *string, purpose string, method string, amount int64, now time.Time) model.Payment {
	return model.Payment{
		ID:        uuid.NewString(),
		OrderID:   orderID,
		Purpose:   purpose,
		Method:    method,
		Amount:    amount,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func isEligibleUpgrade(service *model.Service, pkg *model.Package) bool {
	if service == nil || pkg == nil {
		return false
	}
	if pkg.CPU < service.CPUSnapshot || pkg.RAMMB < service.RAMMBSnapshot || pkg.DiskGB < service.DiskGBSnapshot {
		return false
	}
	if pkg.Price <= service.PriceSnapshot {
		return false
	}
	return pkg.CPU > service.CPUSnapshot || pkg.RAMMB > service.RAMMBSnapshot || pkg.DiskGB > service.DiskGBSnapshot
}

func remainingDurationSeconds(expiresAt *time.Time) int64 {
	if expiresAt == nil || expiresAt.IsZero() {
		return 0
	}
	remaining := int64(time.Until(*expiresAt).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func proratedUpgradeCost(currentPrice int64, targetPrice int64, remainingSeconds int64, billingCycleDays int) int64 {
	if targetPrice <= currentPrice || remainingSeconds <= 0 || billingCycleDays <= 0 {
		return 0
	}
	billingSeconds := int64(billingCycleDays) * 24 * 60 * 60
	if billingSeconds <= 0 {
		return 0
	}
	value := float64(targetPrice-currentPrice) * (float64(remainingSeconds) / float64(billingSeconds))
	return int64(math.Floor(value))
}

func defaultImageOptions() []ImageOption {
	return []ImageOption{
		{Label: "Debian 12", Alias: "images:debian/12"},
		{Label: "Debian 13", Alias: "images:debian/13"},
		{Label: "Ubuntu 22.04", Alias: "images:ubuntu/22.04"},
		{Label: "Ubuntu 24.04", Alias: "images:ubuntu/24.04"},
		{Label: "Kali Linux", Alias: "images:kali/current"},
	}
}

func findImageOption(alias string) (ImageOption, bool) {
	for _, image := range defaultImageOptions() {
		if image.Alias == alias {
			return image, true
		}
	}
	return ImageOption{}, false
}

func normalizeDomain(domain string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	normalized = strings.TrimSuffix(normalized, ".")
	if !domainPattern.MatchString(normalized) {
		return "", ErrInvalidDomain
	}
	return normalized, nil
}

func isValidProxyMode(mode string) bool {
	return slices.Contains([]string{"http", "https", "http_and_https"}, strings.TrimSpace(mode))
}

func transferUser(user *model.User) TransferUser {
	return TransferUser{
		ID:               user.ID,
		TelegramID:       user.TelegramID,
		TelegramUsername: user.TelegramUsername,
		DisplayName:      user.DisplayName,
	}
}

func proratedRefund(currentPrice int64, remainingSeconds int64, billingCycleDays int) int64 {
	if currentPrice <= 0 || remainingSeconds <= 0 || billingCycleDays <= 0 {
		return 0
	}
	billingSeconds := int64(billingCycleDays) * 24 * 60 * 60
	if billingSeconds <= 0 {
		return 0
	}
	value := float64(currentPrice) * (float64(remainingSeconds) / float64(billingSeconds))
	return int64(math.Floor(value))
}

func successJob(serviceID string, orderID *string, jobType string, actorType string, actorID string, operationID string, payload map[string]any) *model.ProvisioningJob {
	now := time.Now().UTC()
	var opID *string
	if strings.TrimSpace(operationID) != "" {
		opID = &operationID
	}
	return &model.ProvisioningJob{
		ID:               uuid.NewString(),
		ServiceID:        &serviceID,
		OrderID:          orderID,
		JobType:          jobType,
		Status:           "success",
		IncusOperationID: opID,
		RequestedByType:  actorType,
		RequestedByID:    &actorID,
		AttemptCount:     1,
		Payload:          payload,
		StartedAt:        &now,
		FinishedAt:       &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func queuedJob(serviceID string, orderID *string, jobType string, actorType string, actorID string, payload map[string]any) *model.ProvisioningJob {
	now := time.Now().UTC()
	return &model.ProvisioningJob{
		ID:              uuid.NewString(),
		ServiceID:       &serviceID,
		OrderID:         orderID,
		JobType:         jobType,
		Status:          "queued",
		RequestedByType: actorType,
		RequestedByID:   &actorID,
		AttemptCount:    0,
		Payload:         payload,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

type incusActionExecutor struct {
	client *incus.Client
}

func newActionExecutor(client *incus.Client) actionExecutor {
	if client == nil {
		return nil
	}
	return &incusActionExecutor{client: client}
}

func (e *incusActionExecutor) ChangeState(instanceName string, action string) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	op, err := server.UpdateInstanceState(instanceName, api.InstanceStatePut{Action: action}, "")
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}
	return op.Get().ID, nil
}

func (e *incusActionExecutor) ChangePassword(instanceName string, password string) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := []string{"sh", "-lc", fmt.Sprintf("echo %s:%s | chpasswd", shellQuote(defaultSSHUsername), shellQuote(password))}
	op, err := server.ExecInstance(instanceName, api.InstanceExecPost{
		Command:      command,
		Interactive:  false,
		RecordOutput: true,
	}, &incusclient.InstanceExecArgs{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}

	return op.Get().ID, nil
}

func (e *incusActionExecutor) ResetSSH(instanceName string) (string, string, error) {
	password := randomPassword(18)
	opID, err := e.ChangePassword(instanceName, password)
	if err != nil {
		return "", "", err
	}

	return opID, password, nil
}

func (e *incusActionExecutor) Reinstall(instanceName string, imageAlias string) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	op, err := server.RebuildInstance(instanceName, api.InstanceRebuildPost{
		Source: api.InstanceSource{
			Type:  "image",
			Alias: imageAlias,
		},
	})
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}

	return op.Get().ID, nil
}

func (e *incusActionExecutor) ApplyResourceLimits(instanceName string, cpu int, ramMB int, diskGB int) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	instance, etag, err := server.GetInstance(instanceName)
	if err != nil {
		return "", err
	}

	writable := instance.Writable()
	if writable.Config == nil {
		writable.Config = api.ConfigMap{}
	}
	writable.Config["limits.cpu"] = strconv.Itoa(cpu)
	writable.Config["limits.memory"] = fmt.Sprintf("%dMiB", ramMB)
	for name, device := range writable.Devices {
		if device["type"] == "disk" && device["path"] == "/" {
			device["size"] = fmt.Sprintf("%dGiB", diskGB)
			writable.Devices[name] = device
			break
		}
	}

	op, err := server.UpdateInstance(instanceName, writable, etag)
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}

	return op.Get().ID, nil
}

func (e *incusActionExecutor) DeleteInstance(instanceName string) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	op, err := server.DeleteInstance(instanceName)
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}

	return op.Get().ID, nil
}

func (e *incusActionExecutor) server() (incusclient.InstanceServer, error) {
	if e == nil || e.client == nil || e.client.Server() == nil {
		return nil, ErrIncusUnavailable
	}
	return e.client.Server(), nil
}

func randomPassword(length int) string {
	const chars = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*"
	var builder strings.Builder
	for i := 0; i < length; i++ {
		builder.WriteByte(chars[rand.IntN(len(chars))])
	}
	return builder.String()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func isRunningStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "running")
}
