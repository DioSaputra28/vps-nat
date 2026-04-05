package containers

import (
	"context"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/activitylog"
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

func (r *Repository) FindAll(ctx context.Context, params ListParams) ([]model.ServiceInstance, int64, error) {
	query := r.baseQuery(ctx)

	search := strings.TrimSpace(params.Search)
	if search != "" {
		like := "%" + search + "%"
		query = query.Joins("JOIN services ON services.id = service_instances.service_id").
			Joins("JOIN users ON users.id = services.owner_user_id").
			Where(
				"service_instances.incus_instance_name ILIKE ? OR users.display_name ILIKE ? OR COALESCE(users.telegram_username, '') ILIKE ? OR CAST(users.telegram_id AS TEXT) ILIKE ?",
				like,
				like,
				like,
				like,
			)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var instances []model.ServiceInstance
	err := query.
		Order("service_instances.created_at DESC").
		Limit(params.Limit).
		Offset((params.Page - 1) * params.Limit).
		Find(&instances).Error
	if err != nil {
		return nil, 0, err
	}

	return instances, total, nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*model.ServiceInstance, error) {
	var instance model.ServiceInstance
	if err := r.baseQuery(ctx).
		Preload("Service.PortMappings", "is_active = ?", true).
		Preload("Service.Domains").
		First(&instance, "service_instances.id = ?", id).Error; err != nil {
		return nil, err
	}

	return &instance, nil
}

func (r *Repository) SaveActionResult(ctx context.Context, instance *model.ServiceInstance, service *model.Service, job *model.ProvisioningJob, event *model.ServiceEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", instance.ID).
			Updates(map[string]any{
				"status":                  instance.Status,
				"last_incus_operation_id": instance.LastIncusOperationID,
				"updated_at":              instance.UpdatedAt,
			}).Error; err != nil {
			return err
		}

		serviceUpdates := map[string]any{
			"status":     service.Status,
			"updated_at": service.UpdatedAt,
		}
		serviceUpdates["suspended_at"] = service.SuspendedAt

		if err := tx.Model(&model.Service{}).
			Where("id = ?", service.ID).
			Updates(serviceUpdates).Error; err != nil {
			return err
		}

		if err := tx.Create(job).Error; err != nil {
			return err
		}

		if err := tx.Create(event).Error; err != nil {
			return err
		}

		action, _ := job.Payload["action"].(string)
		if strings.TrimSpace(action) == "" {
			action = strings.TrimSpace(job.JobType)
		}
		if action == "" {
			action = "unknown"
		}
		return activitylog.Write(ctx, tx, activitylog.Entry{
			ActorType:  "admin",
			ActorID:    job.RequestedByID,
			Action:     "container." + action,
			TargetType: "service_instance",
			TargetID:   &instance.ID,
			Metadata: map[string]any{
				"service_id":         instance.ServiceID,
				"operation_id":       instance.LastIncusOperationID,
				"incus_instance_name": instance.IncusInstanceName,
				"status":             instance.Status,
			},
			CreatedAt: instance.UpdatedAt,
		})
	})
}

func (r *Repository) SaveFailedAction(ctx context.Context, instance *model.ServiceInstance, job *model.ProvisioningJob, event *model.ServiceEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

		if err := tx.Create(job).Error; err != nil {
			return err
		}

		if err := tx.Create(event).Error; err != nil {
			return err
		}
		action, _ := job.Payload["action"].(string)
		if strings.TrimSpace(action) == "" {
			action = strings.TrimSpace(job.JobType)
		}
		if action == "" {
			action = "unknown"
		}
		return activitylog.Write(ctx, tx, activitylog.Entry{
			ActorType:  "admin",
			ActorID:    job.RequestedByID,
			Action:     "container." + action + ".failed",
			TargetType: "service_instance",
			TargetID:   &instance.ID,
			Metadata: map[string]any{
				"service_id":          instance.ServiceID,
				"operation_id":        instance.LastIncusOperationID,
				"incus_instance_name": instance.IncusInstanceName,
				"error":               job.ErrorMessage,
			},
			CreatedAt: time.Now().UTC(),
		})
	})
}

func (r *Repository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&model.ServiceInstance{}).
		Preload("Node").
		Preload("Service").
		Preload("Service.OwnerUser").
		Preload("Service.CurrentPack")
}

func timeNowUTC() any {
	return gorm.Expr("timezone('UTC', now())")
}
