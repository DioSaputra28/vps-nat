package telegram

import (
	"context"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindUserByTelegramID(ctx context.Context, telegramID int64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).
		Preload("Wallet").
		First(&user, "telegram_id = ?", telegramID).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *Repository) FindActivePackages(ctx context.Context) ([]model.Package, error) {
	var packages []model.Package
	err := r.db.WithContext(ctx).
		Model(&model.Package{}).
		Where("is_active = ?", true).
		Order("duration_days ASC").
		Order("price ASC").
		Order("name ASC").
		Find(&packages).Error
	if err != nil {
		return nil, err
	}

	return packages, nil
}

func (r *Repository) CreateOrUpdate(ctx context.Context, user *model.User, wallet *model.Wallet, isNew bool, walletExists bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if isNew {
			if err := tx.Create(user).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&model.User{}).
				Where("id = ?", user.ID).
				Updates(map[string]any{
					"telegram_username": user.TelegramUsername,
					"display_name":      user.DisplayName,
					"updated_at":        user.UpdatedAt,
				}).Error; err != nil {
				return err
			}
		}

		if walletExists {
			return nil
		}

		if err := tx.Create(wallet).Error; err != nil {
			return err
		}

		return nil
	})
}
