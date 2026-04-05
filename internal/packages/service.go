package packages

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/activitylog"
	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidPackageInput = errors.New("invalid package input")
	ErrPackageNotFound     = errors.New("package not found")
)

type Service struct {
	repo *Repository
}

type CreateInput struct {
	Name         string
	Description  *string
	CPU          int
	RAMMB        int
	DiskGB       int
	Price        int64
	DurationDays int
	IsActive     *bool
}

type UpdateInput struct {
	Name         *string
	Description  *string
	CPU          *int
	RAMMB        *int
	DiskGB       *int
	Price        *int64
	DurationDays *int
	IsActive     *bool
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*model.Package, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || input.CPU <= 0 || input.RAMMB <= 0 || input.DiskGB <= 0 || input.Price < 0 || input.DurationDays <= 0 {
		return nil, ErrInvalidPackageInput
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	now := time.Now().UTC()
	pkg := &model.Package{
		ID:           uuid.NewString(),
		Name:         name,
		Description:  normalizeOptionalString(input.Description),
		CPU:          input.CPU,
		RAMMB:        input.RAMMB,
		DiskGB:       input.DiskGB,
		Price:        input.Price,
		DurationDays: input.DurationDays,
		IsActive:     isActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(pkg).Error; err != nil {
			return err
		}
		admin, ok := middleware.CurrentAdminFromContext(ctx)
		if !ok {
			return nil
		}
		return activitylog.Write(ctx, tx, activitylog.Entry{
			ActorType:  "admin",
			ActorID:    &admin.ID,
			Action:     "package.created",
			TargetType: "package",
			TargetID:   &pkg.ID,
			Metadata: map[string]any{
				"name":          pkg.Name,
				"cpu":           pkg.CPU,
				"ram_mb":        pkg.RAMMB,
				"disk_gb":       pkg.DiskGB,
				"price":         pkg.Price,
				"duration_days": pkg.DurationDays,
				"is_active":     pkg.IsActive,
			},
			CreatedAt: now,
		})
	}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, fmt.Errorf("%w: package name already exists", ErrInvalidPackageInput)
		}

		return nil, err
	}

	return pkg, nil
}

func (s *Service) List(ctx context.Context, isActive *bool) ([]model.Package, error) {
	return s.repo.FindAll(ctx, isActive)
}

func (s *Service) GetByID(ctx context.Context, id string) (*model.Package, error) {
	pkg, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}

		return nil, err
	}

	return pkg, nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*model.Package, error) {
	pkg, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{
		"updated_at": time.Now().UTC(),
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrInvalidPackageInput
		}

		updates["name"] = name
	}

	if input.Description != nil {
		updates["description"] = normalizeOptionalString(input.Description)
	}

	if input.CPU != nil {
		if *input.CPU <= 0 {
			return nil, ErrInvalidPackageInput
		}

		updates["cpu"] = *input.CPU
	}

	if input.RAMMB != nil {
		if *input.RAMMB <= 0 {
			return nil, ErrInvalidPackageInput
		}

		updates["ram_mb"] = *input.RAMMB
	}

	if input.DiskGB != nil {
		if *input.DiskGB <= 0 {
			return nil, ErrInvalidPackageInput
		}

		updates["disk_gb"] = *input.DiskGB
	}

	if input.Price != nil {
		if *input.Price < 0 {
			return nil, ErrInvalidPackageInput
		}

		updates["price"] = *input.Price
	}

	if input.DurationDays != nil {
		if *input.DurationDays <= 0 {
			return nil, ErrInvalidPackageInput
		}

		updates["duration_days"] = *input.DurationDays
	}

	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	updatedAt := updates["updated_at"].(time.Time)
	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(pkg).Updates(updates).Error; err != nil {
			return err
		}
		admin, ok := middleware.CurrentAdminFromContext(ctx)
		if !ok {
			return nil
		}
		return activitylog.Write(ctx, tx, activitylog.Entry{
			ActorType:  "admin",
			ActorID:    &admin.ID,
			Action:     "package.updated",
			TargetType: "package",
			TargetID:   &pkg.ID,
			Metadata:   updates,
			CreatedAt:  updatedAt,
		})
	}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, fmt.Errorf("%w: package name already exists", ErrInvalidPackageInput)
		}

		return nil, err
	}

	return s.GetByID(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) (*model.Package, error) {
	pkg, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(pkg).Updates(map[string]any{
			"is_active":  false,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		admin, ok := middleware.CurrentAdminFromContext(ctx)
		if !ok {
			return nil
		}
		return activitylog.Write(ctx, tx, activitylog.Entry{
			ActorType:  "admin",
			ActorID:    &admin.ID,
			Action:     "package.deactivated",
			TargetType: "package",
			TargetID:   &pkg.ID,
			Metadata: map[string]any{
				"name": pkg.Name,
			},
			CreatedAt: now,
		})
	}); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id)
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
