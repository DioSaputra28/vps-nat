package telegram

import (
	"context"
	"errors"
	"strings"

	"github.com/DioSaputra28/vps-nat/internal/incus"
	"github.com/DioSaputra28/vps-nat/internal/model"
	telegramservice "github.com/DioSaputra28/vps-nat/internal/telegram/service"
	"gorm.io/gorm"
)

var (
	ErrInvalidTelegramUser      = errors.New("invalid telegram user")
	ErrTelegramUserNotFound     = errors.New("telegram user not found")
	ErrMyVPSNotFound            = errors.New("my vps not found")
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
	ErrPackageNotFound          = errors.New("package not found")
	ErrHostnameAlreadyExists    = errors.New("hostname already exists")
	ErrOrderNotFound            = errors.New("order not found")
	ErrProvisioningFailed       = errors.New("provisioning failed")
	ErrPaymentVerification      = errors.New("payment verification failed")
	ErrActiveNodeNotFound       = errors.New("active node not found")
)

const actorTypeTelegramUser = "user"
const actorTypeSystem = "system"

type Service struct {
	repo                *Repository
	incus               *incus.Client
	actions             telegramservice.ActionExecutor
	purchaseProvisioner PurchaseProvisioner
	paymentGateway      PaymentGateway
	adminNotifier       AdminNotifier
}

type managedService struct {
	User     *model.User
	Wallet   *model.Wallet
	Instance *model.ServiceInstance
	Service  *model.Service
	Package  *model.Package
}

func NewService(repo *Repository, incusClient *incus.Client) *Service {
	return &Service{
		repo:                repo,
		incus:               incusClient,
		actions:             telegramservice.NewActionExecutor(incusClient),
		purchaseProvisioner: nil,
		paymentGateway:      nil,
		adminNotifier:       nil,
	}
}

func (s *Service) ConfigurePurchase(provisioner PurchaseProvisioner, gateway PaymentGateway, notifier AdminNotifier) {
	s.purchaseProvisioner = provisioner
	s.paymentGateway = gateway
	s.adminNotifier = notifier
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

func transferUser(user *model.User) TransferUser {
	return TransferUser{
		ID:               user.ID,
		TelegramID:       user.TelegramID,
		TelegramUsername: user.TelegramUsername,
		DisplayName:      user.DisplayName,
	}
}
