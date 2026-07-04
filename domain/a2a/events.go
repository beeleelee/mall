package a2a

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type TaskStatusChangedEvent struct {
	TaskID   kernel.ID
	OldState TaskState
	NewState TaskState
	UserID   kernel.ID
}

func (e TaskStatusChangedEvent) EventName() string      { return "a2a.task.status_changed" }
func (e TaskStatusChangedEvent) OccurredAt() time.Time  { return time.Now() }
func (e TaskStatusChangedEvent) AggregateID() kernel.ID { return e.TaskID }

type TaskArtifactCreatedEvent struct {
	TaskID     kernel.ID
	ArtifactID string
	UserID     kernel.ID
}

func (e TaskArtifactCreatedEvent) EventName() string      { return "a2a.task.artifact_created" }
func (e TaskArtifactCreatedEvent) OccurredAt() time.Time  { return time.Now() }
func (e TaskArtifactCreatedEvent) AggregateID() kernel.ID { return e.TaskID }
