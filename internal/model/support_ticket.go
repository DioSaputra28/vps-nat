package model

import "time"

type SupportTicket struct {
	ID        string     `gorm:"column:id;primaryKey"`
	UserID    string     `gorm:"column:user_id"`
	Status    string     `gorm:"column:status"`
	Subject   string     `gorm:"column:subject"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
	ClosedAt  *time.Time `gorm:"column:closed_at"`

	User     *User                  `gorm:"foreignKey:UserID;references:ID"`
	Messages []SupportTicketMessage `gorm:"foreignKey:TicketID;references:ID"`
}

func (SupportTicket) TableName() string {
	return "support_tickets"
}
