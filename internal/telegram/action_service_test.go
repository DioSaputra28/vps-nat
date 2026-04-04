package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRuntimeActionStartRecordsJobAndEvent(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)
	svc.actions = fakeActionExecutor{
		changeStateFn: func(instanceName string, action string) (string, error) {
			if instanceName != "inst-one" || action != "start" {
				t.Fatalf("unexpected executor input: %s %s", instanceName, action)
			}

			return "op-start-1", nil
		},
	}

	now := time.Now().UTC().Truncate(time.Second)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:        3001,
		userID:            "user-1",
		walletID:          "wallet-1",
		containerID:       "container-1",
		serviceID:         "service-1",
		orderID:           "order-1",
		packageID:         "pkg-1",
		packageName:       "Package S",
		packagePrice:      25000,
		packageDuration:   30,
		serviceStatus:     "stopped",
		instanceStatus:    "stopped",
		currentPackageOn:  true,
		servicePrice:      25000,
		serviceCPU:        1,
		serviceRAMMB:      1024,
		serviceDiskGB:     10,
		expiresAt:         now.Add(24 * time.Hour),
		includeSSHMapping: true,
	})

	result, err := svc.RuntimeAction(context.Background(), RuntimeActionInput{
		TelegramID:  3001,
		ContainerID: "container-1",
		Action:      "start",
	})
	if err != nil {
		t.Fatalf("RuntimeAction returned error: %v", err)
	}

	if result.Action != "start" {
		t.Fatalf("expected action start, got %s", result.Action)
	}
	if result.OperationID == nil || *result.OperationID != "op-start-1" {
		t.Fatalf("expected operation id op-start-1, got %#v", result.OperationID)
	}

	var instance model.ServiceInstance
	if err := db.First(&instance, "id = ?", "container-1").Error; err != nil {
		t.Fatalf("failed to load service instance: %v", err)
	}
	if instance.Status != "running" {
		t.Fatalf("expected instance status running, got %s", instance.Status)
	}

	var job model.ProvisioningJob
	if err := db.First(&job, "service_id = ? AND job_type = ?", "service-1", "start").Error; err != nil {
		t.Fatalf("expected provisioning job to be created: %v", err)
	}
	if job.Status != "success" {
		t.Fatalf("expected provisioning job status success, got %s", job.Status)
	}

	var event model.ServiceEvent
	if err := db.First(&event, "service_id = ? AND event_type = ?", "service-1", "container_started").Error; err != nil {
		t.Fatalf("expected service event to be created: %v", err)
	}
}

func TestRenewPreviewUsesCurrentPackageEvenIfInactive(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)

	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(5 * 24 * time.Hour)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3002,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-1",
		packageName:      "Package Legacy",
		packagePrice:     40000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: false,
		servicePrice:     35000,
		serviceCPU:       2,
		serviceRAMMB:     2048,
		serviceDiskGB:    20,
		expiresAt:        expiresAt,
	})

	result, err := svc.RenewPreview(context.Background(), RenewPreviewInput{
		TelegramID:  3002,
		ContainerID: "container-1",
	})
	if err != nil {
		t.Fatalf("RenewPreview returned error: %v", err)
	}

	if result.Package.ID != "pkg-1" {
		t.Fatalf("expected current package id pkg-1, got %s", result.Package.ID)
	}
	if result.Price != 40000 {
		t.Fatalf("expected renewal price 40000, got %d", result.Price)
	}
	if result.NextExpiresAt == nil || !result.NextExpiresAt.After(expiresAt) {
		t.Fatalf("expected next expiry to be after current expiry")
	}
}

func TestRenewSubmitWalletDebitsBalanceAndExtendsExpiry(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)

	now := time.Now().UTC().Truncate(time.Second)
	oldExpiry := now.Add(10 * 24 * time.Hour)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3003,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-1",
		packageName:      "Package S",
		packagePrice:     25000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: true,
		servicePrice:     25000,
		serviceCPU:       1,
		serviceRAMMB:     1024,
		serviceDiskGB:    10,
		expiresAt:        oldExpiry,
		walletBalance:    60000,
	})

	result, err := svc.RenewSubmit(context.Background(), RenewSubmitInput{
		TelegramID:    3003,
		ContainerID:   "container-1",
		PaymentMethod: "wallet",
	})
	if err != nil {
		t.Fatalf("RenewSubmit returned error: %v", err)
	}

	if !result.Applied {
		t.Fatalf("expected renewal to be applied immediately for wallet payment")
	}
	if result.OrderID == "" || result.PaymentID == "" {
		t.Fatalf("expected order and payment ids to be returned")
	}

	var wallet model.Wallet
	if err := db.First(&wallet, "id = ?", "wallet-1").Error; err != nil {
		t.Fatalf("failed to load wallet: %v", err)
	}
	if wallet.Balance != 35000 {
		t.Fatalf("expected wallet balance 35000, got %d", wallet.Balance)
	}

	var service model.Service
	if err := db.First(&service, "id = ?", "service-1").Error; err != nil {
		t.Fatalf("failed to load service: %v", err)
	}
	if service.ExpiresAt == nil || !service.ExpiresAt.After(oldExpiry) {
		t.Fatalf("expected service expiry to be extended")
	}

	var order model.Order
	if err := db.First(&order, "id = ?", result.OrderID).Error; err != nil {
		t.Fatalf("failed to load renewal order: %v", err)
	}
	if order.OrderType != "renewal" || order.Status != "completed" {
		t.Fatalf("unexpected order state: type=%s status=%s", order.OrderType, order.Status)
	}
}

func TestUpgradeOptionsOnlyReturnsStrictlyHigherPackages(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)

	now := time.Now().UTC().Truncate(time.Second)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3004,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-current",
		packageName:      "Package S",
		packagePrice:     25000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: true,
		servicePrice:     25000,
		serviceCPU:       1,
		serviceRAMMB:     1024,
		serviceDiskGB:    10,
		expiresAt:        now.Add(10 * 24 * time.Hour),
	})

	pkgs := []model.Package{
		{ID: "pkg-same", Name: "Same", CPU: 1, RAMMB: 1024, DiskGB: 10, Price: 26000, DurationDays: 30, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pkg-up-1", Name: "Upgrade 1", CPU: 2, RAMMB: 1024, DiskGB: 10, Price: 35000, DurationDays: 30, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pkg-up-2", Name: "Upgrade 2", CPU: 2, RAMMB: 2048, DiskGB: 20, Price: 45000, DurationDays: 30, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pkg-down", Name: "Downgrade", CPU: 1, RAMMB: 512, DiskGB: 10, Price: 15000, DurationDays: 30, IsActive: true, CreatedAt: now, UpdatedAt: now},
	}
	for i := range pkgs {
		if err := db.Create(&pkgs[i]).Error; err != nil {
			t.Fatalf("failed to seed package %s: %v", pkgs[i].ID, err)
		}
	}

	result, err := svc.UpgradeOptions(context.Background(), UpgradeOptionsInput{
		TelegramID:  3004,
		ContainerID: "container-1",
	})
	if err != nil {
		t.Fatalf("UpgradeOptions returned error: %v", err)
	}

	if len(result.Packages) != 2 {
		t.Fatalf("expected 2 eligible upgrade packages, got %d", len(result.Packages))
	}
	if result.Packages[0].ID != "pkg-up-1" || result.Packages[1].ID != "pkg-up-2" {
		t.Fatalf("unexpected eligible packages: %#v", result.Packages)
	}
}

func TestTransferSubmitMovesOwnership(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)

	now := time.Now().UTC().Truncate(time.Second)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3005,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-1",
		packageName:      "Package S",
		packagePrice:     25000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: true,
		servicePrice:     25000,
		serviceCPU:       1,
		serviceRAMMB:     1024,
		serviceDiskGB:    10,
		expiresAt:        now.Add(10 * 24 * time.Hour),
	})
	seedTelegramUser(t, db, model.User{
		ID:          "user-2",
		TelegramID:  9009,
		DisplayName: "Target User",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, model.Wallet{
		ID:        "wallet-2",
		UserID:    "user-2",
		Balance:   0,
		CreatedAt: now,
		UpdatedAt: now,
	})

	result, err := svc.TransferSubmit(context.Background(), TransferSubmitInput{
		TelegramID:       3005,
		ContainerID:      "container-1",
		TargetTelegramID: 9009,
		Reason:           stringPtr("gift"),
	})
	if err != nil {
		t.Fatalf("TransferSubmit returned error: %v", err)
	}

	if result.ToUser.ID != "user-2" {
		t.Fatalf("expected transfer target user-2, got %s", result.ToUser.ID)
	}

	var service model.Service
	if err := db.First(&service, "id = ?", "service-1").Error; err != nil {
		t.Fatalf("failed to load service: %v", err)
	}
	if service.OwnerUserID != "user-2" {
		t.Fatalf("expected service owner to be user-2, got %s", service.OwnerUserID)
	}

	var transfer model.ServiceTransfer
	if err := db.First(&transfer, "service_id = ?", "service-1").Error; err != nil {
		t.Fatalf("expected transfer record: %v", err)
	}
}

func TestCancelPreviewCalculatesProratedRefund(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)

	now := time.Now().UTC().Truncate(time.Second)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3006,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-1",
		packageName:      "Package S",
		packagePrice:     30000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: true,
		servicePrice:     30000,
		serviceCPU:       1,
		serviceRAMMB:     1024,
		serviceDiskGB:    10,
		expiresAt:        now.Add(15 * 24 * time.Hour),
	})

	result, err := svc.CancelPreview(context.Background(), CancelPreviewInput{
		TelegramID:  3006,
		ContainerID: "container-1",
	})
	if err != nil {
		t.Fatalf("CancelPreview returned error: %v", err)
	}

	if result.RefundAmount <= 0 {
		t.Fatalf("expected refund amount > 0, got %d", result.RefundAmount)
	}
	if result.PackageName != "Package S" {
		t.Fatalf("expected package Package S, got %s", result.PackageName)
	}
}

type fakeActionExecutor struct {
	changeStateFn    func(instanceName string, action string) (string, error)
	changePasswordFn func(instanceName string, password string) (string, error)
	resetSSHFn       func(instanceName string) (string, string, error)
	reinstallFn      func(instanceName string, imageAlias string) (string, error)
	applyLimitsFn    func(instanceName string, cpu int, ramMB int, diskGB int) (string, error)
	deleteInstanceFn func(instanceName string) (string, error)
}

func (f fakeActionExecutor) ChangeState(instanceName string, action string) (string, error) {
	if f.changeStateFn == nil {
		return "", errors.New("not implemented")
	}

	return f.changeStateFn(instanceName, action)
}

func (f fakeActionExecutor) ChangePassword(instanceName string, password string) (string, error) {
	if f.changePasswordFn == nil {
		return "", errors.New("not implemented")
	}

	return f.changePasswordFn(instanceName, password)
}

func (f fakeActionExecutor) ResetSSH(instanceName string) (string, string, error) {
	if f.resetSSHFn == nil {
		return "", "", errors.New("not implemented")
	}

	return f.resetSSHFn(instanceName)
}

func (f fakeActionExecutor) Reinstall(instanceName string, imageAlias string) (string, error) {
	if f.reinstallFn == nil {
		return "", errors.New("not implemented")
	}

	return f.reinstallFn(instanceName, imageAlias)
}

func (f fakeActionExecutor) ApplyResourceLimits(instanceName string, cpu int, ramMB int, diskGB int) (string, error) {
	if f.applyLimitsFn == nil {
		return "", errors.New("not implemented")
	}

	return f.applyLimitsFn(instanceName, cpu, ramMB, diskGB)
}

func (f fakeActionExecutor) DeleteInstance(instanceName string) (string, error) {
	if f.deleteInstanceFn == nil {
		return "", errors.New("not implemented")
	}

	return f.deleteInstanceFn(instanceName)
}

type actionSeedInput struct {
	telegramID        int64
	userID            string
	walletID          string
	containerID       string
	serviceID         string
	orderID           string
	packageID         string
	packageName       string
	packagePrice      int64
	packageDuration   int
	serviceStatus     string
	instanceStatus    string
	currentPackageOn  bool
	servicePrice      int64
	serviceCPU        int
	serviceRAMMB      int
	serviceDiskGB     int
	expiresAt         time.Time
	walletBalance     int64
	includeSSHMapping bool
}

func newActionServiceTestHarness(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.Wallet{},
		&model.Package{},
		&model.Order{},
		&model.Payment{},
		&model.Node{},
		&model.Service{},
		&model.ServiceInstance{},
		&model.ServicePortMapping{},
		&model.ServiceDomain{},
		&model.ProvisioningJob{},
		&model.ServiceEvent{},
		&model.ServiceTransfer{},
		&model.WalletTransaction{},
	); err != nil {
		t.Fatalf("failed to migrate sqlite schema: %v", err)
	}

	repo := NewRepository(db)
	return NewService(repo, nil), db
}

func seedActionServiceData(t *testing.T, db *gorm.DB, input actionSeedInput) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	walletBalance := input.walletBalance
	if walletBalance == 0 {
		walletBalance = 100000
	}

	seedTelegramUser(t, db, model.User{
		ID:          input.userID,
		TelegramID:  input.telegramID,
		DisplayName: "Action User",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, model.Wallet{
		ID:        input.walletID,
		UserID:    input.userID,
		Balance:   walletBalance,
		CreatedAt: now,
		UpdatedAt: now,
	})

	pkg := model.Package{
		ID:           input.packageID,
		Name:         input.packageName,
		CPU:          input.serviceCPU,
		RAMMB:        input.serviceRAMMB,
		DiskGB:       input.serviceDiskGB,
		Price:        input.packagePrice,
		DurationDays: input.packageDuration,
		IsActive:     input.currentPackageOn,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	order := model.Order{
		ID:                   input.orderID,
		UserID:               input.userID,
		PackageID:            input.packageID,
		OrderType:            "purchase",
		Status:               "completed",
		PaymentStatus:        "paid",
		PackageNameSnapshot:  input.packageName,
		CPUSnapshot:          input.serviceCPU,
		RAMMBSnapshot:        input.serviceRAMMB,
		DiskGBSnapshot:       input.serviceDiskGB,
		PriceSnapshot:        input.servicePrice,
		DurationDaysSnapshot: input.packageDuration,
		TotalAmount:          input.servicePrice,
		CreatedAt:            now,
		UpdatedAt:            now,
		PaidAt:               &now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	node := model.Node{
		ID:          "node-1",
		Name:        "node-1",
		DisplayName: "Node One",
		PublicIP:    "192.0.2.1",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	service := model.Service{
		ID:                  input.serviceID,
		OrderID:             input.orderID,
		OwnerUserID:         input.userID,
		CurrentPackageID:    input.packageID,
		Status:              input.serviceStatus,
		BillingCycleDays:    input.packageDuration,
		PackageNameSnapshot: input.packageName,
		CPUSnapshot:         input.serviceCPU,
		RAMMBSnapshot:       input.serviceRAMMB,
		DiskGBSnapshot:      input.serviceDiskGB,
		PriceSnapshot:       input.servicePrice,
		StartedAt:           &now,
		ExpiresAt:           &input.expiresAt,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	internalIP := "10.0.0.2"
	mainPublicIP := "192.0.2.10"
	sshPort := 22001
	instance := model.ServiceInstance{
		ID:                input.containerID,
		ServiceID:         input.serviceID,
		NodeID:            "node-1",
		IncusInstanceName: "inst-one",
		ImageAlias:        "ubuntu/24.04",
		InternalIP:        &internalIP,
		MainPublicIP:      &mainPublicIP,
		SSHPort:           &sshPort,
		Status:            input.instanceStatus,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create service instance: %v", err)
	}

	if input.includeSSHMapping {
		mapping := model.ServicePortMapping{
			ID:          "mapping-ssh-1",
			ServiceID:   input.serviceID,
			MappingType: "ssh",
			PublicIP:    mainPublicIP,
			PublicPort:  sshPort,
			Protocol:    "tcp",
			TargetIP:    internalIP,
			TargetPort:  22,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := db.Create(&mapping).Error; err != nil {
			t.Fatalf("failed to create ssh mapping: %v", err)
		}
	}
}

func stringPtr(v string) *string {
	return &v
}
