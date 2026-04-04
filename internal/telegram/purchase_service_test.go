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

func TestBuyVPSSubmitWalletCreatesServiceAndReturnsAccessPayload(t *testing.T) {
	t.Parallel()

	svc, db := newPurchaseServiceTestHarness(t)
	svc.purchaseProvisioner = fakePurchaseProvisioner{
		hostnameExistsFn: func(_ context.Context, hostname string) (bool, error) {
			return false, nil
		},
		provisionFn: func(_ context.Context, req PurchaseProvisionRequest) (*PurchaseProvisionResult, error) {
			if req.Hostname != "hermes" {
				t.Fatalf("expected hostname hermes, got %s", req.Hostname)
			}
			if len(req.PortMappings) != 7 {
				t.Fatalf("expected 7 port mappings, got %d", len(req.PortMappings))
			}

			return &PurchaseProvisionResult{
				OperationID: "op-provision-1",
				Hostname:    req.Hostname,
				PublicIP:    req.Node.PublicIP,
				PrivateIP:   "10.10.0.5",
				SSHUsername: "root",
				SSHPassword: "secret123",
				SSHPort:     req.PortMappings[0].PublicPort,
				PortMappings: []ProvisionedPortMapping{
					{MappingType: "ssh", PublicIP: req.Node.PublicIP, PublicPort: req.PortMappings[0].PublicPort, Protocol: "tcp", TargetIP: "10.10.0.5", TargetPort: 22},
					{MappingType: "custom", PublicIP: req.Node.PublicIP, PublicPort: req.PortMappings[1].PublicPort, Protocol: "tcp", TargetIP: "10.10.0.5", TargetPort: 3000},
				},
			}, nil
		},
	}

	seedPurchaseUser(t, db, 7001, "user-1", "wallet-1", 100000)
	seedPurchasePackage(t, db, "pkg-xl", "Package XL", 4, 8192, 80, 45000, 30, true)
	seedPurchaseNode(t, db, "node-1", "154.12.117.162")

	result, err := svc.BuyVPSSubmit(context.Background(), BuyVPSSubmitInput{
		TelegramID:    7001,
		PackageID:     "pkg-xl",
		ImageAlias:    "images:ubuntu/24.04",
		Hostname:      "hermes",
		PaymentMethod: "wallet",
	})
	if err != nil {
		t.Fatalf("BuyVPSSubmit returned error: %v", err)
	}

	if result.Status != "active" {
		t.Fatalf("expected status active, got %s", result.Status)
	}
	if result.Service == nil {
		t.Fatalf("expected service payload")
	}
	if result.Service.Hostname != "hermes" {
		t.Fatalf("expected hostname hermes, got %s", result.Service.Hostname)
	}
	if result.Service.SSHPassword != "secret123" {
		t.Fatalf("expected ssh password secret123, got %s", result.Service.SSHPassword)
	}

	var wallet model.Wallet
	if err := db.First(&wallet, "id = ?", "wallet-1").Error; err != nil {
		t.Fatalf("failed to load wallet: %v", err)
	}
	if wallet.Balance != 55000 {
		t.Fatalf("expected wallet balance 55000, got %d", wallet.Balance)
	}

	var service model.Service
	if err := db.First(&service, "id = ?", result.Service.ServiceID).Error; err != nil {
		t.Fatalf("failed to load service: %v", err)
	}
	if service.Status != "active" {
		t.Fatalf("expected service status active, got %s", service.Status)
	}

	var instance model.ServiceInstance
	if err := db.First(&instance, "service_id = ?", service.ID).Error; err != nil {
		t.Fatalf("failed to load service instance: %v", err)
	}
	if instance.IncusInstanceName != "hermes" {
		t.Fatalf("expected incus instance name hermes, got %s", instance.IncusInstanceName)
	}

	var count int64
	if err := db.Model(&model.ServicePortMapping{}).Where("service_id = ?", service.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed counting mappings: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 persisted mappings from fake result, got %d", count)
	}
}

func TestBuyVPSSubmitQrisCreatesPendingOrderAndPayment(t *testing.T) {
	t.Parallel()

	svc, db := newPurchaseServiceTestHarness(t)
	svc.paymentGateway = fakePurchasePaymentGateway{
		createQrisFn: func(_ context.Context, req PakasirCreateTransactionRequest) (*PakasirTransaction, error) {
			if req.Method != "qris" {
				t.Fatalf("expected qris method, got %s", req.Method)
			}

			expiredAt := time.Now().UTC().Add(30 * time.Minute)
			return &PakasirTransaction{
				Provider:     "pakasir",
				OrderID:      req.OrderID,
				Amount:       req.Amount,
				Fee:          1000,
				TotalPayment: req.Amount + 1000,
				QRString:     "QR-STRING",
				ExpiredAt:    expiredAt,
				Raw:          map[string]any{"payment_method": "qris"},
			}, nil
		},
	}
	svc.purchaseProvisioner = fakePurchaseProvisioner{
		hostnameExistsFn: func(context.Context, string) (bool, error) { return false, nil },
	}

	seedPurchaseUser(t, db, 7002, "user-2", "wallet-2", 5000)
	seedPurchasePackage(t, db, "pkg-m", "Package M", 2, 4096, 40, 20000, 30, true)
	seedPurchaseNode(t, db, "node-1", "154.12.117.162")

	result, err := svc.BuyVPSSubmit(context.Background(), BuyVPSSubmitInput{
		TelegramID:    7002,
		PackageID:     "pkg-m",
		ImageAlias:    "images:debian/12",
		Hostname:      "atlas",
		PaymentMethod: "qris",
	})
	if err != nil {
		t.Fatalf("BuyVPSSubmit returned error: %v", err)
	}

	if result.Status != "awaiting_payment" {
		t.Fatalf("expected awaiting_payment, got %s", result.Status)
	}
	if result.Payment == nil {
		t.Fatalf("expected payment payload")
	}
	if result.Payment.QRString != "QR-STRING" {
		t.Fatalf("expected QR string to be returned")
	}

	var order model.Order
	if err := db.First(&order, "id = ?", result.OrderID).Error; err != nil {
		t.Fatalf("failed to load order: %v", err)
	}
	if order.Status != "awaiting_payment" {
		t.Fatalf("expected order awaiting_payment, got %s", order.Status)
	}

	var payment model.Payment
	if err := db.First(&payment, "id = ?", result.PaymentID).Error; err != nil {
		t.Fatalf("failed to load payment: %v", err)
	}
	if payment.Status != "pending" {
		t.Fatalf("expected payment pending, got %s", payment.Status)
	}
}

func TestHandlePakasirWebhookVerifiedPaymentProvisionsOrder(t *testing.T) {
	t.Parallel()

	svc, db := newPurchaseServiceTestHarness(t)
	svc.paymentGateway = fakePurchasePaymentGateway{
		verifyQrisFn: func(_ context.Context, req PakasirVerifyTransactionRequest) (*PakasirVerifiedTransaction, error) {
			return &PakasirVerifiedTransaction{
				OrderID:       req.OrderID,
				Amount:        req.Amount,
				Status:        "completed",
				PaymentMethod: "qris",
				CompletedAt:   time.Now().UTC(),
			}, nil
		},
	}
	svc.purchaseProvisioner = fakePurchaseProvisioner{
		hostnameExistsFn: func(context.Context, string) (bool, error) { return false, nil },
		provisionFn: func(_ context.Context, req PurchaseProvisionRequest) (*PurchaseProvisionResult, error) {
			return &PurchaseProvisionResult{
				OperationID: "op-paid-1",
				Hostname:    req.Hostname,
				PublicIP:    req.Node.PublicIP,
				PrivateIP:   "10.10.0.7",
				SSHUsername: "root",
				SSHPassword: "qris-pass",
				SSHPort:     req.PortMappings[0].PublicPort,
				PortMappings: []ProvisionedPortMapping{
					{MappingType: "ssh", PublicIP: req.Node.PublicIP, PublicPort: req.PortMappings[0].PublicPort, Protocol: "tcp", TargetIP: "10.10.0.7", TargetPort: 22},
				},
			}, nil
		},
	}

	seedPurchaseUser(t, db, 7003, "user-3", "wallet-3", 0)
	seedPurchasePackage(t, db, "pkg-l", "Package L", 3, 6144, 60, 30000, 30, true)
	seedPurchaseNode(t, db, "node-1", "154.12.117.162")
	orderID, paymentID := seedPendingQrisOrder(t, db, pendingQrisSeedInput{
		userID:      "user-3",
		packageID:   "pkg-l",
		packageName: "Package L",
		amount:      30000,
		hostname:    "apollo",
		imageAlias:  "images:ubuntu/24.04",
	})

	err := svc.HandlePakasirWebhook(context.Background(), PakasirWebhookInput{
		Amount:        30000,
		OrderID:       orderID,
		Project:       "project-slug",
		Status:        "completed",
		PaymentMethod: "qris",
	})
	if err != nil {
		t.Fatalf("HandlePakasirWebhook returned error: %v", err)
	}

	var order model.Order
	if err := db.First(&order, "id = ?", orderID).Error; err != nil {
		t.Fatalf("failed to load order: %v", err)
	}
	if order.Status != "completed" || order.PaymentStatus != "paid" {
		t.Fatalf("unexpected order state: %s/%s", order.Status, order.PaymentStatus)
	}

	var payment model.Payment
	if err := db.First(&payment, "id = ?", paymentID).Error; err != nil {
		t.Fatalf("failed to load payment: %v", err)
	}
	if payment.Status != "paid" {
		t.Fatalf("expected payment paid, got %s", payment.Status)
	}

	var service model.Service
	if err := db.First(&service, "order_id = ?", orderID).Error; err != nil {
		t.Fatalf("expected service to be created: %v", err)
	}
	if service.Status != "active" {
		t.Fatalf("expected service active, got %s", service.Status)
	}
}

func TestBuyVPSStatusReturnsActiveAccessPayload(t *testing.T) {
	t.Parallel()

	svc, db := newPurchaseServiceTestHarness(t)
	seedPurchaseUser(t, db, 7004, "user-4", "wallet-4", 0)
	seedPurchasePackage(t, db, "pkg-s", "Package S", 1, 1024, 10, 25000, 30, true)
	orderID := seedActivePurchaseOrder(t, db, activePurchaseSeedInput{
		userID:      "user-4",
		packageID:   "pkg-s",
		packageName: "Package S",
		amount:      25000,
		hostname:    "zeus",
		imageAlias:  "images:debian/12",
		publicIP:    "154.12.117.162",
		privateIP:   "10.10.0.9",
		sshPort:     21001,
	})

	result, err := svc.BuyVPSStatus(context.Background(), BuyVPSStatusInput{
		TelegramID: 7004,
		OrderID:    orderID,
	})
	if err != nil {
		t.Fatalf("BuyVPSStatus returned error: %v", err)
	}

	if result.Status != "active" {
		t.Fatalf("expected status active, got %s", result.Status)
	}
	if result.Service == nil {
		t.Fatalf("expected service payload")
	}
	if result.Service.Hostname != "zeus" {
		t.Fatalf("expected hostname zeus, got %s", result.Service.Hostname)
	}
	if result.Service.SSHPort != 21001 {
		t.Fatalf("expected ssh port 21001, got %d", result.Service.SSHPort)
	}
}

type fakePurchaseProvisioner struct {
	hostnameExistsFn func(ctx context.Context, hostname string) (bool, error)
	provisionFn      func(ctx context.Context, req PurchaseProvisionRequest) (*PurchaseProvisionResult, error)
}

func (f fakePurchaseProvisioner) HostnameExists(ctx context.Context, hostname string) (bool, error) {
	if f.hostnameExistsFn == nil {
		return false, nil
	}
	return f.hostnameExistsFn(ctx, hostname)
}

func (f fakePurchaseProvisioner) Provision(ctx context.Context, req PurchaseProvisionRequest) (*PurchaseProvisionResult, error) {
	if f.provisionFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.provisionFn(ctx, req)
}

type fakePurchasePaymentGateway struct {
	createQrisFn func(ctx context.Context, req PakasirCreateTransactionRequest) (*PakasirTransaction, error)
	verifyQrisFn func(ctx context.Context, req PakasirVerifyTransactionRequest) (*PakasirVerifiedTransaction, error)
}

func (f fakePurchasePaymentGateway) CreateQris(ctx context.Context, req PakasirCreateTransactionRequest) (*PakasirTransaction, error) {
	if f.createQrisFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.createQrisFn(ctx, req)
}

func (f fakePurchasePaymentGateway) VerifyTransaction(ctx context.Context, req PakasirVerifyTransactionRequest) (*PakasirVerifiedTransaction, error) {
	if f.verifyQrisFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.verifyQrisFn(ctx, req)
}

type fakeAdminNotifier struct {
	notifyFn func(ctx context.Context, message string) error
}

func (f fakeAdminNotifier) NotifyProvisionFailure(ctx context.Context, message string) error {
	if f.notifyFn == nil {
		return nil
	}
	return f.notifyFn(ctx, message)
}

func newPurchaseServiceTestHarness(t *testing.T) (*Service, *gorm.DB) {
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
		&model.ProvisioningJob{},
		&model.ServiceEvent{},
		&model.WalletTransaction{},
	); err != nil {
		t.Fatalf("failed to migrate sqlite schema: %v", err)
	}

	repo := NewRepository(db)
	svc := NewService(repo, nil)
	svc.adminNotifier = fakeAdminNotifier{}
	return svc, db
}

func seedPurchaseUser(t *testing.T, db *gorm.DB, telegramID int64, userID string, walletID string, balance int64) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	user := model.User{
		ID:          userID,
		TelegramID:  telegramID,
		DisplayName: "Purchase User",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	wallet := model.Wallet{
		ID:        walletID,
		UserID:    userID,
		Balance:   balance,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}
}

func seedPurchasePackage(t *testing.T, db *gorm.DB, packageID string, name string, cpu, ramMB, diskGB int, price int64, durationDays int, active bool) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	pkg := model.Package{
		ID:           packageID,
		Name:         name,
		CPU:          cpu,
		RAMMB:        ramMB,
		DiskGB:       diskGB,
		Price:        price,
		DurationDays: durationDays,
		IsActive:     active,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}
}

func seedPurchaseNode(t *testing.T, db *gorm.DB, nodeID string, publicIP string) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	node := model.Node{
		ID:          nodeID,
		Name:        nodeID,
		DisplayName: nodeID,
		PublicIP:    publicIP,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
}

type pendingQrisSeedInput struct {
	userID      string
	packageID   string
	packageName string
	amount      int64
	hostname    string
	imageAlias  string
}

func seedPendingQrisOrder(t *testing.T, db *gorm.DB, input pendingQrisSeedInput) (string, string) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	order := model.Order{
		ID:                   "order-qris-1",
		UserID:               input.userID,
		PackageID:            input.packageID,
		OrderType:            "purchase",
		Status:               "awaiting_payment",
		PaymentStatus:        "pending",
		PaymentMethod:        stringPtr("qris"),
		SelectedImageAlias:   &input.imageAlias,
		PackageNameSnapshot:  input.packageName,
		CPUSnapshot:          3,
		RAMMBSnapshot:        6144,
		DiskGBSnapshot:       60,
		PriceSnapshot:        input.amount,
		DurationDaysSnapshot: 30,
		TotalAmount:          input.amount,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	payment := model.Payment{
		ID:          "payment-qris-1",
		OrderID:     &order.ID,
		Purpose:     "order",
		Method:      "qris",
		Provider:    stringPtr("pakasir"),
		ProviderRef: &order.ID,
		ProviderPayload: map[string]any{
			"hostname":    input.hostname,
			"image_alias": input.imageAlias,
		},
		Amount:    input.amount,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}
	return order.ID, payment.ID
}

type activePurchaseSeedInput struct {
	userID      string
	packageID   string
	packageName string
	amount      int64
	hostname    string
	imageAlias  string
	publicIP    string
	privateIP   string
	sshPort     int
}

func seedActivePurchaseOrder(t *testing.T, db *gorm.DB, input activePurchaseSeedInput) string {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(30 * 24 * time.Hour)
	order := model.Order{
		ID:                   "order-active-1",
		UserID:               input.userID,
		PackageID:            input.packageID,
		OrderType:            "purchase",
		Status:               "completed",
		PaymentStatus:        "paid",
		PaymentMethod:        stringPtr("wallet"),
		SelectedImageAlias:   &input.imageAlias,
		PackageNameSnapshot:  input.packageName,
		CPUSnapshot:          1,
		RAMMBSnapshot:        1024,
		DiskGBSnapshot:       10,
		PriceSnapshot:        input.amount,
		DurationDaysSnapshot: 30,
		TotalAmount:          input.amount,
		CreatedAt:            now,
		UpdatedAt:            now,
		PaidAt:               &now,
	}
	service := model.Service{
		ID:                  "service-active-1",
		OrderID:             order.ID,
		OwnerUserID:         input.userID,
		CurrentPackageID:    input.packageID,
		Status:              "active",
		BillingCycleDays:    30,
		PackageNameSnapshot: input.packageName,
		CPUSnapshot:         1,
		RAMMBSnapshot:       1024,
		DiskGBSnapshot:      10,
		PriceSnapshot:       input.amount,
		StartedAt:           &now,
		ExpiresAt:           &expiresAt,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	instance := model.ServiceInstance{
		ID:                "container-active-1",
		ServiceID:         service.ID,
		NodeID:            "node-status-1",
		IncusInstanceName: input.hostname,
		ImageAlias:        input.imageAlias,
		InternalIP:        &input.privateIP,
		MainPublicIP:      &input.publicIP,
		SSHPort:           &input.sshPort,
		Status:            "running",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	node := model.Node{
		ID:          "node-status-1",
		Name:        "node-status-1",
		DisplayName: "node-status-1",
		PublicIP:    input.publicIP,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	payment := model.Payment{
		ID:        "payment-active-1",
		OrderID:   &order.ID,
		Purpose:   "order",
		Method:    "wallet",
		Amount:    input.amount,
		Status:    "paid",
		PaidAt:    &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mapping := model.ServicePortMapping{
		ID:          "mapping-active-1",
		ServiceID:   service.ID,
		MappingType: "ssh",
		PublicIP:    input.publicIP,
		PublicPort:  input.sshPort,
		Protocol:    "tcp",
		TargetIP:    input.privateIP,
		TargetPort:  22,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	for _, value := range []any{&order, &payment, &node, &service, &instance, &mapping} {
		if err := db.Create(value).Error; err != nil {
			t.Fatalf("failed to seed active purchase data: %v", err)
		}
	}
	return order.ID
}
