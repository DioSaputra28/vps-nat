package telegram

import (
	"context"
	"errors"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	telegramservice "github.com/DioSaputra28/vps-nat/internal/telegram/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) DomainPreview(ctx context.Context, input DomainPreviewInput) (*DomainPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	domain, ok := telegramservice.NormalizeDomain(input.Domain)
	if !ok {
		return nil, ErrInvalidDomain
	}
	if !telegramservice.IsValidProxyMode(input.ProxyMode) || input.TargetPort < 1 || input.TargetPort > 65535 {
		return nil, ErrInvalidActionRequest
	}

	exists, err := s.repo.DomainExists(ctx, domain)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDomainAlreadyExists
	}

	return &DomainPreviewResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Domain:      domain,
		TargetPort:  input.TargetPort,
		ProxyMode:   input.ProxyMode,
		Available:   true,
	}, nil
}

func (s *Service) DomainSubmit(ctx context.Context, input DomainSubmitInput) (*DomainSubmitResult, error) {
	preview, err := s.DomainPreview(ctx, DomainPreviewInput(input))
	if err != nil {
		return nil, err
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	job := telegramservice.QueuedJob(ms.Service.ID, nil, "reassign_port", actorTypeTelegramUser, ms.User.ID, map[string]any{
		"kind":         "setup_domain",
		"container_id": ms.Instance.ID,
		"domain":       preview.Domain,
		"target_port":  preview.TargetPort,
		"proxy_mode":   preview.ProxyMode,
	})
	record := model.ServiceDomain{
		ID:         uuid.NewString(),
		ServiceID:  ms.Service.ID,
		Domain:     preview.Domain,
		TargetPort: preview.TargetPort,
		ProxyMode:  preview.ProxyMode,
		Status:     "pending",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	event := model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "domain_setup_requested",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "permintaan setup domain dibuat",
		Payload: map[string]any{
			"domain":      record.Domain,
			"target_port": record.TargetPort,
			"proxy_mode":  record.ProxyMode,
			"job_id":      job.ID,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(&event).Error
	}); err != nil {
		return nil, err
	}

	return &DomainSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Domain:      record,
		JobID:       job.ID,
		Status:      "queued",
	}, nil
}

func (s *Service) ReconfigIPPreview(ctx context.Context, input ReconfigIPPreviewInput) (*ReconfigIPPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	port, err := s.repo.FindNextAvailableSSHPort(ctx)
	if err != nil {
		return nil, err
	}

	return &ReconfigIPPreviewResult{
		ContainerID:     ms.Instance.ID,
		ServiceID:       ms.Service.ID,
		MainPublicIP:    ms.Instance.MainPublicIP,
		CurrentSSHPort:  ms.Instance.SSHPort,
		ProposedSSHPort: port,
	}, nil
}

func (s *Service) ReconfigIPSubmit(ctx context.Context, input ReconfigIPSubmitInput) (*ReconfigIPSubmitResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if ms.Instance.MainPublicIP == nil || ms.Instance.InternalIP == nil {
		return nil, ErrInvalidActionRequest
	}

	var newPort int
	if input.RequestedSSHPort != nil {
		newPort = *input.RequestedSSHPort
	} else {
		newPort, err = s.repo.FindNextAvailableSSHPort(ctx)
		if err != nil {
			return nil, err
		}
	}
	if newPort < 1 || newPort > 65535 {
		return nil, ErrInvalidActionRequest
	}

	now := time.Now().UTC()
	job := telegramservice.SuccessJob(ms.Service.ID, nil, "reassign_port", actorTypeTelegramUser, ms.User.ID, "", map[string]any{
		"container_id":   ms.Instance.ID,
		"old_ssh_port":   ms.Instance.SSHPort,
		"new_ssh_port":   newPort,
		"main_public_ip": ms.Instance.MainPublicIP,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "ip_reconfigured",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "akses NAT utama berhasil diubah",
		Payload: map[string]any{
			"container_id": ms.Instance.ID,
			"new_ssh_port": newPort,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", ms.Instance.ID).
			Updates(map[string]any{
				"ssh_port":   newPort,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		var mapping model.ServicePortMapping
		err := tx.Where("service_id = ? AND mapping_type = ? AND is_active = ?", ms.Service.ID, "ssh", true).First(&mapping).Error
		if err == nil {
			if err := tx.Model(&model.ServicePortMapping{}).
				Where("id = ?", mapping.ID).
				Updates(map[string]any{
					"public_port": newPort,
					"updated_at":  now,
				}).Error; err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	return &ReconfigIPSubmitResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "reconfig_ip",
			Accepted:    true,
			JobID:       &job.ID,
			Status:      "completed",
		},
		MainPublicIP: ms.Instance.MainPublicIP,
		SSHPort:      &newPort,
	}, nil
}
