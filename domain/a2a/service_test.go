package a2a

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeLogger struct{}

func (l *fakeLogger) Debug(_ context.Context, msg string, fields ...kernel.LogField) {}
func (l *fakeLogger) Info(_ context.Context, msg string, fields ...kernel.LogField)  {}
func (l *fakeLogger) Warn(_ context.Context, msg string, fields ...kernel.LogField)  {}
func (l *fakeLogger) Error(_ context.Context, msg string, err error, fields ...kernel.LogField) {}

type fakeTaskRepo struct {
	tasks map[kernel.ID]*Task
}

func newFakeTaskRepo() *fakeTaskRepo {
	return &fakeTaskRepo{tasks: make(map[kernel.ID]*Task)}
}

func (r *fakeTaskRepo) Save(_ context.Context, task *Task) error {
	r.tasks[task.ID] = task
	return nil
}

func (r *fakeTaskRepo) FindByID(_ context.Context, id kernel.ID) (*Task, error) {
	return r.tasks[id], nil
}

func (r *fakeTaskRepo) FindByContextID(_ context.Context, contextID string, userID kernel.ID) ([]*Task, error) {
	var res []*Task
	for _, t := range r.tasks {
		if t.ContextID == contextID && t.UserID == userID {
			res = append(res, t)
		}
	}
	return res, nil
}

func (r *fakeTaskRepo) List(_ context.Context, userID kernel.ID, skillID string, states []TaskState, pageToken string, pageSize int) ([]*Task, string, error) {
	var res []*Task
	for _, t := range r.tasks {
		if t.UserID != userID {
			continue
		}
		if skillID != "" && t.SkillID != skillID {
			continue
		}
		if len(states) > 0 {
			found := false
			for _, s := range states {
				if t.Status.State == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		res = append(res, t)
	}
	return res, "", nil
}

func (r *fakeTaskRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(r.tasks, id)
	return nil
}

type fakePushConfigRepo struct {
	configs map[kernel.ID]*PushNotificationConfig
}

func newFakePushConfigRepo() *fakePushConfigRepo {
	return &fakePushConfigRepo{configs: make(map[kernel.ID]*PushNotificationConfig)}
}

func (r *fakePushConfigRepo) Save(_ context.Context, cfg *PushNotificationConfig) error {
	r.configs[cfg.ID] = cfg
	return nil
}

func (r *fakePushConfigRepo) FindByID(_ context.Context, id kernel.ID) (*PushNotificationConfig, error) {
	return r.configs[id], nil
}

func (r *fakePushConfigRepo) FindByTaskID(_ context.Context, taskID kernel.ID) ([]*PushNotificationConfig, error) {
	var res []*PushNotificationConfig
	for _, c := range r.configs {
		if c.TaskID == taskID {
			res = append(res, c)
		}
	}
	return res, nil
}

func (r *fakePushConfigRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(r.configs, id)
	return nil
}

type echoSkillHandler struct {
	skillID string
}

func (h *echoSkillHandler) Handle(_ context.Context, task *Task, msg Message) error {
	task.AddArtifact(Artifact{
		ID:    "response-1",
		Name:  "response",
		Parts: []Part{TextPart("handled by skill: " + h.skillID)},
	})
	return nil
}

type multiTurnHandler struct{}

func (h *multiTurnHandler) Handle(_ context.Context, task *Task, msg Message) error {
	task.AddArtifact(Artifact{
		ID:    "response-1",
		Name:  "response",
		Parts: []Part{TextPart("need more info")},
	})
	task.Transition(TaskStateInputRequired, "please provide more details")
	return nil
}

func TestNewTask(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	task := NewTask(kernel.ID(id), 1, "catalog", "ctx-1")

	if task.Status.State != TaskStateSubmitted {
		t.Errorf("expected submitted, got %s", task.Status.State)
	}
	if task.SkillID != "catalog" {
		t.Errorf("expected catalog, got %s", task.SkillID)
	}
	if task.ContextID != "ctx-1" {
		t.Errorf("expected ctx-1, got %s", task.ContextID)
	}
}

func TestTaskTransition(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	task := NewTask(kernel.ID(id), 1, "catalog", "")

	if err := task.Transition(TaskStateWorking, "processing"); err != nil {
		t.Fatal(err)
	}
	if task.Status.State != TaskStateWorking {
		t.Errorf("expected working, got %s", task.Status.State)
	}

	if err := task.Transition(TaskStateCompleted, "done"); err != nil {
		t.Fatal(err)
	}
	if task.Status.State != TaskStateCompleted {
		t.Errorf("expected completed, got %s", task.Status.State)
	}

	if err := task.Transition(TaskStateWorking, "again"); err == nil {
		t.Error("expected error transitioning from terminal state")
	}
}

func TestTaskCancelable(t *testing.T) {
	tests := []struct {
		state    TaskState
		cancelable bool
	}{
		{TaskStateSubmitted, true},
		{TaskStateWorking, true},
		{TaskStateInputRequired, true},
		{TaskStateCompleted, false},
		{TaskStateFailed, false},
		{TaskStateCanceled, false},
		{TaskStateRejected, false},
	}

	for _, tt := range tests {
		if tt.state.IsCancelable() != tt.cancelable {
			t.Errorf("IsCancelable(%s) = %v, want %v", tt.state, tt.state.IsCancelable(), tt.cancelable)
		}
	}
}

func TestTerminalStates(t *testing.T) {
	tests := []struct {
		state    TaskState
		terminal bool
	}{
		{TaskStateSubmitted, false},
		{TaskStateWorking, false},
		{TaskStateInputRequired, false},
		{TaskStateCompleted, true},
		{TaskStateFailed, true},
		{TaskStateCanceled, true},
		{TaskStateRejected, true},
	}

	for _, tt := range tests {
		if tt.state.IsTerminal() != tt.terminal {
			t.Errorf("IsTerminal(%s) = %v, want %v", tt.state, tt.state.IsTerminal(), tt.terminal)
		}
	}
}

func TestAgentServiceSendMessage(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)
	svc.RegisterSkill("catalog", &echoSkillHandler{skillID: "catalog"})

	task, err := svc.SendMessage(context.Background(), 1, Message{
		Role:  RoleUser,
		Parts: []Part{TextPart("search for running shoes")},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	if task.Status.State != TaskStateCompleted {
		t.Errorf("expected completed, got %s", task.Status.State)
	}
	if len(task.Artifacts) == 0 {
		t.Fatal("expected at least one artifact")
	}
}

func TestAgentServiceGetTask(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)
	svc.RegisterSkill("catalog", &echoSkillHandler{skillID: "catalog"})

	task, err := svc.SendMessage(context.Background(), 1, Message{
		Role:  RoleUser,
		Parts: []Part{TextPart("catalog search for shoes")},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != task.ID {
		t.Errorf("expected task %d, got %d", task.ID, got.ID)
	}
}

func TestAgentServiceCancelTask(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)

	id, _ := sf.NextID()
	task := NewTask(kernel.ID(id), 1, "catalog", "")
	taskRepo.Save(context.Background(), task)

	canceled, err := svc.CancelTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canceled.Status.State != TaskStateCanceled {
		t.Errorf("expected canceled, got %s", canceled.Status.State)
	}
}

func TestAgentServiceListTasks(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)
	svc.RegisterSkill("catalog", &echoSkillHandler{skillID: "catalog"})
	svc.RegisterSkill("cart", &echoSkillHandler{skillID: "cart"})

	svc.SendMessage(context.Background(), 1, Message{Role: RoleUser, Parts: []Part{TextPart("catalog search")}}, "")
	svc.SendMessage(context.Background(), 1, Message{Role: RoleUser, Parts: []Part{TextPart("cart show items")}}, "")
	svc.SendMessage(context.Background(), 2, Message{Role: RoleUser, Parts: []Part{TextPart("catalog search")}}, "")

	tasks, _, err := svc.ListTasks(context.Background(), 1, "", nil, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for user 1, got %d", len(tasks))
	}

	cartTasks, _, _ := svc.ListTasks(context.Background(), 1, "cart", nil, "", 10)
	if len(cartTasks) != 1 {
		t.Errorf("expected 1 cart task, got %d", len(cartTasks))
	}
}

func TestAgentServiceUnknownSkill(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)

	_, err := svc.SendMessage(context.Background(), 1, Message{
		Role:  RoleUser,
		Parts: []Part{TextPart("do something random")},
	}, "")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	a2aErr, ok := err.(*A2AError)
	if !ok {
		t.Fatalf("expected A2AError, got %T", err)
	}
	if a2aErr.Code != ErrSkillNotFound.Error() {
		t.Errorf("expected skill not found, got %s", a2aErr.Code)
	}
}

func TestTaskAddMessage(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	task := NewTask(kernel.ID(id), 1, "catalog", "")

	msg := Message{Role: RoleUser, Parts: []Part{TextPart("hello")}}
	task.AddMessage(msg)
	task.AddMessage(Message{Role: RoleAgent, Parts: []Part{TextPart("world")}})

	if len(task.History) != 2 {
		t.Errorf("expected 2 messages, got %d", len(task.History))
	}
}

func TestSendMessageToTask(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)
	svc.RegisterSkill("multi", &multiTurnHandler{})

	task, _ := svc.SendMessage(context.Background(), 1, Message{
		Role: RoleUser, Parts: []Part{TextPart("multi help")},
	}, "")

	followUp, err := svc.SendMessageToTask(context.Background(), task.ID, Message{
		Role: RoleUser, Parts: []Part{TextPart("here are the details")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if followUp.Status.State != TaskStateInputRequired {
		t.Errorf("expected input-required, got %s", followUp.Status.State)
	}
	if len(followUp.History) != 2 {
		t.Errorf("expected 2 messages, got %d", len(followUp.History))
	}
}

func TestSendMessageToTerminalTask(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)
	svc.RegisterSkill("multi", &multiTurnHandler{})

	task, _ := svc.SendMessage(context.Background(), 1, Message{
		Role: RoleUser, Parts: []Part{TextPart("multi help")},
	}, "")

	svc.CancelTask(context.Background(), task.ID)

	_, err := svc.SendMessageToTask(context.Background(), task.ID, Message{
		Role: RoleUser, Parts: []Part{TextPart("try again")},
	})
	if err == nil {
		t.Fatal("expected error for terminal task")
	}
}

func TestPushNotificationConfig(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)
	svc.RegisterSkill("catalog", &echoSkillHandler{skillID: "catalog"})

	task, _ := svc.SendMessage(context.Background(), 1, Message{
		Role: RoleUser, Parts: []Part{TextPart("catalog search")},
	}, "")

	auth := AuthInfo{Scheme: "bearer", Credentials: "tok-123"}
	config, err := svc.CreatePushNotificationConfig(context.Background(), task.ID, "https://example.com/webhook", auth)
	if err != nil {
		t.Fatal(err)
	}
	if config.URL != "https://example.com/webhook" {
		t.Errorf("expected url, got %s", config.URL)
	}

	got, err := svc.GetPushNotificationConfig(context.Background(), config.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != config.ID {
		t.Errorf("expected config %d, got %d", config.ID, got.ID)
	}

	configs, err := svc.ListPushNotificationConfigs(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("expected 1 config, got %d", len(configs))
	}

	if err := svc.DeletePushNotificationConfig(context.Background(), config.ID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.GetPushNotificationConfig(context.Background(), config.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestAgentCard(t *testing.T) {
	card := DefaultAgentCard("https://mall.example.com")

	if card.Name != "Mall E-Commerce Agent" {
		t.Errorf("unexpected name: %s", card.Name)
	}
	if card.Version != "1.0.0" {
		t.Errorf("unexpected version: %s", card.Version)
	}
	if !card.Capabilities.Streaming {
		t.Error("expected streaming capability")
	}
	if len(card.Skills) != 5 {
		t.Errorf("expected 5 skills, got %d", len(card.Skills))
	}
	if len(card.Interfaces) != 1 {
		t.Errorf("expected 1 interface, got %d", len(card.Interfaces))
	}
	if card.Interfaces[0].Type != "json-rpc-2.0" {
		t.Errorf("expected json-rpc-2.0 interface, got %s", card.Interfaces[0].Type)
	}
}

func TestPartConstructors(t *testing.T) {
	tp := TextPart("hello")
	if tp.Type != PartTypeText || tp.Text != "hello" {
		t.Error("TextPart failed")
	}

	fp := FilePart("image/png", "base64data", "img.png")
	if fp.Type != PartTypeFile || fp.File.MimeType != "image/png" || fp.File.Name != "img.png" {
		t.Error("FilePart failed")
	}

	dp := DataPart(map[string]any{"key": "value"})
	if dp.Type != PartTypeData {
		t.Error("DataPart failed")
	}
}

func TestNewTaskAddArtifact(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	task := NewTask(kernel.ID(id), 1, "catalog", "")

	task.AddArtifact(Artifact{
		ID:    "result-1",
		Name:  "search-results",
		Parts: []Part{TextPart("found 3 products")},
	})

	if len(task.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(task.Artifacts))
	}
	if task.Artifacts[0].Name != "search-results" {
		t.Errorf("expected search-results, got %s", task.Artifacts[0].Name)
	}
	events := task.Events()
	found := false
	for _, e := range events {
		if _, ok := e.(TaskArtifactCreatedEvent); ok {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected TaskArtifactCreatedEvent")
	}
}

func TestAddArtifactToTask(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)
	svc.RegisterSkill("catalog", &echoSkillHandler{skillID: "catalog"})

	task, _ := svc.SendMessage(context.Background(), 1, Message{
		Role: RoleUser, Parts: []Part{TextPart("catalog search")},
	}, "")

	err := svc.AddArtifactToTask(context.Background(), task.ID, "extra-result",
		[]Part{TextPart("additional product found")})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := svc.GetTask(context.Background(), task.ID)
	if len(got.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(got.Artifacts))
	}
}

func TestAppendMessage(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := NewAgentService(taskRepo, pushRepo, &fakeLogger{}, sf)
	svc.RegisterSkill("catalog", &echoSkillHandler{skillID: "catalog"})

	task, _ := svc.SendMessage(context.Background(), 1, Message{
		Role: RoleUser, Parts: []Part{TextPart("catalog search")},
	}, "")

	err := svc.AppendMessage(context.Background(), task.ID, Message{
		Role: RoleAgent, Parts: []Part{TextPart("here are your results")},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := svc.GetTask(context.Background(), task.ID)
	if len(got.History) != 2 {
		t.Errorf("expected 2 messages in history, got %d", len(got.History))
	}
}
