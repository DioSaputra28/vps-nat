package support

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateTicketCreatesOpenTicketWithInitialUserMessage(t *testing.T) {
	t.Parallel()

	svc, db := newSupportServiceTestHarness(t)
	seedSupportUsers(t, db)

	result, err := svc.CreateFromTelegram(context.Background(), CreateFromTelegramInput{
		TelegramID: 900001,
		Subject:    "SSH timeout",
		Message:    "Tidak bisa login ke VPS",
	})
	if err != nil {
		t.Fatalf("CreateFromTelegram returned error: %v", err)
	}

	if result.Ticket.Status != "open" {
		t.Fatalf("expected open status, got %s", result.Ticket.Status)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 initial message, got %d", len(result.Messages))
	}
	if result.Messages[0].SenderType != "user" {
		t.Fatalf("expected first sender user, got %s", result.Messages[0].SenderType)
	}
}

func TestUserCanOnlySeeOwnTicketDetail(t *testing.T) {
	t.Parallel()

	svc, db := newSupportServiceTestHarness(t)
	seedSupportUsers(t, db)
	ticketID := seedSupportTicket(t, db, "ticket-1", "user-1", "open", "Need help", []supportSeedMessage{
		{ID: "msg-1", SenderType: "user", SenderID: stringPtr("user-1"), Message: "hello"},
	})

	_, err := svc.GetForTelegram(context.Background(), GetForTelegramInput{
		TelegramID: 900002,
		TicketID:   ticketID,
	})
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestReplyFromTelegramFailsWhenTicketClosed(t *testing.T) {
	t.Parallel()

	svc, db := newSupportServiceTestHarness(t)
	seedSupportUsers(t, db)
	ticketID := seedSupportTicket(t, db, "ticket-1", "user-1", "closed", "Need help", []supportSeedMessage{
		{ID: "msg-1", SenderType: "user", SenderID: stringPtr("user-1"), Message: "hello"},
	})

	_, err := svc.ReplyFromTelegram(context.Background(), ReplyFromTelegramInput{
		TelegramID: 900001,
		TicketID:   ticketID,
		Message:    "Ada update",
	})
	if !errors.Is(err, ErrTicketClosed) {
		t.Fatalf("expected ErrTicketClosed, got %v", err)
	}
}

func TestReplyFromAdminMovesTicketToInProgress(t *testing.T) {
	t.Parallel()

	svc, db := newSupportServiceTestHarness(t)
	seedSupportUsers(t, db)
	ticketID := seedSupportTicket(t, db, "ticket-1", "user-1", "open", "Need help", []supportSeedMessage{
		{ID: "msg-1", SenderType: "user", SenderID: stringPtr("user-1"), Message: "hello"},
	})

	admin := model.AdminUser{ID: "admin-1", Email: "admin@example.com", Role: "admin", Status: "active"}
	result, err := svc.ReplyFromAdmin(context.Background(), ReplyFromAdminInput{
		TicketID: ticketID,
		Admin:    admin,
		Message:  "Sedang kami cek",
	})
	if err != nil {
		t.Fatalf("ReplyFromAdmin returned error: %v", err)
	}

	if result.Ticket.Status != "in_progress" {
		t.Fatalf("expected in_progress status, got %s", result.Ticket.Status)
	}
	if result.Messages[len(result.Messages)-1].SenderType != "admin" {
		t.Fatalf("expected last sender admin, got %s", result.Messages[len(result.Messages)-1].SenderType)
	}

	var logs int64
	if err := db.Model(&model.ActivityLog{}).Where("action = ?", "support.ticket_replied").Count(&logs).Error; err != nil {
		t.Fatalf("failed counting activity logs: %v", err)
	}
	if logs != 1 {
		t.Fatalf("expected 1 support reply activity log, got %d", logs)
	}
}

func TestUpdateStatusClosesTicketAndSetsClosedAt(t *testing.T) {
	t.Parallel()

	svc, db := newSupportServiceTestHarness(t)
	seedSupportUsers(t, db)
	ticketID := seedSupportTicket(t, db, "ticket-1", "user-1", "in_progress", "Need help", []supportSeedMessage{
		{ID: "msg-1", SenderType: "user", SenderID: stringPtr("user-1"), Message: "hello"},
	})

	admin := model.AdminUser{ID: "admin-1", Email: "admin@example.com", Role: "admin", Status: "active"}
	result, err := svc.UpdateStatus(context.Background(), UpdateStatusInput{
		TicketID: ticketID,
		Admin:    admin,
		Status:   "closed",
	})
	if err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}

	if result.Status != "closed" {
		t.Fatalf("expected closed status, got %s", result.Status)
	}
	if result.ClosedAt == nil {
		t.Fatalf("expected closed_at to be set")
	}

	var logs int64
	if err := db.Model(&model.ActivityLog{}).Where("action = ?", "support.ticket_status_updated").Count(&logs).Error; err != nil {
		t.Fatalf("failed counting activity logs: %v", err)
	}
	if logs != 1 {
		t.Fatalf("expected 1 support status activity log, got %d", logs)
	}
}

func TestListForAdminReturnsTicketsWithUserData(t *testing.T) {
	t.Parallel()

	svc, db := newSupportServiceTestHarness(t)
	seedSupportUsers(t, db)
	seedSupportTicket(t, db, "ticket-1", "user-1", "open", "Need help", []supportSeedMessage{
		{ID: "msg-1", SenderType: "user", SenderID: stringPtr("user-1"), Message: "hello"},
	})

	result, err := svc.ListForAdmin(context.Background(), ListForAdminInput{
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListForAdmin returned error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(result.Items))
	}
	if result.Items[0].User.ID != "user-1" {
		t.Fatalf("expected ticket user user-1, got %s", result.Items[0].User.ID)
	}
	if result.Items[0].LastMessage == nil || *result.Items[0].LastMessage != "hello" {
		t.Fatalf("expected last message hello, got %#v", result.Items[0].LastMessage)
	}
}

func newSupportServiceTestHarness(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.AdminUser{},
		&model.SupportTicket{},
		&model.SupportTicketMessage{},
		&model.ActivityLog{},
	); err != nil {
		t.Fatalf("failed to migrate sqlite schema: %v", err)
	}

	repo := NewRepository(db)
	return NewService(repo), db
}

func seedSupportUsers(t *testing.T, db *gorm.DB) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	users := []model.User{
		{ID: "user-1", TelegramID: 900001, DisplayName: "User One", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "user-2", TelegramID: 900002, DisplayName: "User Two", Status: "active", CreatedAt: now, UpdatedAt: now},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("failed to seed user %s: %v", users[i].ID, err)
		}
	}
}

type supportSeedMessage struct {
	ID         string
	SenderType string
	SenderID   *string
	Message    string
}

func seedSupportTicket(t *testing.T, db *gorm.DB, ticketID string, userID string, status string, subject string, messages []supportSeedMessage) string {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	ticket := model.SupportTicket{
		ID:        ticketID,
		UserID:    userID,
		Status:    status,
		Subject:   subject,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if status == "closed" {
		ticket.ClosedAt = &now
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("failed to seed ticket: %v", err)
	}
	for _, msg := range messages {
		row := model.SupportTicketMessage{
			ID:         msg.ID,
			TicketID:   ticketID,
			SenderType: msg.SenderType,
			SenderID:   msg.SenderID,
			Message:    msg.Message,
			CreatedAt:  now,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("failed to seed ticket message: %v", err)
		}
	}
	return ticketID
}

func stringPtr(v string) *string {
	return &v
}
