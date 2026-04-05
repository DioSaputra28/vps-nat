package containers

import (
	"context"
	"testing"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSaveActionResultWritesActivityLog(t *testing.T) {
	t.Parallel()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Package{}, &model.Node{}, &model.Service{}, &model.ServiceInstance{}, &model.ProvisioningJob{}, &model.ServiceEvent{}, &model.ActivityLog{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	user := model.User{ID: "user-1", TelegramID: 7001, DisplayName: "Owner", Status: "active", CreatedAt: now, UpdatedAt: now}
	pkg := model.Package{ID: "pkg-1", Name: "Package", CPU: 1, RAMMB: 1024, DiskGB: 10, Price: 10000, DurationDays: 30, IsActive: true, CreatedAt: now, UpdatedAt: now}
	node := model.Node{ID: "node-1", Name: "dio-node", DisplayName: "Dio Node", PublicIP: "82.25.44.113", Status: "active", CreatedAt: now, UpdatedAt: now}
	service := model.Service{ID: "service-1", OrderID: "order-1", OwnerUserID: user.ID, CurrentPackageID: pkg.ID, Status: "active", BillingCycleDays: 30, PackageNameSnapshot: "Package", CPUSnapshot: 1, RAMMBSnapshot: 1024, DiskGBSnapshot: 10, PriceSnapshot: 10000, CreatedAt: now, UpdatedAt: now}
	instance := &model.ServiceInstance{ID: "instance-1", ServiceID: service.ID, NodeID: node.ID, IncusInstanceName: "hermes", Status: "running", UpdatedAt: now, Service: &service}

	for _, row := range []any{&user, &pkg, &node, &service, instance} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("failed seeding %T: %v", row, err)
		}
	}

	repo := NewRepository(db)
	job := &model.ProvisioningJob{ID: "job-1", ServiceID: &service.ID, JobType: "start", Status: "success", RequestedByType: "admin", RequestedByID: strPtr("admin-1"), CreatedAt: now, UpdatedAt: now}
	event := &model.ServiceEvent{ID: "event-1", ServiceID: service.ID, EventType: "container_started", ActorType: "admin", ActorID: strPtr("admin-1"), Summary: "started", CreatedAt: now}

	if err := repo.SaveActionResult(context.Background(), instance, &service, job, event); err != nil {
		t.Fatalf("SaveActionResult returned error: %v", err)
	}

	var count int64
	if err := db.Model(&model.ActivityLog{}).Where("action = ?", "container.start").Count(&count).Error; err != nil {
		t.Fatalf("failed counting activity logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 container action activity log, got %d", count)
	}
}

func strPtr(v string) *string {
	return &v
}
