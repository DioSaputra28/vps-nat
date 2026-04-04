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

func (s *Service) TransferPreview(ctx context.Context, input TransferPreviewInput) (*TransferPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	target, err := s.repo.FindUserByTelegramID(ctx, input.TargetTelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransferTargetNotFound
		}
		return nil, err
	}
	if target.ID == ms.User.ID {
		return nil, ErrTransferTargetSame
	}

	return &TransferPreviewResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		FromUser:    transferUser(ms.User),
		ToUser:      transferUser(target),
	}, nil
}

func (s *Service) TransferSubmit(ctx context.Context, input TransferSubmitInput) (*TransferSubmitResult, error) {
	preview, err := s.TransferPreview(ctx, TransferPreviewInput{
		TelegramID:       input.TelegramID,
		ContainerID:      input.ContainerID,
		TargetTelegramID: input.TargetTelegramID,
	})
	if err != nil {
		return nil, err
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}
	target, err := s.repo.FindUserByTelegramID(ctx, input.TargetTelegramID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	transfer := &model.ServiceTransfer{
		ID:            uuid.NewString(),
		ServiceID:     ms.Service.ID,
		FromUserID:    ms.User.ID,
		ToUserID:      target.ID,
		Reason:        normalizeOptionalString(input.Reason),
		CreatedByType: actorTypeTelegramUser,
		CreatedByID:   &ms.User.ID,
		CreatedAt:     now,
	}
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "service_transferred",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "kepemilikan layanan berhasil dipindahkan",
		Payload: map[string]any{
			"from_user_id": ms.User.ID,
			"to_user_id":   target.ID,
			"reason":       transfer.Reason,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Service{}).
			Where("id = ?", ms.Service.ID).
			Updates(map[string]any{
				"owner_user_id": target.ID,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Create(transfer).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	return &TransferSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		FromUser:    preview.FromUser,
		ToUser:      preview.ToUser,
	}, nil
}

func (s *Service) CancelPreview(ctx context.Context, input CancelPreviewInput) (*CancelPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	remaining := telegramservice.RemainingDurationSeconds(ms.Service.ExpiresAt)
	return &CancelPreviewResult{
		ContainerID:      ms.Instance.ID,
		ServiceID:        ms.Service.ID,
		PackageName:      ms.Service.PackageNameSnapshot,
		CurrentExpiresAt: ms.Service.ExpiresAt,
		RemainingSeconds: remaining,
		RefundAmount:     telegramservice.ProratedRefund(ms.Service.PriceSnapshot, remaining, ms.Service.BillingCycleDays),
	}, nil
}

func (s *Service) CancelSubmit(ctx context.Context, input CancelSubmitInput) (*CancelSubmitResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}

	if s.actions == nil {
		return nil, ErrIncusUnavailable
	}

	refund := telegramservice.ProratedRefund(ms.Service.PriceSnapshot, telegramservice.RemainingDurationSeconds(ms.Service.ExpiresAt), ms.Service.BillingCycleDays)
	opID, err := s.actions.DeleteInstance(ms.Instance.IncusInstanceName)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	sourceServiceID := ms.Service.ID
	note := "service cancellation refund"
	job := telegramservice.SuccessJob(ms.Service.ID, nil, "cancel", actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
		"container_id": ms.Instance.ID,
		"refund":       refund,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "service_canceled",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "layanan berhasil dibatalkan",
		Payload: map[string]any{
			"container_id": ms.Instance.ID,
			"refund":       refund,
			"operation_id": opID,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if refund > 0 && ms.Wallet != nil {
			if err := tx.Model(&model.Wallet{}).
				Where("id = ?", ms.Wallet.ID).
				Update("balance", ms.Wallet.Balance+refund).Error; err != nil {
				return err
			}
			txRecord := &model.WalletTransaction{
				ID:              uuid.NewString(),
				WalletID:        ms.Wallet.ID,
				UserID:          ms.User.ID,
				Direction:       "credit",
				TransactionType: "refund",
				Amount:          refund,
				BalanceBefore:   ms.Wallet.Balance,
				BalanceAfter:    ms.Wallet.Balance + refund,
				SourceType:      "service",
				SourceID:        &sourceServiceID,
				Note:            &note,
				CreatedAt:       now,
			}
			if err := tx.Create(txRecord).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&model.Service{}).
			Where("id = ?", ms.Service.ID).
			Updates(map[string]any{
				"status":        "terminated",
				"canceled_at":   now,
				"terminated_at": now,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", ms.Instance.ID).
			Updates(map[string]any{
				"status":                  "deleted",
				"last_incus_operation_id": opID,
				"updated_at":              now,
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

	return &CancelSubmitResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "cancel",
			Accepted:    true,
			OperationID: &opID,
			JobID:       &job.ID,
			Status:      "completed",
		},
		RefundAmount: refund,
	}, nil
}
