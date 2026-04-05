package auth

import (
	"testing"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoginAndLogoutWriteActivityLogs(t *testing.T) {
	t.Parallel()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.AdminUser{}, &model.AdminSession{}, &model.ActivityLog{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	passwordHash, err := HashPassword("AdminPass123!")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	admin := model.AdminUser{
		ID:           "admin-1",
		Email:        "admin@example.com",
		PasswordHash: passwordHash,
		Role:         "admin",
		Status:       "active",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}

	svc := NewService(db, config.AuthConfig{SessionTTL: 24 * time.Hour})
	result, err := svc.Login(LoginInput{
		Email:     "admin@example.com",
		Password:  "AdminPass123!",
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if err := svc.Logout(result.Token); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}

	var count int64
	if err := db.Model(&model.ActivityLog{}).Where("target_type = ?", "admin").Count(&count).Error; err != nil {
		t.Fatalf("failed counting auth activity logs: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 auth activity logs, got %d", count)
	}
}
