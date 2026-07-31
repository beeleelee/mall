package notification

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type NotificationRepository interface {
	Save(ctx context.Context, n *Notification) error
	FindByID(ctx context.Context, id kernel.ID) (*Notification, error)
	FindByUserID(ctx context.Context, userID kernel.ID, limit int) ([]*Notification, error)
	MarkRead(ctx context.Context, id kernel.ID, userID kernel.ID) error
	MarkAllRead(ctx context.Context, userID kernel.ID) error
	UnreadCount(ctx context.Context, userID kernel.ID) (int, error)
}

type NotificationPreferenceRepository interface {
	Get(ctx context.Context, userID kernel.ID) (*NotificationPreferences, error)
	Upsert(ctx context.Context, prefs *NotificationPreferences) error
}
