package a2a

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type TaskState string

const (
	TaskStateSubmitted     TaskState = "submitted"
	TaskStateWorking       TaskState = "working"
	TaskStateInputRequired TaskState = "input-required"
	TaskStateCompleted     TaskState = "completed"
	TaskStateFailed        TaskState = "failed"
	TaskStateCanceled      TaskState = "canceled"
	TaskStateRejected      TaskState = "rejected"
)

func (s TaskState) IsTerminal() bool {
	switch s {
	case TaskStateCompleted, TaskStateFailed, TaskStateCanceled, TaskStateRejected:
		return true
	default:
		return false
	}
}

func (s TaskState) IsCancelable() bool {
	switch s {
	case TaskStateSubmitted, TaskStateWorking, TaskStateInputRequired:
		return true
	default:
		return false
	}
}

type TaskStatus struct {
	State       TaskState  `json:"state"`
	Message     string     `json:"message,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type Task struct {
	kernel.AggregateRoot
	Status    TaskStatus     `json:"status"`
	History   []Message      `json:"history,omitempty"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	ContextID string         `json:"contextId,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	UserID    kernel.ID      `json:"userId,omitempty"`
	SkillID   string         `json:"skillId,omitempty"`
}

func NewTask(id kernel.ID, userID kernel.ID, skillID string, contextID string) *Task {
	now := time.Now().UTC()
	return &Task{
		AggregateRoot: kernel.NewAggregateRoot(id),
		Status: TaskStatus{
			State:     TaskStateSubmitted,
			UpdatedAt: now,
		},
		ContextID: contextID,
		UserID:    userID,
		SkillID:   skillID,
	}
}

func (t *Task) Transition(state TaskState, message string) error {
	if t.Status.State.IsTerminal() {
		return NewA2AError(ErrUnsupportedOperation, "task is already in terminal state: "+string(t.Status.State))
	}
	if state == TaskStateCompleted || state == TaskStateFailed || state == TaskStateCanceled || state == TaskStateRejected {
		now := time.Now().UTC()
		t.Status.CompletedAt = &now
	}
	now := time.Now().UTC()
	t.Status = TaskStatus{
		State:       state,
		Message:     message,
		UpdatedAt:   now,
		CompletedAt: t.Status.CompletedAt,
	}
	t.AddEvent(TaskStatusChangedEvent{
		TaskID:   t.ID,
		OldState: t.Status.State,
		NewState: state,
		UserID:   t.UserID,
	})
	return nil
}

func (t *Task) AddMessage(msg Message) {
	t.History = append(t.History, msg)
}

func (t *Task) AddArtifact(artifact Artifact) {
	t.Artifacts = append(t.Artifacts, artifact)
	t.AddEvent(TaskArtifactCreatedEvent{
		TaskID:     t.ID,
		ArtifactID: artifact.ID,
		UserID:     t.UserID,
	})
}
