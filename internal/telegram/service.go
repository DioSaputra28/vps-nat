package telegram

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidTelegramUser  = errors.New("invalid telegram user")
	ErrTelegramUserNotFound = errors.New("telegram user not found")
)

type Service struct {
	repo *Repository
}

type SyncStartInput struct {
	TelegramID       int64
	TelegramUsername *string
	DisplayName      string
	FirstName        *string
	LastName         *string
}

type SyncStartResult struct {
	User    model.User
	Created bool
}

type HomeInput struct {
	TelegramID int64
}

type HomeResult struct {
	User             HomeUser
	Wallet           HomeWallet
	Packages         []HomePackage
	OperatingSystems []string
	Rules            []string
	Platform         HomePlatform
}

type BuyVPSInput struct {
	TelegramID int64
}

type BuyVPSResult struct {
	User     HomeUser
	Wallet   HomeWallet
	Packages []HomePackage
}

type HomeUser struct {
	ID               string    `json:"id"`
	TelegramID       int64     `json:"telegram_id"`
	TelegramUsername *string   `json:"telegram_username,omitempty"`
	DisplayName      string    `json:"display_name"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type HomeWallet struct {
	ID        string    `json:"id"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type HomePackage struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	CPU          int     `json:"cpu"`
	RAMMB        int     `json:"ram_mb"`
	DiskGB       int     `json:"disk_gb"`
	Price        int64   `json:"price"`
	DurationDays int     `json:"duration_days"`
}

type HomePlatform struct {
	Name           string `json:"name"`
	ServiceType    string `json:"service_type"`
	Virtualization string `json:"virtualization"`
	AccessMethod   string `json:"access_method"`
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SyncStart(ctx context.Context, input SyncStartInput) (*SyncStartResult, error) {
	if input.TelegramID <= 0 {
		return nil, ErrInvalidTelegramUser
	}

	displayName := resolveDisplayName(input.DisplayName, input.FirstName, input.LastName, input.TelegramUsername)
	if displayName == "" {
		return nil, ErrInvalidTelegramUser
	}

	username := normalizeOptionalString(input.TelegramUsername)
	now := time.Now().UTC()

	existing, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		user := model.User{
			ID:               uuid.NewString(),
			TelegramID:       input.TelegramID,
			TelegramUsername: username,
			DisplayName:      displayName,
			Status:           "active",
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		wallet := model.Wallet{
			ID:        uuid.NewString(),
			UserID:    user.ID,
			Balance:   0,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.repo.CreateOrUpdate(ctx, &user, &wallet, true, false); err != nil {
			return nil, err
		}

		user.Wallet = &wallet
		return &SyncStartResult{
			User:    user,
			Created: true,
		}, nil
	}

	existing.TelegramUsername = username
	existing.DisplayName = displayName
	existing.UpdatedAt = now

	walletExists := existing.Wallet != nil
	var wallet model.Wallet
	if walletExists {
		wallet = *existing.Wallet
	} else {
		wallet = model.Wallet{
			ID:        uuid.NewString(),
			UserID:    existing.ID,
			Balance:   0,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	if err := s.repo.CreateOrUpdate(ctx, existing, &wallet, false, walletExists); err != nil {
		return nil, err
	}

	existing.Wallet = &wallet
	return &SyncStartResult{
		User:    *existing,
		Created: false,
	}, nil
}

func (s *Service) Home(ctx context.Context, input HomeInput) (*HomeResult, error) {
	if input.TelegramID <= 0 {
		return nil, ErrInvalidTelegramUser
	}

	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}

		return nil, err
	}

	packages, err := s.repo.FindActivePackages(ctx)
	if err != nil {
		return nil, err
	}

	result := &HomeResult{
		User: HomeUser{
			ID:               user.ID,
			TelegramID:       user.TelegramID,
			TelegramUsername: user.TelegramUsername,
			DisplayName:      user.DisplayName,
			Status:           user.Status,
			CreatedAt:        user.CreatedAt,
			UpdatedAt:        user.UpdatedAt,
		},
		OperatingSystems: defaultOperatingSystems(),
		Rules:            defaultRules(),
		Platform:         defaultPlatform(),
	}

	if user.Wallet != nil {
		result.Wallet = HomeWallet{
			ID:        user.Wallet.ID,
			Balance:   user.Wallet.Balance,
			CreatedAt: user.Wallet.CreatedAt,
			UpdatedAt: user.Wallet.UpdatedAt,
		}
	}

	result.Packages = make([]HomePackage, 0, len(packages))
	for i := range packages {
		result.Packages = append(result.Packages, HomePackage{
			ID:           packages[i].ID,
			Name:         packages[i].Name,
			Description:  packages[i].Description,
			CPU:          packages[i].CPU,
			RAMMB:        packages[i].RAMMB,
			DiskGB:       packages[i].DiskGB,
			Price:        packages[i].Price,
			DurationDays: packages[i].DurationDays,
		})
	}

	return result, nil
}

func (s *Service) BuyVPS(ctx context.Context, input BuyVPSInput) (*BuyVPSResult, error) {
	if input.TelegramID <= 0 {
		return nil, ErrInvalidTelegramUser
	}

	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}

		return nil, err
	}

	packages, err := s.repo.FindActivePackages(ctx)
	if err != nil {
		return nil, err
	}

	result := &BuyVPSResult{
		User: HomeUser{
			ID:               user.ID,
			TelegramID:       user.TelegramID,
			TelegramUsername: user.TelegramUsername,
			DisplayName:      user.DisplayName,
			Status:           user.Status,
			CreatedAt:        user.CreatedAt,
			UpdatedAt:        user.UpdatedAt,
		},
	}

	if user.Wallet != nil {
		result.Wallet = HomeWallet{
			ID:        user.Wallet.ID,
			Balance:   user.Wallet.Balance,
			CreatedAt: user.Wallet.CreatedAt,
			UpdatedAt: user.Wallet.UpdatedAt,
		}
	}

	result.Packages = make([]HomePackage, 0, len(packages))
	for i := range packages {
		result.Packages = append(result.Packages, HomePackage{
			ID:           packages[i].ID,
			Name:         packages[i].Name,
			Description:  packages[i].Description,
			CPU:          packages[i].CPU,
			RAMMB:        packages[i].RAMMB,
			DiskGB:       packages[i].DiskGB,
			Price:        packages[i].Price,
			DurationDays: packages[i].DurationDays,
		})
	}

	return result, nil
}

func resolveDisplayName(displayName string, firstName *string, lastName *string, username *string) string {
	if trimmed := strings.TrimSpace(displayName); trimmed != "" {
		return trimmed
	}

	parts := []string{}
	if firstName != nil {
		if trimmed := strings.TrimSpace(*firstName); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if lastName != nil {
		if trimmed := strings.TrimSpace(*lastName); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}

	if username != nil {
		if trimmed := strings.TrimSpace(*username); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func defaultOperatingSystems() []string {
	return []string{
		"Debian 10",
		"Debian 11",
		"Debian 12",
		"Debian 13",
		"Ubuntu 20.04",
		"Ubuntu 22.04",
		"Ubuntu 24.04",
		"Kali Linux",
	}
}

func defaultRules() []string {
	return []string{
		"Dilarang melakukan aktivitas ilegal atau penyalahgunaan layanan.",
		"Dilarang DDoS, botnet, phishing, carding, atau spam abuse.",
		"Dilarang penggunaan resource secara berlebihan dan terus-menerus tanpa batas wajar.",
		"Dilarang VPN, proxy, dan tunneling berlebihan jika mengganggu stabilitas node.",
		"Pelanggaran dapat menyebabkan suspend permanen tanpa refund.",
	}
}

func defaultPlatform() HomePlatform {
	return HomePlatform{
		Name:           "VPS NAT",
		ServiceType:    "VPS NAT",
		Virtualization: "Incus Container",
		AccessMethod:   "SSH",
	}
}
