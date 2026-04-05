package support

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) CreateFromTelegram(ctx context.Context, input CreateFromTelegramInput) (*TicketDetail, error) {
	if input.TelegramID <= 0 || normalizeSubject(input.Subject) == "" || normalizeMessage(input.Message) == "" {
		return nil, ErrInvalidTicketRequest
	}

	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}
		return nil, err
	}

	now := time.Now().UTC()
	ticket := &model.SupportTicket{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Status:    "open",
		Subject:   normalizeSubject(input.Subject),
		CreatedAt: now,
		UpdatedAt: now,
	}
	message := &model.SupportTicketMessage{
		ID:         uuid.NewString(),
		TicketID:   ticket.ID,
		SenderType: "user",
		SenderID:   &user.ID,
		Message:    normalizeMessage(input.Message),
		CreatedAt:  now,
	}
	if err := s.repo.CreateTicket(ctx, ticket, message); err != nil {
		return nil, err
	}

	return &TicketDetail{
		Ticket:   toTicketSummary(ticket, user, &message.Message),
		Messages: []TicketMessage{toTicketMessage(message)},
	}, nil
}

func (s *Service) ListForTelegram(ctx context.Context, input ListForTelegramInput) ([]TicketSummary, error) {
	if input.TelegramID <= 0 || !validStatus(input.Status) {
		return nil, ErrInvalidTicketRequest
	}

	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}
		return nil, err
	}

	tickets, err := s.repo.ListTicketsForUser(ctx, user.ID, input.Status)
	if err != nil {
		return nil, err
	}

	result := make([]TicketSummary, 0, len(tickets))
	for i := range tickets {
		messages, err := s.repo.FindMessagesByTicketID(ctx, tickets[i].ID)
		if err != nil {
			return nil, err
		}
		var lastMessage *string
		if len(messages) > 0 {
			lastMessage = &messages[len(messages)-1].Message
		}
		result = append(result, toTicketSummary(&tickets[i], user, lastMessage))
	}
	return result, nil
}

func (s *Service) GetForTelegram(ctx context.Context, input GetForTelegramInput) (*TicketDetail, error) {
	if input.TelegramID <= 0 || input.TicketID == "" {
		return nil, ErrInvalidTicketRequest
	}
	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}
		return nil, err
	}
	ticket, err := s.repo.FindTicketForUser(ctx, user.ID, input.TicketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return s.buildDetail(ctx, ticket)
}

func (s *Service) ReplyFromTelegram(ctx context.Context, input ReplyFromTelegramInput) (*TicketDetail, error) {
	if input.TelegramID <= 0 || input.TicketID == "" || normalizeMessage(input.Message) == "" {
		return nil, ErrInvalidTicketRequest
	}
	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}
		return nil, err
	}
	ticket, err := s.repo.FindTicketForUser(ctx, user.ID, input.TicketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.Status == "closed" {
		return nil, ErrTicketClosed
	}

	now := time.Now().UTC()
	message := &model.SupportTicketMessage{
		ID:         uuid.NewString(),
		TicketID:   ticket.ID,
		SenderType: "user",
		SenderID:   &user.ID,
		Message:    normalizeMessage(input.Message),
		CreatedAt:  now,
	}
	if err := s.repo.AddMessage(ctx, ticket.ID, message, map[string]any{
		"updated_at": now,
	}); err != nil {
		return nil, err
	}

	ticket.UpdatedAt = now
	return s.buildDetail(ctx, ticket)
}

func (s *Service) ListForAdmin(ctx context.Context, input ListForAdminInput) (*ListForAdminResult, error) {
	if input.Page <= 0 || input.Limit <= 0 || !validStatus(input.Status) {
		return nil, ErrInvalidTicketRequest
	}
	tickets, total, err := s.repo.ListTicketsForAdmin(ctx, input)
	if err != nil {
		return nil, err
	}
	items := make([]TicketSummary, 0, len(tickets))
	for i := range tickets {
		messages, err := s.repo.FindMessagesByTicketID(ctx, tickets[i].ID)
		if err != nil {
			return nil, err
		}
		var lastMessage *string
		if len(messages) > 0 {
			lastMessage = &messages[len(messages)-1].Message
		}
		items = append(items, toTicketSummary(&tickets[i], tickets[i].User, lastMessage))
	}
	return &ListForAdminResult{
		Items:      items,
		Page:       input.Page,
		Limit:      input.Limit,
		TotalItems: total,
		TotalPages: int(math.Ceil(float64(total) / float64(input.Limit))),
		Status:     normalizeStatus(input.Status),
		Search:     input.Search,
	}, nil
}

func (s *Service) GetForAdmin(ctx context.Context, ticketID string) (*TicketDetail, error) {
	if ticketID == "" {
		return nil, ErrInvalidTicketRequest
	}
	ticket, err := s.repo.FindTicketByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return s.buildDetail(ctx, ticket)
}

func (s *Service) ReplyFromAdmin(ctx context.Context, input ReplyFromAdminInput) (*TicketDetail, error) {
	if input.TicketID == "" || input.Admin.ID == "" || normalizeMessage(input.Message) == "" {
		return nil, ErrInvalidTicketRequest
	}
	ticket, err := s.repo.FindTicketByID(ctx, input.TicketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.Status == "closed" {
		return nil, ErrTicketClosed
	}

	now := time.Now().UTC()
	nextStatus := ticket.Status
	updates := map[string]any{"updated_at": now}
	if ticket.Status == "open" {
		nextStatus = "in_progress"
		updates["status"] = nextStatus
	}
	message := &model.SupportTicketMessage{
		ID:         uuid.NewString(),
		TicketID:   ticket.ID,
		SenderType: "admin",
		SenderID:   &input.Admin.ID,
		Message:    normalizeMessage(input.Message),
		CreatedAt:  now,
	}
	if err := s.repo.AddMessage(ctx, ticket.ID, message, updates); err != nil {
		return nil, err
	}
	ticket.Status = nextStatus
	ticket.UpdatedAt = now
	return s.buildDetail(ctx, ticket)
}

func (s *Service) UpdateStatus(ctx context.Context, input UpdateStatusInput) (*model.SupportTicket, error) {
	status := normalizeStatus(input.Status)
	if input.TicketID == "" || input.Admin.ID == "" || !validStatus(status) || status == "" {
		return nil, ErrInvalidTicketRequest
	}
	ticket, err := s.repo.FindTicketByID(ctx, input.TicketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}

	now := time.Now().UTC()
	updates := map[string]any{
		"status":     status,
		"updated_at": now,
	}
	if status == "closed" {
		updates["closed_at"] = now
		ticket.ClosedAt = &now
	} else {
		updates["closed_at"] = nil
		ticket.ClosedAt = nil
	}
	if err := s.repo.UpdateTicketStatus(ctx, ticket.ID, updates); err != nil {
		return nil, err
	}
	ticket.Status = status
	ticket.UpdatedAt = now
	return ticket, nil
}

func (s *Service) buildDetail(ctx context.Context, ticket *model.SupportTicket) (*TicketDetail, error) {
	messages, err := s.repo.FindMessagesByTicketID(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}
	var lastMessage *string
	if len(messages) > 0 {
		lastMessage = &messages[len(messages)-1].Message
	}
	result := &TicketDetail{
		Ticket:   toTicketSummary(ticket, ticket.User, lastMessage),
		Messages: make([]TicketMessage, 0, len(messages)),
	}
	for i := range messages {
		result.Messages = append(result.Messages, toTicketMessage(&messages[i]))
	}
	return result, nil
}

func toTicketSummary(ticket *model.SupportTicket, user *model.User, lastMessage *string) TicketSummary {
	return TicketSummary{
		ID:          ticket.ID,
		UserID:      ticket.UserID,
		Status:      ticket.Status,
		Subject:     ticket.Subject,
		CreatedAt:   ticket.CreatedAt,
		UpdatedAt:   ticket.UpdatedAt,
		ClosedAt:    ticket.ClosedAt,
		User:        toTicketUser(user),
		LastMessage: lastMessage,
	}
}

func toTicketUser(user *model.User) TicketUser {
	if user == nil {
		return TicketUser{}
	}
	return TicketUser{
		ID:               user.ID,
		TelegramID:       user.TelegramID,
		TelegramUsername: user.TelegramUsername,
		DisplayName:      user.DisplayName,
	}
}

func toTicketMessage(message *model.SupportTicketMessage) TicketMessage {
	return TicketMessage{
		ID:         message.ID,
		TicketID:   message.TicketID,
		SenderType: message.SenderType,
		SenderID:   message.SenderID,
		Message:    message.Message,
		CreatedAt:  message.CreatedAt,
	}
}
