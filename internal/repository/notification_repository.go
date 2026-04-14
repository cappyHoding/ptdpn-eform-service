package repository

import (
	"context"
	"fmt"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"gorm.io/gorm"
)

type NotificationRepository interface {
	Create(ctx context.Context, log *model.NotificationLog) error
	Update(ctx context.Context, log *model.NotificationLog) error
}

type notificationRepository struct{ db *gorm.DB }

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(ctx context.Context, log *model.NotificationLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("create notification log: %w", err)
	}
	return nil
}

func (r *notificationRepository) Update(ctx context.Context, log *model.NotificationLog) error {
	if err := r.db.WithContext(ctx).Save(log).Error; err != nil {
		return fmt.Errorf("update notification log: %w", err)
	}
	return nil
}