package activitylog

import (
	"context"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Entry struct {
	ActorType  string
	ActorID    *string
	Action     string
	TargetType string
	TargetID   *string
	Metadata   map[string]any
	CreatedAt  time.Time
}

func Write(ctx context.Context, db *gorm.DB, entry Entry) error {
	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	row := model.ActivityLog{
		ID:         uuid.NewString(),
		ActorType:  entry.ActorType,
		ActorID:    entry.ActorID,
		Action:     entry.Action,
		TargetType: entry.TargetType,
		TargetID:   entry.TargetID,
		Metadata:   entry.Metadata,
		CreatedAt:  createdAt,
	}

	return db.WithContext(ctx).Create(&row).Error
}
