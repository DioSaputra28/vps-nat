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

func (r *Repository) FindUserServiceInstances(ctx context.Context, telegramID int64) ([]model.ServiceInstance, error) {
	var instances []model.ServiceInstance
	err := r.db.WithContext(ctx).
		Model(&model.ServiceInstance{}).
		Joins("JOIN services ON services.id = service_instances.service_id").
		Joins("JOIN users ON users.id = services.owner_user_id").
		Where("users.telegram_id = ?", telegramID).
		Preload("Node").
		Preload("Service").
		Order("service_instances.created_at DESC").
		Find(&instances).Error
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (r *Repository) FindUserServiceInstanceByID(ctx context.Context, telegramID int64, containerID string) (*model.ServiceInstance, error) {
	var instance model.ServiceInstance
	err := r.db.WithContext(ctx).
		Model(&model.ServiceInstance{}).
		Joins("JOIN services ON services.id = service_instances.service_id").
		Joins("JOIN users ON users.id = services.owner_user_id").
		Where("users.telegram_id = ? AND service_instances.id = ?", telegramID, containerID).
		Preload("Node").
		Preload("Service").
		Preload("Service.CurrentPack").
		Preload("Service.PortMappings", "is_active = ?", true).
		Preload("Service.Domains").
		First(&instance).Error
	if err != nil {
		return nil, err
	}

	return &instance, nil
}

func (r *Repository) FindPackageByID(ctx context.Context, id string) (*model.Package, error) {
	var pkg model.Package
	if err := r.db.WithContext(ctx).
		First(&pkg, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &pkg, nil
}

func (r *Repository) HasConflictingJob(ctx context.Context, serviceID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ProvisioningJob{}).
		Where("service_id = ? AND status IN ?", serviceID, []string{"queued", "running"}).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *Repository) DomainExists(ctx context.Context, domain string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ServiceDomain{}).
		Where("LOWER(domain) = LOWER(?)", domain).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *Repository) FindNextAvailableSSHPort(ctx context.Context) (int, error) {
	const (
		minPort = 20000
		maxPort = 39999
	)

	var used []int
	if err := r.db.WithContext(ctx).
		Model(&model.ServicePortMapping{}).
		Where("mapping_type = ? AND is_active = ?", "ssh", true).
		Pluck("public_port", &used).Error; err != nil {
		return 0, err
	}

	usedSet := make(map[int]struct{}, len(used))
	for _, port := range used {
		usedSet[port] = struct{}{}
	}

	for port := minPort; port <= maxPort; port++ {
		if _, exists := usedSet[port]; !exists {
			return port, nil
		}
	}

	return 0, gorm.ErrRecordNotFound
}

func (r *Repository) persistActionArtifacts(ctx context.Context, instance *model.ServiceInstance, service *model.Service, job *model.ProvisioningJob, event *model.ServiceEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if instance != nil {
			updates := map[string]any{
				"updated_at": timeNowUTC(),
			}
			if instance.LastIncusOperationID != nil {
				updates["last_incus_operation_id"] = instance.LastIncusOperationID
			}
			if err := tx.Model(&model.ServiceInstance{}).
				Where("id = ?", instance.ID).
				Updates(updates).Error; err != nil {
				return err
			}
		}

		if service != nil {
			if err := tx.Model(&model.Service{}).
				Where("id = ?", service.ID).
				Updates(map[string]any{
					"status":     service.Status,
					"updated_at": timeNowUTC(),
				}).Error; err != nil {
				return err
			}
		}

		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	})
}

func timeNowUTC() any {
	return gorm.Expr("timezone('UTC', now())")
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
