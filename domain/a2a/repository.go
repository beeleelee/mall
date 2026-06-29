package a2a

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type TaskRepository interface {
	Save(ctx context.Context, task *Task) error
	FindByID(ctx context.Context, id kernel.ID) (*Task, error)
	FindByContextID(ctx context.Context, contextID string, userID kernel.ID) ([]*Task, error)
	List(ctx context.Context, userID kernel.ID, skillID string, states []TaskState, pageToken string, pageSize int) ([]*Task, string, error)
	Delete(ctx context.Context, id kernel.ID) error
}

type PushNotificationConfigRepository interface {
	Save(ctx context.Context, config *PushNotificationConfig) error
	FindByID(ctx context.Context, id kernel.ID) (*PushNotificationConfig, error)
	FindByTaskID(ctx context.Context, taskID kernel.ID) ([]*PushNotificationConfig, error)
	Delete(ctx context.Context, id kernel.ID) error
}
