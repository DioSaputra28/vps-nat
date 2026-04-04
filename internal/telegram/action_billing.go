package telegram

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	telegramservice "github.com/DioSaputra28/vps-nat/internal/telegram/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) RenewPreview(ctx context.Context, input RenewPreviewInput) (*RenewPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := ensureServiceOperable(ms.Service); err != nil {
		return nil, err
	}

	nextExpiresAt := telegramservice.ExtendExpiry(ms.Service.ExpiresAt, ms.Package.DurationDays)

	return &RenewPreviewResult{
		ContainerID:      ms.Instance.ID,
		ServiceID:        ms.Service.ID,
		Package:          packageQuote(ms.Package),
		Price:            ms.Package.Price,
		CurrentExpiresAt: ms.Service.ExpiresAt,
		NextExpiresAt:    nextExpiresAt,
		PaymentMethods:   []string{"wallet", "qris"},
	}, nil
}

func (s *Service) RenewSubmit(ctx context.Context, input RenewSubmitInput) (*BillingSubmitResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := ensureServiceOperable(ms.Service); err != nil {
		return nil, err
	}

	if !telegramservice.ValidatePaymentMethod(input.PaymentMethod) {
		return nil, ErrUnsupportedPayment
	}

	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}

	nextExpiresAt := telegramservice.ExtendExpiry(ms.Service.ExpiresAt, ms.Package.DurationDays)
	if nextExpiresAt == nil {
		return nil, ErrInvalidActionRequest
	}

	now := time.Now().UTC()
	order := telegramservice.BuildOrder(ms.User.ID, ms.Package, "renewal", &ms.Service.ID, ms.Package.Price, nil, now)
	order.PaymentMethod = &input.PaymentMethod
	payment := telegramservice.BuildPayment(&order.ID, "order", input.PaymentMethod, ms.Package.Price, now)

	result := &BillingSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		OrderID:     order.ID,
		PaymentID:   payment.ID,
		Action:      "renew",
		Amount:      ms.Package.Price,
		Applied:     false,
		Status:      "awaiting_payment",
		ExpiresAt:   ms.Service.ExpiresAt,
	}

	switch input.PaymentMethod {
	case "wallet":
		if ms.Wallet == nil || ms.Wallet.Balance < ms.Package.Price {
			return nil, ErrInsufficientBalance
		}

		balanceBefore := ms.Wallet.Balance
		balanceAfter := balanceBefore - ms.Package.Price
		order.Status = "completed"
		order.PaymentStatus = "paid"
		order.PaidAt = &now
		order.UpdatedAt = now
		payment.Status = "paid"
		payment.PaidAt = &now
		payment.UpdatedAt = now

		sourceOrderID := order.ID
		note := "renewal payment"
		walletTxn := &model.WalletTransaction{
			ID:              uuid.NewString(),
			WalletID:        ms.Wallet.ID,
			UserID:          ms.User.ID,
			Direction:       "debit",
			TransactionType: "payment",
			Amount:          ms.Package.Price,
			BalanceBefore:   balanceBefore,
			BalanceAfter:    balanceAfter,
			SourceType:      "order",
			SourceID:        &sourceOrderID,
			Note:            &note,
			CreatedAt:       now,
		}
		event := &model.ServiceEvent{
			ID:        uuid.NewString(),
			ServiceID: ms.Service.ID,
			EventType: "service_renewed",
			ActorType: actorTypeTelegramUser,
			ActorID:   &ms.User.ID,
			Summary:   "layanan berhasil diperpanjang",
			Payload: map[string]any{
				"order_id":   order.ID,
				"payment_id": payment.ID,
				"old_expiry": ms.Service.ExpiresAt,
				"new_expiry": nextExpiresAt,
				"amount":     ms.Package.Price,
			},
			CreatedAt: now,
		}

		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Wallet{}).
				Where("id = ?", ms.Wallet.ID).
				Update("balance", balanceAfter).Error; err != nil {
				return err
			}
			if err := tx.Create(walletTxn).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Service{}).
				Where("id = ?", ms.Service.ID).
				Updates(map[string]any{
					"expires_at": nextExpiresAt,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
			return tx.Create(event).Error
		}); err != nil {
			return nil, err
		}

		result.Applied = true
		result.Status = "completed"
		result.ExpiresAt = nextExpiresAt
	case "qris":
		order.Status = "awaiting_payment"
		order.UpdatedAt = now
		payment.Status = "pending"
		payment.UpdatedAt = now

		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			return tx.Create(&payment).Error
		}); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (s *Service) UpgradeOptions(ctx context.Context, input UpgradeOptionsInput) (*UpgradeOptionsResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	packages, err := s.repo.FindActivePackages(ctx)
	if err != nil {
		return nil, err
	}

	result := &UpgradeOptionsResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Current:     packageQuote(ms.Package),
		Packages:    make([]PackageQuote, 0),
	}

	for i := range packages {
		if telegramservice.IsEligibleUpgrade(ms.Service.CPUSnapshot, ms.Service.RAMMBSnapshot, ms.Service.DiskGBSnapshot, ms.Service.PriceSnapshot, &packages[i]) {
			result.Packages = append(result.Packages, packageQuote(&packages[i]))
		}
	}

	return result, nil
}

func (s *Service) UpgradePreview(ctx context.Context, input UpgradePreviewInput) (*UpgradePreviewResult, error) {
	if strings.TrimSpace(input.TargetPackageID) == "" {
		return nil, ErrInvalidActionRequest
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	targetPackage, err := s.repo.FindPackageByID(ctx, input.TargetPackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpgradePackageNotFound
		}
		return nil, err
	}

	if !targetPackage.IsActive || !telegramservice.IsEligibleUpgrade(ms.Service.CPUSnapshot, ms.Service.RAMMBSnapshot, ms.Service.DiskGBSnapshot, ms.Service.PriceSnapshot, targetPackage) {
		return nil, ErrUpgradePackageIneligible
	}

	remainingSeconds := telegramservice.RemainingDurationSeconds(ms.Service.ExpiresAt)
	amount := telegramservice.ProratedUpgradeCost(ms.Service.PriceSnapshot, targetPackage.Price, remainingSeconds, ms.Service.BillingCycleDays)
	if amount <= 0 {
		return nil, ErrUpgradePackageIneligible
	}

	return &UpgradePreviewResult{
		ContainerID:      ms.Instance.ID,
		ServiceID:        ms.Service.ID,
		Current:          packageQuote(ms.Package),
		Target:           packageQuote(targetPackage),
		CurrentExpiresAt: ms.Service.ExpiresAt,
		RemainingSeconds: remainingSeconds,
		PriceDifference:  amount,
		PaymentMethods:   []string{"wallet", "qris"},
	}, nil
}

func (s *Service) UpgradeSubmit(ctx context.Context, input UpgradeSubmitInput) (*BillingSubmitResult, error) {
	log.Printf("[telegram][upgrade] telegram_id=%d container_id=%s target_package_id=%s payment_method=%s", input.TelegramID, input.ContainerID, input.TargetPackageID, input.PaymentMethod)
	if strings.TrimSpace(input.TargetPackageID) == "" {
		return nil, ErrInvalidActionRequest
	}

	if !telegramservice.ValidatePaymentMethod(input.PaymentMethod) {
		return nil, ErrUnsupportedPayment
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureNoConflictingJobs(ctx, ms.Service.ID); err != nil {
		return nil, err
	}

	targetPackage, err := s.repo.FindPackageByID(ctx, input.TargetPackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpgradePackageNotFound
		}
		return nil, err
	}

	if !targetPackage.IsActive || !telegramservice.IsEligibleUpgrade(ms.Service.CPUSnapshot, ms.Service.RAMMBSnapshot, ms.Service.DiskGBSnapshot, ms.Service.PriceSnapshot, targetPackage) {
		return nil, ErrUpgradePackageIneligible
	}

	remainingSeconds := telegramservice.RemainingDurationSeconds(ms.Service.ExpiresAt)
	amount := telegramservice.ProratedUpgradeCost(ms.Service.PriceSnapshot, targetPackage.Price, remainingSeconds, ms.Service.BillingCycleDays)
	if amount <= 0 {
		return nil, ErrUpgradePackageIneligible
	}

	now := time.Now().UTC()
	order := telegramservice.BuildOrder(ms.User.ID, targetPackage, "upgrade", &ms.Service.ID, amount, nil, now)
	order.PaymentMethod = &input.PaymentMethod
	payment := telegramservice.BuildPayment(&order.ID, "order", input.PaymentMethod, amount, now)

	result := &BillingSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		OrderID:     order.ID,
		PaymentID:   payment.ID,
		Action:      "upgrade",
		Amount:      amount,
		Applied:     false,
		Status:      "awaiting_payment",
		ExpiresAt:   ms.Service.ExpiresAt,
	}

	switch input.PaymentMethod {
	case "wallet":
		if ms.Wallet == nil || ms.Wallet.Balance < amount {
			return nil, ErrInsufficientBalance
		}
		if s.actions == nil {
			return nil, ErrIncusUnavailable
		}

		opID, err := s.actions.ApplyResourceLimits(ms.Instance.IncusInstanceName, targetPackage.CPU, targetPackage.RAMMB, targetPackage.DiskGB)
		if err != nil {
			log.Printf("[telegram][upgrade] instance=%s apply limits failed: %v", ms.Instance.IncusInstanceName, err)
			return nil, err
		}
		log.Printf("[telegram][upgrade] instance=%s apply limits operation_id=%s target_package=%s amount=%d", ms.Instance.IncusInstanceName, opID, targetPackage.ID, amount)

		order.Status = "completed"
		order.PaymentStatus = "paid"
		order.PaidAt = &now
		payment.Status = "paid"
		payment.PaidAt = &now
		sourceOrderID := order.ID
		note := "upgrade payment"
		walletTxn := &model.WalletTransaction{
			ID:              uuid.NewString(),
			WalletID:        ms.Wallet.ID,
			UserID:          ms.User.ID,
			Direction:       "debit",
			TransactionType: "payment",
			Amount:          amount,
			BalanceBefore:   ms.Wallet.Balance,
			BalanceAfter:    ms.Wallet.Balance - amount,
			SourceType:      "order",
			SourceID:        &sourceOrderID,
			Note:            &note,
			CreatedAt:       now,
		}
		job := telegramservice.SuccessJob(ms.Service.ID, &order.ID, "upgrade", actorTypeTelegramUser, ms.User.ID, opID, map[string]any{
			"container_id":      ms.Instance.ID,
			"target_package_id": targetPackage.ID,
		})
		event := &model.ServiceEvent{
			ID:        uuid.NewString(),
			ServiceID: ms.Service.ID,
			EventType: "service_upgraded",
			ActorType: actorTypeTelegramUser,
			ActorID:   &ms.User.ID,
			Summary:   "resource layanan berhasil di-upgrade",
			Payload: map[string]any{
				"order_id":            order.ID,
				"payment_id":          payment.ID,
				"target_package_id":   targetPackage.ID,
				"target_package_name": targetPackage.Name,
				"operation_id":        opID,
			},
			CreatedAt: now,
		}

		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Wallet{}).Where("id = ?", ms.Wallet.ID).Update("balance", ms.Wallet.Balance-amount).Error; err != nil {
				return err
			}
			if err := tx.Create(walletTxn).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Service{}).
				Where("id = ?", ms.Service.ID).
				Updates(map[string]any{
					"current_package_id":    targetPackage.ID,
					"package_name_snapshot": targetPackage.Name,
					"cpu_snapshot":          targetPackage.CPU,
					"ram_mb_snapshot":       targetPackage.RAMMB,
					"disk_gb_snapshot":      targetPackage.DiskGB,
					"price_snapshot":        targetPackage.Price,
					"updated_at":            now,
				}).Error; err != nil {
				return err
			}
			if err := tx.Create(job).Error; err != nil {
				return err
			}
			return tx.Create(event).Error
		}); err != nil {
			log.Printf("[telegram][upgrade] instance=%s persist failed after apply limits, reverting to cpu=%d ram_mb=%d disk_gb=%d: %v", ms.Instance.IncusInstanceName, ms.Service.CPUSnapshot, ms.Service.RAMMBSnapshot, ms.Service.DiskGBSnapshot, err)
			revertOpID, revertErr := s.actions.ApplyResourceLimits(ms.Instance.IncusInstanceName, ms.Service.CPUSnapshot, ms.Service.RAMMBSnapshot, ms.Service.DiskGBSnapshot)
			if revertErr != nil {
				log.Printf("[telegram][upgrade] instance=%s revert limits failed after persist error: %v", ms.Instance.IncusInstanceName, revertErr)
				return nil, revertErr
			}
			log.Printf("[telegram][upgrade] instance=%s revert limits operation_id=%s after persist error", ms.Instance.IncusInstanceName, revertOpID)
			return nil, err
		}

		result.Applied = true
		result.Status = "completed"
	case "qris":
		order.Status = "awaiting_payment"
		payment.Status = "pending"
		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			return tx.Create(&payment).Error
		}); err != nil {
			return nil, err
		}
	}

	return result, nil
}
