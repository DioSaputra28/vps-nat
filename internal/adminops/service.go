package adminops

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	repo    *Repository
	metrics DashboardMetricsProvider
}

func NewService(repo *Repository, metrics DashboardMetricsProvider) *Service {
	return &Service{
		repo:    repo,
		metrics: metrics,
	}
}

func (s *Service) DashboardOverview(ctx context.Context) (*DashboardOverviewResult, error) {
	totalUsers, activeUsers, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, err
	}

	activeServices, runningContainers, stoppedContainers, provisioningPending, err := s.repo.CountServices(ctx)
	if err != nil {
		return nil, err
	}

	cpuCores, ramMB, diskGB, err := s.repo.AllocatedResources(ctx)
	if err != nil {
		return nil, err
	}

	grossRevenue, refundedAmount, err := s.repo.RevenueSummary(ctx)
	if err != nil {
		return nil, err
	}

	openAlertCount, err := s.repo.CountOpenAlerts(ctx)
	if err != nil {
		return nil, err
	}
	latestOpenAlerts, err := s.repo.ListLatestOpenAlerts(ctx, 10)
	if err != nil {
		return nil, err
	}

	live := DashboardLiveSnapshot{}
	if s.metrics != nil {
		live = s.metrics.Snapshot(ctx)
	}

	alertItems := make([]AlertSummaryItem, 0, len(latestOpenAlerts))
	for i := range latestOpenAlerts {
		item := toAlertSummaryItem(&latestOpenAlerts[i])
		alertItems = append(alertItems, item)
	}

	return &DashboardOverviewResult{
		Users: DashboardUserStats{
			Total:  totalUsers,
			Active: activeUsers,
		},
		Services: DashboardServiceStats{
			Active:              activeServices,
			RunningContainers:   runningContainers,
			StoppedContainers:   stoppedContainers,
			ProvisioningPending: provisioningPending,
		},
		ResourceUsage: DashboardResourceUsage{
			Allocated: DashboardAllocatedResources{
				CPUCores: cpuCores,
				RAMMB:    ramMB,
				DiskGB:   diskGB,
			},
			Live: DashboardLiveResources{
				CPUUsagePercent: live.CPUUsagePercent,
				RAMUsageBytes:   live.RAMUsageBytes,
				DiskUsageBytes:  live.DiskUsageBytes,
				WarmingUp:       live.WarmingUp,
				SampledAt:       live.LastSampleAt,
				InstanceCount:   live.InstanceCount,
			},
		},
		Revenue: DashboardRevenueStats{
			GrossRevenue:   grossRevenue,
			RefundedAmount: refundedAmount,
			NetRevenue:     grossRevenue - refundedAmount,
		},
		Alerts: DashboardAlertStats{
			OpenCount:       openAlertCount,
			LatestOpenItems: alertItems,
		},
		Incus: DashboardIncusStats{
			LiveAvailable: live.LiveAvailable,
			LastSampleAt:  live.LastSampleAt,
			Warning:       live.Warning,
		},
	}, nil
}

func (s *Service) FinanceSummary(ctx context.Context) (*FinanceSummaryResult, error) {
	grossRevenue, refundedAmount, err := s.repo.RevenueSummary(ctx)
	if err != nil {
		return nil, err
	}

	totalServerCost, serverCostCount, lastRecordedAt, err := s.repo.ServerCostSummary(ctx)
	if err != nil {
		return nil, err
	}

	netRevenue := grossRevenue - refundedAmount
	difference := netRevenue - totalServerCost

	return &FinanceSummaryResult{
		GrossRevenue:          grossRevenue,
		RefundedAmount:        refundedAmount,
		NetRevenue:            netRevenue,
		TotalServerCost:       totalServerCost,
		DifferenceToBreakEven: difference,
		IsBreakEven:           difference >= 0,
		ServerCostCount:       serverCostCount,
		LastRecordedAt:        lastRecordedAt,
	}, nil
}

func (s *Service) ListServerCosts(ctx context.Context) (*ServerCostListResult, error) {
	costs, err := s.repo.ListServerCosts(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]ServerCostItem, 0, len(costs))
	for i := range costs {
		items = append(items, toServerCostItem(&costs[i]))
	}

	return &ServerCostListResult{Items: items}, nil
}

func (s *Service) ListActivityLogs(ctx context.Context, input ListActivityLogsInput) (*ActivityLogListResult, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	input.Page = page
	input.Limit = limit

	logs, total, err := s.repo.ListActivityLogs(ctx, input)
	if err != nil {
		return nil, err
	}

	adminIDs := make([]string, 0)
	userIDs := make([]string, 0)
	for i := range logs {
		if logs[i].ActorID == nil {
			continue
		}
		switch logs[i].ActorType {
		case "admin":
			adminIDs = append(adminIDs, *logs[i].ActorID)
		case "user":
			userIDs = append(userIDs, *logs[i].ActorID)
		}
	}

	adminMap := map[string]model.AdminUser{}
	if len(adminIDs) > 0 {
		admins, err := s.repo.FindAdminsByIDs(ctx, uniqueStrings(adminIDs))
		if err != nil {
			return nil, err
		}
		for i := range admins {
			adminMap[admins[i].ID] = admins[i]
		}
	}

	userMap := map[string]model.User{}
	if len(userIDs) > 0 {
		users, err := s.repo.FindUsersByIDs(ctx, uniqueStrings(userIDs))
		if err != nil {
			return nil, err
		}
		for i := range users {
			userMap[users[i].ID] = users[i]
		}
	}

	items := make([]ActivityLogItem, 0, len(logs))
	for i := range logs {
		item := ActivityLogItem{
			ID:         logs[i].ID,
			ActorType:  logs[i].ActorType,
			ActorID:    logs[i].ActorID,
			Action:     logs[i].Action,
			TargetType: logs[i].TargetType,
			TargetID:   logs[i].TargetID,
			Metadata:   logs[i].Metadata,
			CreatedAt:  logs[i].CreatedAt,
		}
		if logs[i].ActorID != nil {
			switch logs[i].ActorType {
			case "admin":
				if admin, ok := adminMap[*logs[i].ActorID]; ok {
					email := admin.Email
					role := admin.Role
					status := admin.Status
					item.ActorSummary = &ActivityActorRef{
						ID:        admin.ID,
						Type:      "admin",
						Email:     &email,
						Role:      &role,
						Status:    &status,
						CreatedAt: admin.CreatedAt,
					}
				}
			case "user":
				if user, ok := userMap[*logs[i].ActorID]; ok {
					displayName := user.DisplayName
					status := user.Status
					telegramID := user.TelegramID
					item.ActorSummary = &ActivityActorRef{
						ID:               user.ID,
						Type:             "user",
						TelegramID:       &telegramID,
						TelegramUsername: user.TelegramUsername,
						DisplayName:      &displayName,
						Status:           &status,
						CreatedAt:        user.CreatedAt,
					}
				}
			}
		}
		items = append(items, item)
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}

	return &ActivityLogListResult{
		Items:      items,
		Page:       page,
		Limit:      limit,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) ListAlerts(ctx context.Context, input ListAlertsInput) (*AlertListResult, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	input.Page = page
	input.Limit = limit

	alerts, total, err := s.repo.ListAlerts(ctx, input)
	if err != nil {
		return nil, err
	}

	items := make([]AlertSummaryItem, 0, len(alerts))
	for i := range alerts {
		items = append(items, toAlertSummaryItem(&alerts[i]))
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}

	return &AlertListResult{
		Items:      items,
		Page:       page,
		Limit:      limit,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) CreateServerCost(ctx context.Context, admin model.AdminUser, input CreateServerCostInput) (*ServerCostItem, error) {
	if strings.TrimSpace(input.NodeID) == "" || input.PurchaseCost < 0 {
		return nil, ErrInvalidServerCost
	}

	node, err := s.findNode(ctx, input.NodeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	now := time.Now().UTC()
	recordedAt := now
	if input.RecordedAt != nil && !input.RecordedAt.IsZero() {
		recordedAt = input.RecordedAt.UTC()
	}

	cost := &model.ServerCost{
		ID:               uuid.NewString(),
		NodeID:           node.ID,
		PurchaseCost:     input.PurchaseCost,
		Notes:            normalizeOptionalString(input.Notes),
		RecordedAt:       recordedAt,
		CreatedByAdminID: &admin.ID,
		CreatedAt:        now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(cost).Error; err != nil {
			return err
		}
		return tx.Create(&model.ActivityLog{
			ID:         uuid.NewString(),
			ActorType:  "admin",
			ActorID:    &admin.ID,
			Action:     "server_cost.created",
			TargetType: "server_cost",
			TargetID:   &cost.ID,
			Metadata: map[string]any{
				"node_id":       node.ID,
				"purchase_cost": cost.PurchaseCost,
				"notes":         cost.Notes,
				"recorded_at":   recordedAt,
			},
			CreatedAt: now,
		}).Error
	}); err != nil {
		return nil, err
	}

	cost.Node = node
	cost.CreatedByAdmin = &admin
	item := toServerCostItem(cost)
	return &item, nil
}

func (s *Service) AdjustWallet(ctx context.Context, admin model.AdminUser, input WalletAdjustmentInput) (*WalletAdjustmentResult, error) {
	if strings.TrimSpace(input.UserID) == "" || input.Amount <= 0 || strings.TrimSpace(input.Reason) == "" {
		return nil, ErrInvalidRequest
	}

	adjustmentType := strings.ToLower(strings.TrimSpace(input.AdjustmentType))
	if adjustmentType != "credit" && adjustmentType != "debit" {
		return nil, ErrInvalidAdjustment
	}

	user, err := s.repo.FindUserWithWalletByID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if user.Wallet == nil {
		return nil, ErrNotFound
	}

	now := time.Now().UTC()
	var result WalletAdjustmentResult

	err = s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var wallet model.Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&wallet, "id = ?", user.Wallet.ID).Error; err != nil {
			return err
		}

		balanceBefore := wallet.Balance
		balanceAfter := balanceBefore
		direction := "credit"
		if adjustmentType == "credit" {
			balanceAfter += input.Amount
		} else {
			if balanceBefore < input.Amount {
				return ErrInsufficientBalance
			}
			balanceAfter -= input.Amount
			direction = "debit"
		}

		if err := tx.Model(&wallet).Update("balance", balanceAfter).Error; err != nil {
			return err
		}

		txn := &model.WalletTransaction{
			ID:              uuid.NewString(),
			WalletID:        wallet.ID,
			UserID:          user.ID,
			Direction:       direction,
			TransactionType: "admin_adjustment",
			Amount:          input.Amount,
			BalanceBefore:   balanceBefore,
			BalanceAfter:    balanceAfter,
			SourceType:      "admin_adjustment",
			SourceID:        nil,
			Note:            stringPtr(input.Reason),
			CreatedAt:       now,
		}
		if err := tx.Create(txn).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.ActivityLog{
			ID:         uuid.NewString(),
			ActorType:  "admin",
			ActorID:    &admin.ID,
			Action:     "wallet.adjusted",
			TargetType: "user",
			TargetID:   &user.ID,
			Metadata: map[string]any{
				"direction":      direction,
				"amount":         input.Amount,
				"wallet_id":      wallet.ID,
				"transaction_id": txn.ID,
				"reason":         input.Reason,
				"balance_before": balanceBefore,
				"balance_after":  balanceAfter,
			},
			CreatedAt: now,
		}).Error; err != nil {
			return err
		}

		wallet.Balance = balanceAfter
		result = WalletAdjustmentResult{
			User: WalletUserSummary{
				ID:          user.ID,
				TelegramID:  user.TelegramID,
				DisplayName: user.DisplayName,
				Status:      user.Status,
				CreatedAt:   user.CreatedAt,
			},
			Wallet: AdminWalletSummary{
				ID:      wallet.ID,
				Balance: wallet.Balance,
			},
			Adjustment: WalletAdjustmentSummary{
				TransactionID: txn.ID,
				Direction:     direction,
				Amount:        input.Amount,
				BalanceBefore: balanceBefore,
				BalanceAfter:  balanceAfter,
				Reason:        input.Reason,
				CreatedAt:     now,
			},
			PerformedBy: AdminUserSummary{
				ID:        admin.ID,
				Email:     admin.Email,
				Role:      admin.Role,
				Status:    admin.Status,
				CreatedAt: admin.CreatedAt,
			},
		}
		result.User.TelegramUsername = user.TelegramUsername
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Service) findNode(ctx context.Context, nodeID string) (*model.Node, error) {
	var node model.Node
	if err := s.repo.db.WithContext(ctx).First(&node, "id = ?", strings.TrimSpace(nodeID)).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

func toServerCostItem(cost *model.ServerCost) ServerCostItem {
	item := ServerCostItem{
		ID:           cost.ID,
		NodeID:       cost.NodeID,
		PurchaseCost: cost.PurchaseCost,
		Notes:        cost.Notes,
		RecordedAt:   cost.RecordedAt,
		CreatedAt:    cost.CreatedAt,
	}
	if cost.Node != nil {
		item.Node = AdminNodeSummary{
			ID:          cost.Node.ID,
			Name:        cost.Node.Name,
			DisplayName: cost.Node.DisplayName,
			PublicIP:    cost.Node.PublicIP,
			Status:      cost.Node.Status,
		}
	}
	if cost.CreatedByAdmin != nil {
		item.CreatedByAdmin = &AdminUserSummary{
			ID:        cost.CreatedByAdmin.ID,
			Email:     cost.CreatedByAdmin.Email,
			Role:      cost.CreatedByAdmin.Role,
			Status:    cost.CreatedByAdmin.Status,
			CreatedAt: cost.CreatedByAdmin.CreatedAt,
		}
	}
	return item
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func toAlertSummaryItem(alert *model.ResourceAlert) AlertSummaryItem {
	item := AlertSummaryItem{
		ID:               alert.ID,
		AlertType:        alert.AlertType,
		ThresholdPercent: alert.ThresholdPercent,
		DurationMinutes:  alert.DurationMinutes,
		Status:           alert.Status,
		OpenedAt:         alert.OpenedAt,
		ResolvedAt:       alert.ResolvedAt,
		Metadata:         alert.Metadata,
	}

	if alert.Service != nil {
		serviceSummary := &AlertServiceSummary{
			ID:            alert.Service.ID,
			OwnerUserID:   alert.Service.OwnerUserID,
			PackageName:   alert.Service.PackageNameSnapshot,
			ServiceStatus: alert.Service.Status,
		}
		if alert.Service.OwnerUser != nil {
			displayName := alert.Service.OwnerUser.DisplayName
			telegramID := alert.Service.OwnerUser.TelegramID
			serviceSummary.OwnerDisplayName = &displayName
			serviceSummary.OwnerTelegramID = &telegramID
		}
		if alert.Service.Instance != nil {
			serviceSummary.ContainerID = &alert.Service.Instance.ID
			serviceSummary.IncusInstanceName = &alert.Service.Instance.IncusInstanceName
		}
		item.ServiceSummary = serviceSummary
	}

	if alert.Node != nil {
		item.NodeSummary = &AdminNodeSummary{
			ID:          alert.Node.ID,
			Name:        alert.Node.Name,
			DisplayName: alert.Node.DisplayName,
			PublicIP:    alert.Node.PublicIP,
			Status:      alert.Node.Status,
		}
	}

	return item
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
