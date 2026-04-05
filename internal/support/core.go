package support

import (
	"errors"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
)

var (
	ErrInvalidTicketRequest = errors.New("invalid ticket request")
	ErrTicketNotFound       = errors.New("ticket not found")
	ErrTicketClosed         = errors.New("ticket is closed")
	ErrInvalidStatus        = errors.New("invalid ticket status")
	ErrTelegramUserNotFound = errors.New("telegram user not found")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type TicketSummary struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Status      string     `json:"status"`
	Subject     string     `json:"subject"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	User        TicketUser `json:"user"`
	LastMessage *string    `json:"last_message,omitempty"`
}

type TicketUser struct {
	ID               string  `json:"id"`
	TelegramID       int64   `json:"telegram_id"`
	TelegramUsername *string `json:"telegram_username,omitempty"`
	DisplayName      string  `json:"display_name"`
}

type TicketMessage struct {
	ID         string    `json:"id"`
	TicketID   string    `json:"ticket_id"`
	SenderType string    `json:"sender_type"`
	SenderID   *string   `json:"sender_id,omitempty"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}

type TicketDetail struct {
	Ticket   TicketSummary   `json:"ticket"`
	Messages []TicketMessage `json:"messages"`
}

type CreateFromTelegramInput struct {
	TelegramID int64
	Subject    string
	Message    string
}

type ListForTelegramInput struct {
	TelegramID int64
	Status     string
}

type GetForTelegramInput struct {
	TelegramID int64
	TicketID   string
}

type ReplyFromTelegramInput struct {
	TelegramID int64
	TicketID   string
	Message    string
}

type ListForAdminInput struct {
	Page   int
	Limit  int
	Status string
	Search string
}

type ListForAdminResult struct {
	Items      []TicketSummary `json:"items"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
	TotalItems int64           `json:"total_items"`
	TotalPages int             `json:"total_pages"`
	Status     string          `json:"status"`
	Search     string          `json:"search"`
}

type ReplyFromAdminInput struct {
	TicketID string
	Admin    model.AdminUser
	Message  string
}

type UpdateStatusInput struct {
	TicketID string
	Admin    model.AdminUser
	Status   string
}

func normalizeMessage(value string) string {
	return strings.TrimSpace(value)
}

func normalizeSubject(value string) string {
	return strings.TrimSpace(value)
}

func normalizeStatus(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validStatus(value string) bool {
	switch normalizeStatus(value) {
	case "", "open", "in_progress", "closed":
		return true
	default:
		return false
	}
}
