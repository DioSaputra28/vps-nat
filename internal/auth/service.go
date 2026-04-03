package auth

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrSessionExpired     = errors.New("session expired")
)

type Service struct {
	db  *gorm.DB
	cfg config.AuthConfig
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	Admin     model.AdminUser
}

func NewService(db *gorm.DB, cfg config.AuthConfig) *Service {
	return &Service{
		db:  db,
		cfg: cfg,
	}
}

func (s *Service) Login(input LoginInput) (*LoginResult, error) {
	var admin model.AdminUser

	err := s.db.Where("LOWER(email) = LOWER(?)", strings.TrimSpace(input.Email)).First(&admin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf("find admin: %w", err)
	}

	if admin.Status != "active" {
		return nil, ErrInvalidCredentials
	}

	if err := CheckPassword(admin.PasswordHash, input.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, tokenHash, err := GenerateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.cfg.SessionTTL)
	userAgent := strings.TrimSpace(input.UserAgent)
	session := model.AdminSession{
		ID:          uuid.NewString(),
		AdminUserID: admin.ID,
		TokenHash:   tokenHash,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
		LastUsedAt:  &now,
	}
	if userAgent != "" {
		session.UserAgent = &userAgent
	}

	if input.IPAddress != "" {
		parsedIP := net.ParseIP(strings.TrimSpace(input.IPAddress))
		if parsedIP != nil {
			ip := parsedIP.String()
			session.IPAddress = &ip
		}
	}

	if err := s.db.Create(&session).Error; err != nil {
		return nil, fmt.Errorf("create admin session: %w", err)
	}

	return &LoginResult{
		Token:     token,
		ExpiresAt: expiresAt,
		Admin:     admin,
	}, nil
}

func (s *Service) Authenticate(token string) (*model.AdminUser, *model.AdminSession, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil, ErrUnauthorized
	}

	tokenHash := HashSessionToken(token)

	var session model.AdminSession
	err := s.db.Preload("AdminUser").Where("token_hash = ?", tokenHash).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrUnauthorized
		}

		return nil, nil, fmt.Errorf("find session: %w", err)
	}

	now := time.Now().UTC()
	if session.RevokedAt != nil {
		return nil, nil, ErrUnauthorized
	}

	if !session.ExpiresAt.After(now) {
		return nil, nil, ErrSessionExpired
	}

	if session.AdminUser == nil || session.AdminUser.Status != "active" {
		return nil, nil, ErrUnauthorized
	}

	if err := s.db.Model(&session).Updates(map[string]any{
		"last_used_at": now,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, nil, fmt.Errorf("update session last_used_at: %w", err)
	}

	session.LastUsedAt = &now
	session.UpdatedAt = now

	return session.AdminUser, &session, nil
}

func (s *Service) Logout(token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrUnauthorized
	}

	now := time.Now().UTC()
	result := s.db.Model(&model.AdminSession{}).
		Where("token_hash = ? AND revoked_at IS NULL", HashSessionToken(token)).
		Updates(map[string]any{
			"revoked_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("revoke session: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrUnauthorized
	}

	return nil
}
