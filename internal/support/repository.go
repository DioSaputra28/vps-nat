package support

import (
	"context"
	"strings"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindUserByTelegramID(ctx context.Context, telegramID int64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "telegram_id = ?", telegramID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateTicket(ctx context.Context, ticket *model.SupportTicket, message *model.SupportTicketMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		return tx.Create(message).Error
	})
}

func (r *Repository) FindTicketForUser(ctx context.Context, userID string, ticketID string) (*model.SupportTicket, error) {
	var ticket model.SupportTicket
	if err := r.db.WithContext(ctx).
		Preload("User").
		First(&ticket, "id = ? AND user_id = ?", ticketID, userID).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *Repository) FindTicketByID(ctx context.Context, ticketID string) (*model.SupportTicket, error) {
	var ticket model.SupportTicket
	if err := r.db.WithContext(ctx).
		Preload("User").
		First(&ticket, "id = ?", ticketID).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *Repository) FindMessagesByTicketID(ctx context.Context, ticketID string) ([]model.SupportTicketMessage, error) {
	var messages []model.SupportTicketMessage
	if err := r.db.WithContext(ctx).
		Order("created_at ASC").
		Find(&messages, "ticket_id = ?", ticketID).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *Repository) ListTicketsForUser(ctx context.Context, userID string, status string) ([]model.SupportTicket, error) {
	query := r.db.WithContext(ctx).
		Model(&model.SupportTicket{}).
		Preload("User").
		Where("user_id = ?", userID).
		Order("updated_at DESC")
	if normalized := normalizeStatus(status); normalized != "" {
		query = query.Where("status = ?", normalized)
	}

	var tickets []model.SupportTicket
	if err := query.Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *Repository) ListTicketsForAdmin(ctx context.Context, input ListForAdminInput) ([]model.SupportTicket, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.SupportTicket{}).
		Preload("User")

	if normalized := normalizeStatus(input.Status); normalized != "" {
		query = query.Where("support_tickets.status = ?", normalized)
	}
	if search := strings.TrimSpace(input.Search); search != "" {
		query = query.Joins("JOIN users ON users.id = support_tickets.user_id").
			Where("support_tickets.subject ILIKE ? OR users.display_name ILIKE ? OR COALESCE(users.telegram_username, '') ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tickets []model.SupportTicket
	if err := query.
		Order("support_tickets.updated_at DESC").
		Limit(input.Limit).
		Offset((input.Page - 1) * input.Limit).
		Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

func (r *Repository) AddMessage(ctx context.Context, ticketID string, message *model.SupportTicketMessage, updates map[string]any) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&model.SupportTicket{}).
				Where("id = ?", ticketID).
				Updates(updates).Error; err != nil {
				return err
			}
		}
		return tx.Create(message).Error
	})
}

func (r *Repository) UpdateTicketStatus(ctx context.Context, ticketID string, updates map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&model.SupportTicket{}).
		Where("id = ?", ticketID).
		Updates(updates).Error
}
