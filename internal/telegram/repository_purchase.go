package telegram

import (
	"context"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/gorm"
)

func (r *Repository) FindActiveNode(ctx context.Context) (*model.Node, error) {
	var node model.Node
	if err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Order("created_at ASC").
		First(&node).Error; err != nil {
		return nil, err
	}

	return &node, nil
}

func (r *Repository) FindPurchaseOrderForUser(ctx context.Context, telegramID int64, orderID string) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Joins("JOIN users ON users.id = orders.user_id").
		Where("users.telegram_id = ? AND orders.id = ? AND orders.order_type = ?", telegramID, orderID, "purchase").
		Preload("User").
		Preload("User.Wallet").
		Preload("Payments").
		Preload("Service").
		Preload("Service.Instance").
		Preload("Service.PortMappings", "is_active = ?", true).
		First(&order).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *Repository) FindPurchaseOrderByID(ctx context.Context, orderID string) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("orders.id = ? AND orders.order_type = ?", orderID, "purchase").
		Preload("User").
		Preload("User.Wallet").
		Preload("Payments").
		Preload("Service").
		Preload("Service.Instance").
		Preload("Service.PortMappings", "is_active = ?", true).
		First(&order).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *Repository) FindWalletTopupForUser(ctx context.Context, telegramID int64, topupID string) (*model.WalletTopup, error) {
	var topup model.WalletTopup
	err := r.db.WithContext(ctx).
		Model(&model.WalletTopup{}).
		Joins("JOIN users ON users.id = wallet_topups.user_id").
		Where("users.telegram_id = ? AND wallet_topups.id = ?", telegramID, topupID).
		Preload("User").
		Preload("User.Wallet").
		Preload("Payments").
		First(&topup).Error
	if err != nil {
		return nil, err
	}

	return &topup, nil
}

func (r *Repository) FindWalletTopupByID(ctx context.Context, topupID string) (*model.WalletTopup, error) {
	var topup model.WalletTopup
	err := r.db.WithContext(ctx).
		Model(&model.WalletTopup{}).
		Where("wallet_topups.id = ?", topupID).
		Preload("User").
		Preload("User.Wallet").
		Preload("Payments").
		First(&topup).Error
	if err != nil {
		return nil, err
	}

	return &topup, nil
}

func (r *Repository) HasHostnameConflict(ctx context.Context, hostname string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(hostname))

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.ServiceInstance{}).
		Where("LOWER(incus_instance_name) = ?", normalized).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	query := r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Joins("JOIN orders ON orders.id = payments.order_id").
		Where("orders.order_type = ?", "purchase").
		Where("orders.status IN ?", []string{"pending", "awaiting_payment", "paid", "processing"})

	switch r.db.Dialector.Name() {
	case "sqlite":
		query = query.Where("LOWER(json_extract(payments.provider_payload, '$.hostname')) = ?", normalized)
	default:
		query = query.Where("LOWER(payments.provider_payload->>'hostname') = ?", normalized)
	}

	if err := query.Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *Repository) FindNextAvailablePublicPort(ctx context.Context, publicIP string, protocol string, excluded map[int]struct{}) (int, error) {
	const (
		minPort = 20000
		maxPort = 39999
	)

	var used []int
	if err := r.db.WithContext(ctx).
		Model(&model.ServicePortMapping{}).
		Where("public_ip = ? AND protocol = ? AND is_active = ?", publicIP, protocol, true).
		Pluck("public_port", &used).Error; err != nil {
		return 0, err
	}

	usedSet := make(map[int]struct{}, len(used)+len(excluded))
	for _, port := range used {
		usedSet[port] = struct{}{}
	}
	for port := range excluded {
		usedSet[port] = struct{}{}
	}

	for port := minPort; port <= maxPort; port++ {
		if _, exists := usedSet[port]; !exists {
			return port, nil
		}
	}

	return 0, gorm.ErrRecordNotFound
}

func (r *Repository) IsPublicPortAvailable(ctx context.Context, publicIP string, protocol string, port int) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.ServicePortMapping{}).
		Where("public_ip = ? AND protocol = ? AND public_port = ? AND is_active = ?", publicIP, protocol, port, true).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count == 0, nil
}

func (r *Repository) UpdatePaymentProviderPayload(ctx context.Context, paymentID string, payload map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&model.Payment{}).
		Where("id = ?", paymentID).
		Updates(&model.Payment{
			ProviderPayload: payload,
			UpdatedAt:       time.Now().UTC(),
		}).Error
}
