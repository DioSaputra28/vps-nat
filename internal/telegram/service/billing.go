package service

import (
	"math"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
)

func ExtendExpiry(current *time.Time, durationDays int) *time.Time {
	base := time.Now().UTC()
	if current != nil && current.After(base) {
		base = *current
	}

	next := base.Add(time.Duration(durationDays) * 24 * time.Hour)
	return &next
}

func ValidatePaymentMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "wallet", "qris":
		return true
	default:
		return false
	}
}

func BuildOrder(userID string, pkg *model.Package, orderType string, targetServiceID *string, totalAmount int64, selectedImage *string, now time.Time) model.Order {
	return model.Order{
		ID:                   uuid.NewString(),
		UserID:               userID,
		PackageID:            pkg.ID,
		TargetServiceID:      targetServiceID,
		OrderType:            orderType,
		Status:               "pending",
		PaymentStatus:        "pending",
		SelectedImageAlias:   selectedImage,
		PackageNameSnapshot:  pkg.Name,
		CPUSnapshot:          pkg.CPU,
		RAMMBSnapshot:        pkg.RAMMB,
		DiskGBSnapshot:       pkg.DiskGB,
		PriceSnapshot:        pkg.Price,
		DurationDaysSnapshot: pkg.DurationDays,
		TotalAmount:          totalAmount,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func BuildPayment(orderID *string, purpose string, method string, amount int64, now time.Time) model.Payment {
	return model.Payment{
		ID:        uuid.NewString(),
		OrderID:   orderID,
		Purpose:   purpose,
		Method:    method,
		Amount:    amount,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func IsEligibleUpgrade(currentCPU int, currentRAMMB int, currentDiskGB int, currentPrice int64, pkg *model.Package) bool {
	if pkg == nil {
		return false
	}
	if pkg.CPU < currentCPU || pkg.RAMMB < currentRAMMB || pkg.DiskGB < currentDiskGB {
		return false
	}
	if pkg.Price <= currentPrice {
		return false
	}
	return pkg.CPU > currentCPU || pkg.RAMMB > currentRAMMB || pkg.DiskGB > currentDiskGB
}

func RemainingDurationSeconds(expiresAt *time.Time) int64 {
	if expiresAt == nil || expiresAt.IsZero() {
		return 0
	}
	remaining := int64(time.Until(*expiresAt).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func ProratedUpgradeCost(currentPrice int64, targetPrice int64, remainingSeconds int64, billingCycleDays int) int64 {
	if targetPrice <= currentPrice || remainingSeconds <= 0 || billingCycleDays <= 0 {
		return 0
	}
	billingSeconds := int64(billingCycleDays) * 24 * 60 * 60
	if billingSeconds <= 0 {
		return 0
	}
	value := float64(targetPrice-currentPrice) * (float64(remainingSeconds) / float64(billingSeconds))
	return int64(math.Floor(value))
}

func ProratedRefund(currentPrice int64, remainingSeconds int64, billingCycleDays int) int64 {
	if currentPrice <= 0 || remainingSeconds <= 0 || billingCycleDays <= 0 {
		return 0
	}
	billingSeconds := int64(billingCycleDays) * 24 * 60 * 60
	if billingSeconds <= 0 {
		return 0
	}
	value := float64(currentPrice) * (float64(remainingSeconds) / float64(billingSeconds))
	return int64(math.Floor(value))
}

func SuccessJob(serviceID string, orderID *string, jobType string, actorType string, actorID string, operationID string, payload map[string]any) *model.ProvisioningJob {
	now := time.Now().UTC()
	var opID *string
	if strings.TrimSpace(operationID) != "" {
		opID = &operationID
	}
	return &model.ProvisioningJob{
		ID:               uuid.NewString(),
		ServiceID:        &serviceID,
		OrderID:          orderID,
		JobType:          jobType,
		Status:           "success",
		IncusOperationID: opID,
		RequestedByType:  actorType,
		RequestedByID:    &actorID,
		AttemptCount:     1,
		Payload:          payload,
		StartedAt:        &now,
		FinishedAt:       &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func QueuedJob(serviceID string, orderID *string, jobType string, actorType string, actorID string, payload map[string]any) *model.ProvisioningJob {
	now := time.Now().UTC()
	return &model.ProvisioningJob{
		ID:              uuid.NewString(),
		ServiceID:       &serviceID,
		OrderID:         orderID,
		JobType:         jobType,
		Status:          "queued",
		RequestedByType: actorType,
		RequestedByID:   &actorID,
		AttemptCount:    0,
		Payload:         payload,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}
