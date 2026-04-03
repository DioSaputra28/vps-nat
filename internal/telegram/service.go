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
	ErrInvalidTelegramUser = errors.New("invalid telegram user")
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
