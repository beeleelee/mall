package a2a

import (
	"context"
	"strings"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type SkillHandler interface {
	Handle(ctx context.Context, task *Task, message Message) error
}

type AgentService struct {
	tasks    TaskRepository
	pushCfgs PushNotificationConfigRepository
	skills   map[string]SkillHandler
	logger   kernel.Logger
	sf       *kernel.Snowflake
}

func NewAgentService(
	tasks TaskRepository,
	pushCfgs PushNotificationConfigRepository,
	logger kernel.Logger,
	sf *kernel.Snowflake,
) *AgentService {
	return &AgentService{
		tasks:    tasks,
		pushCfgs: pushCfgs,
		skills:   make(map[string]SkillHandler),
		logger:   logger,
		sf:       sf,
	}
}

func (s *AgentService) RegisterSkill(id string, handler SkillHandler) {
	s.skills[id] = handler
}

func (s *AgentService) SendMessage(ctx context.Context, userID kernel.ID, message Message, contextID string) (*Task, error) {
	skillID := s.detectSkill(message)
	id, err := s.sf.NextID()
	if err != nil {
		return nil, NewA2AError(ErrUnsupportedOperation, "failed to generate task id")
	}

	task := NewTask(kernel.ID(id), userID, skillID, contextID)
	task.AddMessage(message)

	if err := s.tasks.Save(ctx, task); err != nil {
		return nil, err
	}

	handler, ok := s.skills[skillID]
	if !ok {
		task.Transition(TaskStateFailed, "no handler for skill: "+skillID)
		s.tasks.Save(ctx, task)
		return task, NewA2AError(ErrSkillNotFound, "no handler for skill: "+skillID)
	}

	task.Transition(TaskStateWorking, "processing")
	s.tasks.Save(ctx, task)

	if err := handler.Handle(ctx, task, message); err != nil {
		task.Transition(TaskStateFailed, err.Error())
		s.tasks.Save(ctx, task)
		return task, nil
	}

	if !task.Status.State.IsTerminal() && task.Status.State != TaskStateInputRequired {
		task.Transition(TaskStateCompleted, "task completed successfully")
	}
	s.tasks.Save(ctx, task)
	return task, nil
}

func (s *AgentService) SendMessageToTask(ctx context.Context, taskID kernel.ID, message Message) (*Task, error) {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, NewA2AError(ErrTaskNotFound, "task not found")
	}

	if task.Status.State.IsTerminal() {
		return nil, NewA2AError(ErrUnsupportedOperation, "task is in terminal state")
	}

	task.AddMessage(message)
	task.Transition(TaskStateWorking, "processing follow-up")
	s.tasks.Save(ctx, task)

	handler, ok := s.skills[task.SkillID]
	if ok {
		if err := handler.Handle(ctx, task, message); err != nil {
			task.Transition(TaskStateFailed, err.Error())
			s.tasks.Save(ctx, task)
			return task, nil
		}
	}

	if !task.Status.State.IsTerminal() && task.Status.State != TaskStateInputRequired {
		task.Transition(TaskStateCompleted, "follow-up completed")
	}
	s.tasks.Save(ctx, task)
	return task, nil
}

func (s *AgentService) GetTask(ctx context.Context, id kernel.ID) (*Task, error) {
	task, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, NewA2AError(ErrTaskNotFound, "task not found")
	}
	return task, nil
}

func (s *AgentService) ListTasks(ctx context.Context, userID kernel.ID, skillID string, states []TaskState, pageToken string, pageSize int) ([]*Task, string, error) {
	return s.tasks.List(ctx, userID, skillID, states, pageToken, pageSize)
}

func (s *AgentService) CancelTask(ctx context.Context, id kernel.ID) (*Task, error) {
	task, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, NewA2AError(ErrTaskNotFound, "task not found")
	}

	if !task.Status.State.IsCancelable() {
		return nil, NewA2AError(ErrTaskNotCancelable, "task is not cancelable")
	}

	task.Transition(TaskStateCanceled, "canceled by user")
	s.tasks.Save(ctx, task)
	return task, nil
}

func (s *AgentService) CreatePushNotificationConfig(ctx context.Context, taskID kernel.ID, url string, auth AuthInfo) (*PushNotificationConfig, error) {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, NewA2AError(ErrTaskNotFound, "task not found")
	}

	id, err := s.sf.NextID()
	if err != nil {
		return nil, NewA2AError(ErrUnsupportedOperation, "failed to generate config id")
	}

	config := NewPushNotificationConfig(kernel.ID(id), taskID, url, auth)
	if err := s.pushCfgs.Save(ctx, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (s *AgentService) GetPushNotificationConfig(ctx context.Context, id kernel.ID) (*PushNotificationConfig, error) {
	config, err := s.pushCfgs.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, NewA2AError(ErrTaskNotFound, "push notification config not found")
	}
	return config, nil
}

func (s *AgentService) ListPushNotificationConfigs(ctx context.Context, taskID kernel.ID) ([]*PushNotificationConfig, error) {
	return s.pushCfgs.FindByTaskID(ctx, taskID)
}

func (s *AgentService) DeletePushNotificationConfig(ctx context.Context, id kernel.ID) error {
	return s.pushCfgs.Delete(ctx, id)
}

func (s *AgentService) detectSkill(msg Message) string {
	text := ""
	for _, p := range msg.Parts {
		if p.Type == PartTypeText {
			text = p.Text
			break
		}
		if p.Type == PartTypeData {
			if m, ok := p.Data.(map[string]any); ok {
				if s, ok := m["skill"]; ok {
					if str, ok := s.(string); ok {
						return str
					}
				}
			}
		}
	}

	text = strings.ToLower(text)
	for id := range s.skills {
		if strings.Contains(text, id) {
			return id
		}
	}

	results := make([]string, 0, len(s.skills))
	for id := range s.skills {
		results = append(results, id)
	}
	if len(results) > 0 {
		return results[0]
	}
	return "unknown"
}

func (s *AgentService) AddArtifactToTask(ctx context.Context, taskID kernel.ID, name string, parts []Part) error {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return NewA2AError(ErrTaskNotFound, "task not found")
	}

	artifact := Artifact{
		ID:    name + "-" + time.Now().UTC().Format(time.RFC3339Nano),
		Name:  name,
		Parts: parts,
		Index: len(task.Artifacts),
	}
	task.AddArtifact(artifact)
	return s.tasks.Save(ctx, task)
}

func (s *AgentService) AppendMessage(ctx context.Context, taskID kernel.ID, msg Message) error {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return NewA2AError(ErrTaskNotFound, "task not found")
	}
	task.AddMessage(msg)
	return s.tasks.Save(ctx, task)
}
