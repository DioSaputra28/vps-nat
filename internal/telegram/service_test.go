package telegram

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestServiceMyVPSReturnsOwnedInstances(t *testing.T) {
	t.Parallel()

	svc, db := newTelegramServiceTestHarness(t)

	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(72 * time.Hour)

	seedTelegramUser(t, db, model.User{
		ID:          "user-1",
		TelegramID:  1001,
		DisplayName: "User One",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, model.Wallet{
		ID:        "wallet-1",
		UserID:    "user-1",
		Balance:   12000,
		CreatedAt: now,
		UpdatedAt: now,
	})

	seedTelegramUser(t, db, model.User{
		ID:          "user-2",
		TelegramID:  1002,
		DisplayName: "User Two",
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

	seedServiceGraph(t, db,
		model.Node{
			ID:          "node-1",
			Name:        "node-1",
			DisplayName: "Node One",
			PublicIP:    "1.1.1.1",
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		model.Service{
			ID:                  "service-1",
			OrderID:             "order-1",
			OwnerUserID:         "user-1",
			CurrentPackageID:    "pkg-1",
			Status:              "active",
			BillingCycleDays:    30,
			PackageNameSnapshot: "Package S",
			CPUSnapshot:         1,
			RAMMBSnapshot:       1024,
			DiskGBSnapshot:      10,
			PriceSnapshot:       25000,
			ExpiresAt:           &expiresAt,
			CreatedAt:           now,
			UpdatedAt:           now,
		},
		model.ServiceInstance{
			ID:                "instance-1",
			ServiceID:         "service-1",
			NodeID:            "node-1",
			IncusInstanceName: "vpsnat-instance-1",
			ImageAlias:        "ubuntu/24.04",
			Status:            "created",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	)

	seedServiceGraph(t, db,
		model.Node{
			ID:          "node-2",
			Name:        "node-2",
			DisplayName: "Node Two",
			PublicIP:    "2.2.2.2",
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		model.Service{
			ID:                  "service-2",
			OrderID:             "order-2",
			OwnerUserID:         "user-2",
			CurrentPackageID:    "pkg-2",
			Status:              "active",
			BillingCycleDays:    30,
			PackageNameSnapshot: "Package M",
			CPUSnapshot:         2,
			RAMMBSnapshot:       2048,
			DiskGBSnapshot:      20,
			PriceSnapshot:       50000,
			ExpiresAt:           &expiresAt,
			CreatedAt:           now,
			UpdatedAt:           now,
		},
		model.ServiceInstance{
			ID:                "instance-2",
			ServiceID:         "service-2",
			NodeID:            "node-2",
			IncusInstanceName: "vpsnat-instance-2",
			ImageAlias:        "debian/12",
			Status:            "running",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	)

	result, err := svc.MyVPS(context.Background(), MyVPSInput{TelegramID: 1001})
	if err != nil {
		t.Fatalf("MyVPS returned error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}

	item := result.Items[0]
	if item.ContainerID != "instance-1" {
		t.Fatalf("expected container instance-1, got %s", item.ContainerID)
	}
	if item.ServiceID != "service-1" {
		t.Fatalf("expected service service-1, got %s", item.ServiceID)
	}
	if item.PackageName != "Package S" {
		t.Fatalf("expected package Package S, got %s", item.PackageName)
	}
	if item.ServiceStatus != "active" {
		t.Fatalf("expected service status active, got %s", item.ServiceStatus)
	}
	if item.Status != "created" {
		t.Fatalf("expected fallback DB status created, got %s", item.Status)
	}
	if item.RemainingDays == nil || *item.RemainingDays < 2 {
		t.Fatalf("expected remaining days to be populated, got %#v", item.RemainingDays)
	}
	if item.LiveAvailable {
		t.Fatalf("expected live availability false when incus is not configured")
	}
}

func TestServiceMyVPSDetailReturnsDBBackedDetailWithoutIncus(t *testing.T) {
	t.Parallel()

	svc, db := newTelegramServiceTestHarness(t)

	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(48 * time.Hour)
	sshPort := 22001
	osFamily := "ubuntu"
	internalIP := "10.10.0.2"
	publicIP := "103.10.10.10"

	seedTelegramUser(t, db, model.User{
		ID:          "user-1",
		TelegramID:  2001,
		DisplayName: "User One",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, model.Wallet{
		ID:        "wallet-1",
		UserID:    "user-1",
		Balance:   99000,
		CreatedAt: now,
		UpdatedAt: now,
	})

	seedServiceGraph(t, db,
		model.Node{
			ID:          "node-1",
			Name:        "node-1",
			DisplayName: "Node One",
			PublicIP:    "1.1.1.1",
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		model.Service{
			ID:                  "service-1",
			OrderID:             "order-1",
			OwnerUserID:         "user-1",
			CurrentPackageID:    "pkg-1",
			Status:              "active",
			BillingCycleDays:    30,
			PackageNameSnapshot: "Package XL",
			CPUSnapshot:         4,
			RAMMBSnapshot:       8192,
			DiskGBSnapshot:      80,
			PriceSnapshot:       120000,
			ExpiresAt:           &expiresAt,
			CreatedAt:           now,
			UpdatedAt:           now,
		},
		model.ServiceInstance{
			ID:                "instance-1",
			ServiceID:         "service-1",
			NodeID:            "node-1",
			IncusInstanceName: "vpsnat-instance-1",
			ImageAlias:        "ubuntu/24.04",
			OSFamily:          &osFamily,
			InternalIP:        &internalIP,
			MainPublicIP:      &publicIP,
			SSHPort:           &sshPort,
			Status:            "stopped",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	)

	result, err := svc.MyVPSDetail(context.Background(), MyVPSDetailInput{
		TelegramID:  2001,
		ContainerID: "instance-1",
	})
	if err != nil {
		t.Fatalf("MyVPSDetail returned error: %v", err)
	}

	if result.ContainerID != "instance-1" {
		t.Fatalf("expected container instance-1, got %s", result.ContainerID)
	}
	if result.PackageName != "Package XL" {
		t.Fatalf("expected package Package XL, got %s", result.PackageName)
	}
	if result.CPULimit != 4 || result.RAMMBLimit != 8192 || result.DiskGBLimit != 80 {
		t.Fatalf("unexpected limits: cpu=%d ram=%d disk=%d", result.CPULimit, result.RAMMBLimit, result.DiskGBLimit)
	}
	if result.Price != 120000 {
		t.Fatalf("expected price 120000, got %d", result.Price)
	}
	if result.Status != "stopped" {
		t.Fatalf("expected status stopped from DB fallback, got %s", result.Status)
	}
	if result.Live.Available {
		t.Fatalf("expected live availability false when incus is not configured")
	}
	if result.Live.Error == nil || *result.Live.Error == "" {
		t.Fatalf("expected live error message when incus is not configured")
	}
	if result.RemainingDays == nil || *result.RemainingDays < 1 {
		t.Fatalf("expected remaining days to be populated, got %#v", result.RemainingDays)
	}
}

func newTelegramServiceTestHarness(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.Wallet{},
		&model.Node{},
		&model.Service{},
		&model.ServiceInstance{},
		&model.ServicePortMapping{},
		&model.ServiceDomain{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	repo := NewRepository(db)
	return NewService(repo, nil), db
}

func seedTelegramUser(t *testing.T, db *gorm.DB, user model.User, wallet model.Wallet) {
	t.Helper()

	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}
}

func seedServiceGraph(t *testing.T, db *gorm.DB, node model.Node, service model.Service, instance model.ServiceInstance) {
	t.Helper()

	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create service instance: %v", err)
	}
}
