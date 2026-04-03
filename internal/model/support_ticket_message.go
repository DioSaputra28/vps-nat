package model

import "time"

type SupportTicketMessage struct {
	ID         string    `gorm:"column:id;primaryKey"`
	TicketID   string    `gorm:"column:ticket_id"`
	SenderType string    `gorm:"column:sender_type"`
	SenderID   *string   `gorm:"column:sender_id"`
	Message    string    `gorm:"column:message"`
	CreatedAt  time.Time `gorm:"column:created_at"`

	Ticket *SupportTicket `gorm:"foreignKey:TicketID;references:ID"`
}

func (SupportTicketMessage) TableName() string {
	return "support_ticket_messages"
}
