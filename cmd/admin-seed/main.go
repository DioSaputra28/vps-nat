package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/auth"
	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/database"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func main() {
	email := flag.String("email", "", "admin email")
	password := flag.String("password", "", "admin password")
	role := flag.String("role", "super_admin", "admin role: super_admin or admin")
	flag.Parse()

	if strings.TrimSpace(*email) == "" || strings.TrimSpace(*password) == "" {
		log.Fatal("email and password are required")
	}

	if *role != "super_admin" && *role != "admin" {
		log.Fatal("role must be super_admin or admin")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.New(cfg.Database)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	if err := seedAdmin(db, *email, *password, *role); err != nil {
		log.Fatalf("seed admin: %v", err)
	}

	fmt.Printf("admin %s is ready\n", strings.TrimSpace(*email))
}

func seedAdmin(db *gorm.DB, email string, password string, role string) error {
	now := time.Now().UTC()
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	var admin model.AdminUser
	tx := db.Where("LOWER(email) = LOWER(?)", strings.TrimSpace(email)).First(&admin)
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return tx.Error
	}

	if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		admin = model.AdminUser{
			ID:           uuid.NewString(),
			Email:        strings.TrimSpace(email),
			PasswordHash: passwordHash,
			Role:         role,
			Status:       "active",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		return db.Create(&admin).Error
	}

	return db.Model(&admin).Updates(map[string]any{
		"password_hash": passwordHash,
		"role":          role,
		"status":        "active",
		"updated_at":    now,
	}).Error
}
