package packages

import (
	"context"
	"testing"

	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPackageLifecycleWritesActivityLogs(t *testing.T) {
	t.Parallel()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.Package{}, &model.ActivityLog{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	svc := NewService(NewRepository(db))
	ctx := middleware.ContextWithAdmin(context.Background(), model.AdminUser{ID: "admin-1", Role: "super_admin"})

	pkg, err := svc.Create(ctx, CreateInput{Name: "Starter", CPU: 1, RAMMB: 1024, DiskGB: 10, Price: 10000, DurationDays: 30})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Update(ctx, pkg.ID, UpdateInput{Name: strPtr("Starter Plus")}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if _, err := svc.Delete(ctx, pkg.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	var count int64
	if err := db.Model(&model.ActivityLog{}).Where("target_type = ?", "package").Count(&count).Error; err != nil {
		t.Fatalf("failed to count activity logs: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 package activity logs, got %d", count)
	}
}

func strPtr(v string) *string {
	return &v
}
