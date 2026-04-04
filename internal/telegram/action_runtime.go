package telegram

import (
	"context"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	telegramservice "github.com/DioSaputra28/vps-nat/internal/telegram/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) RuntimeAction(ctx context.Context, input RuntimeActionInput) (*ActionAcceptedResult, error) {
	if input.TelegramID <= 0 {
		return nil, ErrInvalidActionRequest
	}

	action := telegramservice.NormalizeRuntimeAction(input.Action)
	if action == "" {
		return nil, ErrInvalidActionRequest
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}
	if err := ensureServiceOperable(ms.Service); err != nil {
		return nil, err
	}
	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}
	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	opID, err := s.actions.ChangeState(ms.Instance.IncusInstanceName, action)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	instanceStatus, serviceStatus := telegramservice.RuntimeStatuses(action)
	job := telegramservice.SuccessJob(ms.Service.ID, nil, telegramservice.ActionToJobType(action), actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
		"action":              action,
		"container_id":        ms.Instance.ID,
		"incus_instance_name": ms.Instance.IncusInstanceName,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: telegramservice.RuntimeEventType(action),
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   telegramservice.RuntimeSummary(action, ms.Instance.IncusInstanceName),
		Payload: map[string]any{
			"action":       action,
			"container_id": ms.Instance.ID,
			"operation_id": opID,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", ms.Instance.ID).
			Updates(map[string]any{
				"status":                  instanceStatus,
				"last_incus_operation_id": opID,
				"updated_at":              now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.Service{}).
			Where("id = ?", ms.Service.ID).
			Updates(map[string]any{
				"status":       serviceStatus,
				"suspended_at": nil,
				"updated_at":   now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	return &ActionAcceptedResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Action:      action,
		Accepted:    true,
		OperationID: &opID,
		JobID:       &job.ID,
		Status:      "completed",
	}, nil
}

func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) (*ChangePasswordResult, error) {
	if input.TelegramID <= 0 || len(strings.TrimSpace(input.NewPassword)) < 8 {
		return nil, ErrInvalidActionRequest
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}
	if !telegramservice.IsRunningStatus(ms.Instance.Status) {
		return nil, ErrContainerNotRunning
	}
	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	opID, err := s.actions.ChangePassword(ms.Instance.IncusInstanceName, strings.TrimSpace(input.NewPassword))
	if err != nil {
		return nil, err
	}
	ms.Instance.LastIncusOperationID = &opID

	job := telegramservice.SuccessJob(ms.Service.ID, nil, "change_password", actorTypeTelegramUser, ms.User.ID, opID, map[string]any{"container_id": ms.Instance.ID})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "password_changed",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "password layanan berhasil diubah",
		Payload: map[string]any{
			"container_id": ms.Instance.ID,
			"operation_id": opID,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.persistActionArtifacts(ctx, ms.Instance, nil, job, event); err != nil {
		return nil, err
	}

	return &ChangePasswordResult{ActionAcceptedResult: ActionAcceptedResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Action:      "change_password",
		Accepted:    true,
		OperationID: &opID,
		JobID:       &job.ID,
		Status:      "completed",
	}}, nil
}

func (s *Service) ResetSSH(ctx context.Context, input ResetSSHInput) (*ResetSSHResult, error) {
	if input.TelegramID <= 0 {
		return nil, ErrInvalidActionRequest
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}
	if !telegramservice.IsRunningStatus(ms.Instance.Status) {
		return nil, ErrContainerNotRunning
	}
	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	opID, password, err := s.actions.ResetSSH(ms.Instance.IncusInstanceName)
	if err != nil {
		return nil, err
	}
	ms.Instance.LastIncusOperationID = &opID

	job := telegramservice.SuccessJob(ms.Service.ID, nil, "change_password", actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
		"container_id": ms.Instance.ID,
		"kind":         "ssh_reset",
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "ssh_reset",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "akses SSH berhasil direset",
		Payload: map[string]any{
			"container_id": ms.Instance.ID,
			"operation_id": opID,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.persistActionArtifacts(ctx, ms.Instance, nil, job, event); err != nil {
		return nil, err
	}

	return &ResetSSHResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "ssh_reset",
			Accepted:    true,
			OperationID: &opID,
			JobID:       &job.ID,
			Status:      "completed",
		},
		Username:    telegramservice.DefaultSSHUsername,
		NewPassword: password,
	}, nil
}
