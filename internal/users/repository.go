package users

import (
	"context"
	"strings"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type ListParams struct {
	Page   int
	Limit  int
	Search string
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(ctx context.Context, params ListParams) ([]model.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.User{})

	search := strings.TrimSpace(params.Search)
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"display_name ILIKE ? OR COALESCE(telegram_username, '') ILIKE ? OR CAST(telegram_id AS TEXT) ILIKE ?",
			like,
			like,
			like,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	err := query.
		Preload("Wallet").
		Order("created_at DESC").
		Limit(params.Limit).
		Offset((params.Page - 1) * params.Limit).
		Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).
		Preload("Wallet").
		First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
