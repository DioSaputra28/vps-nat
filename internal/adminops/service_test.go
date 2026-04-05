package adminops

import (
	"context"
	"testing"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/lxc/incus/v6/shared/api"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDashboardCacheWarmupThenLiveSample(t *testing.T) {
	t.Parallel()

	server := &fakeDashboardServer{
		instances: []api.InstanceFull{
			newRunningInstance("c1", 1_000_000_000, 1_000_000_000, 256*1024*1024, 1024*1024*1024),
		},
	}
	cache := NewDashboardCache(server, 30*time.Second)

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	first := cache.Snapshot(context.Background())
	if !first.WarmingUp {
		t.Fatalf("expected first snapshot to be warming up")
	}
	if first.CPUUsagePercent != nil {
		t.Fatalf("expected first snapshot cpu percent to be nil")
	}

	server.instances[0] = newRunningInstance("c1", 1_100_000_000, 1_000_000_000, 512*1024*1024, 2*1024*1024*1024)
	time.Sleep(20 * time.Millisecond)

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("second refresh failed: %v", err)
	}

	second := cache.Snapshot(context.Background())
	if second.WarmingUp {
		t.Fatalf("expected second snapshot to be ready")
	}
	if second.CPUUsagePercent == nil || *second.CPUUsagePercent <= 0 {
		t.Fatalf("expected cpu percent to be computed, got %#v", second.CPUUsagePercent)
	}
	if second.RAMUsageBytes == 0 || second.DiskUsageBytes == 0 {
		t.Fatalf("expected live resource usage to be populated, got %#v", second)
	}
}

func TestDashboardOverviewSummariesDBAndLive(t *testing.T) {
	t.Parallel()

	svc, _ := newAdminOpsServiceHarness(t)
	svc.metrics = fakeDashboardMetricsProvider{
		snapshot: DashboardLiveSnapshot{
			LiveAvailable:   true,
			CPUUsagePercent: floatPtr(37.5),
			RAMUsageBytes:   4096,
			DiskUsageBytes:  8192,
			WarmingUp:       false,
			InstanceCount:   2,
			LastSampleAt:    timePtr(time.Now().UTC()),
		},
	}

	seedAdminOpsPackage(t, svc.repo.db, "pkg-1", "Package 1", 1, 1024, 10, 10000, 30, true)
	seedAdminOpsPackage(t, svc.repo.db, "pkg-2", "Package 2", 2, 2048, 20, 20000, 30, true)
	seedAdminOpsNode(t, svc.repo.db, "node-1", "dio-node", "Dio Node", "82.25.44.113", "active")
	username := "alice"
	seedAdminOpsUser(t, svc.repo.db, "user-1", 7001, "alice", &username, "active")
	seedAdminOpsUser(t, svc.repo.db, "user-2", 7002, "bob", nil, "inactive")
	seedAdminOpsService(t, svc.repo.db, "service-1", "user-1", "pkg-1", "active", 1, 1024, 10)
	seedAdminOpsService(t, svc.repo.db, "service-2", "user-1", "pkg-2", "stopped", 2, 2048, 20)
	seedAdminOpsInstance(t, svc.repo.db, "instance-1", "service-1", "running")
	seedAdminOpsInstance(t, svc.repo.db, "instance-2", "service-2", "stopped")
	seedAdminOpsProvisioningJob(t, svc.repo.db, "job-1", "queued")
	seedAdminOpsOrder(t, svc.repo.db, "order-1", "user-1", "purchase", "paid", 10000)
	seedAdminOpsOrder(t, svc.repo.db, "order-2", "user-1", "renewal", "paid", 5000)
	seedAdminOpsOrder(t, svc.repo.db, "order-3", "user-1", "upgrade", "paid", 7000)
	seedAdminOpsRefund(t, svc.repo.db, "wallet-user-1", "user-1", 2500)

	result, err := svc.DashboardOverview(context.Background())
	if err != nil {
		t.Fatalf("DashboardOverview returned error: %v", err)
	}

	if result.Users.Total != 2 || result.Users.Active != 1 {
		t.Fatalf("unexpected user summary: %#v", result.Users)
	}
	if result.Services.Active != 1 || result.Services.RunningContainers != 1 || result.Services.StoppedContainers != 1 || result.Services.ProvisioningPending != 1 {
		t.Fatalf("unexpected service summary: %#v", result.Services)
	}
	if result.ResourceUsage.Allocated.CPUCores != 1 || result.ResourceUsage.Allocated.RAMMB != 1024 || result.ResourceUsage.Allocated.DiskGB != 10 {
		t.Fatalf("unexpected allocated resources: %#v", result.ResourceUsage.Allocated)
	}
	if result.Revenue.GrossRevenue != 22000 || result.Revenue.RefundedAmount != 2500 || result.Revenue.NetRevenue != 19500 {
		t.Fatalf("unexpected revenue summary: %#v", result.Revenue)
	}
	if !result.Incus.LiveAvailable || result.Incus.Warning != nil {
		t.Fatalf("unexpected incus summary: %#v", result.Incus)
	}
}

func TestFinanceSummaryAndWalletAdjustment(t *testing.T) {
	t.Parallel()

	svc, db := newAdminOpsServiceHarness(t)
	seedAdminOpsPackage(t, db, "pkg-1", "Package 1", 1, 1024, 10, 10000, 30, true)
	seedAdminOpsNode(t, db, "node-1", "dio-node", "Dio Node", "82.25.44.113", "active")
	seedAdminOpsUser(t, db, "user-1", 7001, "alice", nil, "active")
	seedAdminOpsOrder(t, db, "order-1", "user-1", "purchase", "paid", 10000)
	seedAdminOpsOrder(t, db, "order-2", "user-1", "renewal", "paid", 5000)
	seedAdminOpsOrder(t, db, "order-3", "user-1", "upgrade", "paid", 7000)
	seedAdminOpsRefund(t, db, "wallet-user-1", "user-1", 1500)
	seedAdminOpsServerCost(t, db, "cost-1", "node-1", 2000, nil, time.Now().UTC().Add(-2*time.Hour))
	seedAdminOpsServerCost(t, db, "cost-2", "node-1", 3000, nil, time.Now().UTC().Add(-time.Hour))

	summary, err := svc.FinanceSummary(context.Background())
	if err != nil {
		t.Fatalf("FinanceSummary returned error: %v", err)
	}
	if summary.GrossRevenue != 22000 || summary.RefundedAmount != 1500 || summary.NetRevenue != 20500 {
		t.Fatalf("unexpected finance summary: %#v", summary)
	}
	if summary.TotalServerCost != 5000 || summary.ServerCostCount != 2 || !summary.IsBreakEven {
		t.Fatalf("unexpected server cost summary: %#v", summary)
	}

	seedAdminOpsAdmin(t, db, "admin-1", "admin@example.com", "admin")
	adjusted, err := svc.AdjustWallet(context.Background(), model.AdminUser{ID: "admin-1", Email: "admin@example.com", Role: "admin", Status: "active"}, WalletAdjustmentInput{
		UserID:         "user-1",
		AdjustmentType: "credit",
		Amount:         2500,
		Reason:         "bonus",
	})
	if err != nil {
		t.Fatalf("AdjustWallet credit returned error: %v", err)
	}
	if adjusted.Wallet.Balance != 2500 {
		t.Fatalf("expected wallet balance 2500, got %d", adjusted.Wallet.Balance)
	}

	adjusted, err = svc.AdjustWallet(context.Background(), model.AdminUser{ID: "admin-1", Email: "admin@example.com", Role: "admin", Status: "active"}, WalletAdjustmentInput{
		UserID:         "user-1",
		AdjustmentType: "debit",
		Amount:         500,
		Reason:         "correction",
	})
	if err != nil {
		t.Fatalf("AdjustWallet debit returned error: %v", err)
	}
	if adjusted.Wallet.Balance != 2000 {
		t.Fatalf("expected wallet balance 2000, got %d", adjusted.Wallet.Balance)
	}

	var txCount int64
	if err := db.Model(&model.WalletTransaction{}).Where("source_type = ? AND transaction_type = ?", "admin_adjustment", "admin_adjustment").Count(&txCount).Error; err != nil {
		t.Fatalf("failed to count wallet tx: %v", err)
	}
	if txCount != 2 {
		t.Fatalf("expected 2 wallet adjustment tx, got %d", txCount)
	}
}

func TestCreateServerCostWritesActivityLog(t *testing.T) {
	t.Parallel()

	svc, db := newAdminOpsServiceHarness(t)
	seedAdminOpsNode(t, db, "node-1", "dio-node", "Dio Node", "82.25.44.113", "active")
	admin := seedAdminOpsAdmin(t, db, "admin-1", "super@example.com", "super_admin")

	result, err := svc.CreateServerCost(context.Background(), admin, CreateServerCostInput{
		NodeID:       "node-1",
		PurchaseCost: 1500000,
		Notes:        stringPtr("hardware purchase"),
	})
	if err != nil {
		t.Fatalf("CreateServerCost returned error: %v", err)
	}
	if result.Node.ID != "node-1" || result.PurchaseCost != 1500000 {
		t.Fatalf("unexpected server cost result: %#v", result)
	}

	var logs int64
	if err := db.Model(&model.ActivityLog{}).Where("action = ? AND target_type = ?", "server_cost.created", "server_cost").Count(&logs).Error; err != nil {
		t.Fatalf("failed to count activity logs: %v", err)
	}
	if logs != 1 {
		t.Fatalf("expected 1 activity log, got %d", logs)
	}
}

func TestListActivityLogsIncludesActorSummaryAndFilters(t *testing.T) {
	t.Parallel()

	svc, db := newAdminOpsServiceHarness(t)
	seedAdminOpsAdmin(t, db, "admin-1", "admin@example.com", "admin")
	seedAdminOpsUser(t, db, "user-1", 7001, "alice", nil, "active")

	now := time.Now().UTC().Truncate(time.Second)
	actorID := "admin-1"
	userID := "user-1"
	if err := db.Create(&model.ActivityLog{
		ID:         "log-1",
		ActorType:  "admin",
		ActorID:    &actorID,
		Action:     "wallet.adjusted",
		TargetType: "user",
		TargetID:   &userID,
		Metadata:   map[string]any{"amount": 1000},
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("failed to seed admin activity log: %v", err)
	}
	if err := db.Create(&model.ActivityLog{
		ID:         "log-2",
		ActorType:  "system",
		Action:     "resource_alert.opened",
		TargetType: "service",
		TargetID:   nil,
		Metadata:   map[string]any{"alert_type": "cpu_high"},
		CreatedAt:  now.Add(time.Second),
	}).Error; err != nil {
		t.Fatalf("failed to seed system activity log: %v", err)
	}

	result, err := svc.ListActivityLogs(context.Background(), ListActivityLogsInput{
		Page:      1,
		Limit:     10,
		ActorType: "admin",
	})
	if err != nil {
		t.Fatalf("ListActivityLogs returned error: %v", err)
	}

	if result.TotalItems != 1 || len(result.Items) != 1 {
		t.Fatalf("expected 1 filtered activity log, got total=%d items=%d", result.TotalItems, len(result.Items))
	}
	if result.Items[0].Action != "wallet.adjusted" {
		t.Fatalf("expected wallet.adjusted action, got %s", result.Items[0].Action)
	}
	if result.Items[0].ActorSummary == nil || result.Items[0].ActorSummary.Email == nil || *result.Items[0].ActorSummary.Email != "admin@example.com" {
		t.Fatalf("expected actor summary email admin@example.com, got %#v", result.Items[0].ActorSummary)
	}
}

func TestDashboardOverviewIncludesOpenAlerts(t *testing.T) {
	t.Parallel()

	svc, db := newAdminOpsServiceHarness(t)
	seedAdminOpsPackage(t, db, "pkg-1", "Package 1", 1, 1024, 10, 10000, 30, true)
	seedAdminOpsNode(t, db, "node-1", "dio-node", "Dio Node", "82.25.44.113", "active")
	seedAdminOpsUser(t, db, "user-1", 7001, "alice", nil, "active")
	seedAdminOpsService(t, db, "service-1", "user-1", "pkg-1", "active", 1, 1024, 10)
	seedAdminOpsOpenAlert(t, db, "alert-1", "service-1", "node-1", "cpu_high", "open")
	seedAdminOpsOpenAlert(t, db, "alert-2", "service-1", "node-1", "ram_high", "resolved")

	result, err := svc.DashboardOverview(context.Background())
	if err != nil {
		t.Fatalf("DashboardOverview returned error: %v", err)
	}

	if result.Alerts.OpenCount != 1 {
		t.Fatalf("expected 1 open alert, got %d", result.Alerts.OpenCount)
	}
	if len(result.Alerts.LatestOpenItems) != 1 {
		t.Fatalf("expected 1 latest open alert item, got %d", len(result.Alerts.LatestOpenItems))
	}
	if result.Alerts.LatestOpenItems[0].AlertType != "cpu_high" {
		t.Fatalf("expected cpu_high alert, got %s", result.Alerts.LatestOpenItems[0].AlertType)
	}
}

func TestAlertMonitorOpensAndResolvesAlerts(t *testing.T) {
	t.Parallel()

	svc, db := newAdminOpsServiceHarness(t)
	seedAdminOpsPackage(t, db, "pkg-1", "Package 1", 1, 1024, 10, 10000, 30, true)
	seedAdminOpsNode(t, db, "node-1", "dio-node", "Dio Node", "82.25.44.113", "active")
	seedAdminOpsUser(t, db, "user-1", 7001, "alice", nil, "active")
	seedAdminOpsService(t, db, "service-1", "user-1", "pkg-1", "active", 1, 1024, 10)
	seedAdminOpsInstance(t, db, "instance-1", "service-1", "running")

	now := time.Now().UTC().Truncate(time.Second)
	server := &fakeDashboardServer{
		instances: []api.InstanceFull{
			newRunningInstanceWithTotals("instance-1", 1_000_000_000, 1_000_000_000, 990, 1000, 1024, 2048),
		},
	}
	notifier := &fakeAlertNotifier{}
	monitor := NewAlertMonitor(svc.repo, server, notifier, AlertMonitorConfig{
		Interval:         30 * time.Second,
		ThresholdPercent: 95,
		Duration:         10 * time.Minute,
	})
	monitor.now = func() time.Time { return now }

	if err := monitor.RunOnce(context.Background()); err != nil {
		t.Fatalf("first RunOnce returned error: %v", err)
	}

	server.instances[0] = newRunningInstanceWithTotals("instance-1", 700_000_000_000, 1_000_000_000, 990, 1000, 1980, 2000)
	monitor.now = func() time.Time { return now.Add(11 * time.Minute) }
	if err := monitor.RunOnce(context.Background()); err != nil {
		t.Fatalf("second RunOnce returned error: %v", err)
	}

	var openCount int64
	if err := db.Model(&model.ResourceAlert{}).Where("status = ?", "open").Count(&openCount).Error; err != nil {
		t.Fatalf("failed counting open alerts: %v", err)
	}
	if openCount != 2 {
		t.Fatalf("expected 2 open alerts, got %d", openCount)
	}
	if notifier.calls != 2 {
		t.Fatalf("expected 2 alert notifications, got %d", notifier.calls)
	}

	server.instances[0] = newRunningInstanceWithTotals("instance-1", 701_000_000_000, 1_000_000_000, 100, 1000, 100, 2000)
	monitor.now = func() time.Time { return now.Add(12 * time.Minute) }
	if err := monitor.RunOnce(context.Background()); err != nil {
		t.Fatalf("third RunOnce returned error: %v", err)
	}

	var resolvedCount int64
	if err := db.Model(&model.ResourceAlert{}).Where("status = ?", "resolved").Count(&resolvedCount).Error; err != nil {
		t.Fatalf("failed counting resolved alerts: %v", err)
	}
	if resolvedCount != 2 {
		t.Fatalf("expected 2 resolved alerts, got %d", resolvedCount)
	}
}

type fakeDashboardMetricsProvider struct {
	snapshot DashboardLiveSnapshot
}

func (f fakeDashboardMetricsProvider) Snapshot(ctx context.Context) DashboardLiveSnapshot {
	return f.snapshot
}

type fakeDashboardServer struct {
	instances []api.InstanceFull
	err       error
}

func (f *fakeDashboardServer) GetInstancesFull(instanceType api.InstanceType) ([]api.InstanceFull, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.instances, nil
}

type fakeAlertNotifier struct {
	calls    int
	messages []string
	err      error
}

func (f *fakeAlertNotifier) NotifyAlert(ctx context.Context, message string) error {
	f.calls++
	f.messages = append(f.messages, message)
	return f.err
}

func newAdminOpsServiceHarness(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.Wallet{},
		&model.WalletTransaction{},
		&model.Order{},
		&model.Package{},
		&model.ServerCost{},
		&model.Node{},
		&model.Service{},
		&model.ServiceInstance{},
		&model.ProvisioningJob{},
		&model.ActivityLog{},
		&model.ResourceAlert{},
		&model.AdminUser{},
	); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	repo := NewRepository(db)
	svc := NewService(repo, fakeDashboardMetricsProvider{})
	return svc, db
}

func seedAdminOpsUser(t *testing.T, db *gorm.DB, userID string, telegramID int64, displayName string, username *string, status string) string {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	user := model.User{
		ID:               userID,
		TelegramID:       telegramID,
		TelegramUsername: username,
		DisplayName:      displayName,
		Status:           status,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	wallet := model.Wallet{
		ID:        "wallet-" + userID,
		UserID:    userID,
		Balance:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatalf("failed to seed wallet: %v", err)
	}

	return wallet.ID
}

func seedAdminOpsAdmin(t *testing.T, db *gorm.DB, adminID string, email string, role string) model.AdminUser {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	admin := model.AdminUser{
		ID:           adminID,
		Email:        email,
		PasswordHash: "hash",
		Role:         role,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}
	return admin
}

func seedAdminOpsNode(t *testing.T, db *gorm.DB, nodeID string, name string, displayName string, publicIP string, status string) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	node := model.Node{
		ID:          nodeID,
		Name:        name,
		DisplayName: displayName,
		PublicIP:    publicIP,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}
}

func seedAdminOpsPackage(t *testing.T, db *gorm.DB, packageID string, name string, cpu int, ram int, disk int, price int64, durationDays int, active bool) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	pkg := model.Package{
		ID:           packageID,
		Name:         name,
		CPU:          cpu,
		RAMMB:        ram,
		DiskGB:       disk,
		Price:        price,
		DurationDays: durationDays,
		IsActive:     active,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to seed package: %v", err)
	}
}

func seedAdminOpsService(t *testing.T, db *gorm.DB, serviceID string, userID string, packageID string, status string, cpu int, ram int, disk int) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	service := model.Service{
		ID:               serviceID,
		OrderID:          "order-" + serviceID,
		OwnerUserID:      userID,
		CurrentPackageID: packageID,
		Status:           status,
		BillingCycleDays: 30,
		CPUSnapshot:      cpu,
		RAMMBSnapshot:    ram,
		DiskGBSnapshot:   disk,
		PriceSnapshot:    10000,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("failed to seed service: %v", err)
	}
}

func seedAdminOpsInstance(t *testing.T, db *gorm.DB, instanceID string, serviceID string, status string) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	instance := model.ServiceInstance{
		ID:                instanceID,
		ServiceID:         serviceID,
		NodeID:            "node-1",
		IncusInstanceName: instanceID,
		Status:            status,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to seed instance: %v", err)
	}
}

func seedAdminOpsProvisioningJob(t *testing.T, db *gorm.DB, jobID string, status string) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	job := model.ProvisioningJob{
		ID:              jobID,
		JobType:         "provision",
		Status:          status,
		RequestedByType: "admin",
		AttemptCount:    1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("failed to seed job: %v", err)
	}
}

func seedAdminOpsOrder(t *testing.T, db *gorm.DB, orderID string, userID string, orderType string, paymentStatus string, amount int64) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	order := model.Order{
		ID:                   orderID,
		UserID:               userID,
		PackageID:            "pkg-1",
		OrderType:            orderType,
		Status:               "completed",
		PaymentStatus:        paymentStatus,
		PackageNameSnapshot:  "Package",
		CPUSnapshot:          1,
		RAMMBSnapshot:        1024,
		DiskGBSnapshot:       10,
		PriceSnapshot:        amount,
		DurationDaysSnapshot: 30,
		TotalAmount:          amount,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("failed to seed order: %v", err)
	}
}

func seedAdminOpsRefund(t *testing.T, db *gorm.DB, walletID string, userID string, amount int64) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	txn := model.WalletTransaction{
		ID:              "txn-" + walletID,
		WalletID:        walletID,
		UserID:          userID,
		Direction:       "credit",
		TransactionType: "refund",
		Amount:          amount,
		BalanceBefore:   0,
		BalanceAfter:    amount,
		SourceType:      "order",
		CreatedAt:       now,
	}
	if err := db.Create(&txn).Error; err != nil {
		t.Fatalf("failed to seed refund: %v", err)
	}
}

func seedAdminOpsOpenAlert(t *testing.T, db *gorm.DB, alertID string, serviceID string, nodeID string, alertType string, status string) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	alert := model.ResourceAlert{
		ID:               alertID,
		ServiceID:        &serviceID,
		NodeID:           &nodeID,
		AlertType:        alertType,
		ThresholdPercent: 95,
		DurationMinutes:  10,
		Status:           status,
		OpenedAt:         now,
		Metadata:         map[string]any{"instance_name": "instance-1"},
	}
	if status == "resolved" {
		alert.ResolvedAt = &now
	}
	if err := db.Create(&alert).Error; err != nil {
		t.Fatalf("failed to seed resource alert: %v", err)
	}
}

func newRunningInstance(name string, cpuUsage int64, allocatedTime int64, ramUsage int64, diskUsage int64) api.InstanceFull {
	return api.InstanceFull{
		Instance: api.Instance{
			Name:   name,
			Status: "Running",
			Type:   "container",
		},
		State: &api.InstanceState{
			Status: "Running",
			CPU: api.InstanceStateCPU{
				Usage:         cpuUsage,
				AllocatedTime: allocatedTime,
			},
			Memory: api.InstanceStateMemory{
				Usage: ramUsage,
			},
			Disk: map[string]api.InstanceStateDisk{
				"root": {
					Usage: diskUsage,
				},
			},
		},
	}
}

func newRunningInstanceWithTotals(name string, cpuUsage int64, allocatedTime int64, ramUsage int64, ramTotal int64, diskUsage int64, diskTotal int64) api.InstanceFull {
	return api.InstanceFull{
		Instance: api.Instance{
			Name:   name,
			Status: "Running",
			Type:   "container",
		},
		State: &api.InstanceState{
			Status: "Running",
			CPU: api.InstanceStateCPU{
				Usage:         cpuUsage,
				AllocatedTime: allocatedTime,
			},
			Memory: api.InstanceStateMemory{
				Usage: ramUsage,
				Total: ramTotal,
			},
			Disk: map[string]api.InstanceStateDisk{
				"root": {
					Usage: diskUsage,
					Total: diskTotal,
				},
			},
		},
	}
}

func seedAdminOpsServerCost(t *testing.T, db *gorm.DB, costID string, nodeID string, purchaseCost int64, notes *string, recordedAt time.Time) {
	t.Helper()

	cost := model.ServerCost{
		ID:           costID,
		NodeID:       nodeID,
		PurchaseCost: purchaseCost,
		Notes:        notes,
		RecordedAt:   recordedAt.UTC(),
		CreatedAt:    recordedAt.UTC(),
	}
	if err := db.Create(&cost).Error; err != nil {
		t.Fatalf("failed to seed server cost: %v", err)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}
