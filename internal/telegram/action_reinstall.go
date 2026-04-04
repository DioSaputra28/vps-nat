package telegram

import (
	"context"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	telegramservice "github.com/DioSaputra28/vps-nat/internal/telegram/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

	job := telegramservice.SuccessJob(ms.Service.ID, nil, "reinstall", actorTypeTelegramUser, ms.User.ID, reinstallOpID, map[string]any{
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
		Username:    telegramservice.DefaultSSHUsername,
		NewPassword: newPassword,
		ImageAlias:  input.ImageAlias,
	}, nil
}
