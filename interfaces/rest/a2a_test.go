package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/beeleelee/mall/domain/a2a"
	"github.com/beeleelee/mall/domain/kernel"
)

type fakeTaskRepo struct {
	tasks map[kernel.ID]*a2a.Task
}

func newFakeTaskRepo() *fakeTaskRepo {
	return &fakeTaskRepo{tasks: make(map[kernel.ID]*a2a.Task)}
}

func (r *fakeTaskRepo) Save(_ context.Context, task *a2a.Task) error {
	r.tasks[task.ID] = task
	return nil
}

func (r *fakeTaskRepo) FindByID(_ context.Context, id kernel.ID) (*a2a.Task, error) {
	return r.tasks[id], nil
}

func (r *fakeTaskRepo) FindByContextID(_ context.Context, contextID string, userID kernel.ID) ([]*a2a.Task, error) {
	var res []*a2a.Task
	for _, t := range r.tasks {
		if t.ContextID == contextID && t.UserID == userID {
			res = append(res, t)
		}
	}
	return res, nil
}

func (r *fakeTaskRepo) List(_ context.Context, userID kernel.ID, skillID string, states []a2a.TaskState, pageToken string, pageSize int) ([]*a2a.Task, string, error) {
	var res []*a2a.Task
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
	configs map[kernel.ID]*a2a.PushNotificationConfig
}

func newFakePushConfigRepo() *fakePushConfigRepo {
	return &fakePushConfigRepo{configs: make(map[kernel.ID]*a2a.PushNotificationConfig)}
}

func (r *fakePushConfigRepo) Save(_ context.Context, cfg *a2a.PushNotificationConfig) error {
	r.configs[cfg.ID] = cfg
	return nil
}

func (r *fakePushConfigRepo) FindByID(_ context.Context, id kernel.ID) (*a2a.PushNotificationConfig, error) {
	return r.configs[id], nil
}

func (r *fakePushConfigRepo) FindByTaskID(_ context.Context, taskID kernel.ID) ([]*a2a.PushNotificationConfig, error) {
	var res []*a2a.PushNotificationConfig
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

type fakeA2ALogger struct{}

func (l *fakeA2ALogger) Debug(_ context.Context, msg string, fields ...kernel.LogField)            {}
func (l *fakeA2ALogger) Info(_ context.Context, msg string, fields ...kernel.LogField)             {}
func (l *fakeA2ALogger) Warn(_ context.Context, msg string, fields ...kernel.LogField)             {}
func (l *fakeA2ALogger) Error(_ context.Context, msg string, err error, fields ...kernel.LogField) {}

type echoA2ASkillHandler struct{}

func (h *echoA2ASkillHandler) Handle(_ context.Context, task *a2a.Task, msg a2a.Message) error {
	task.AddArtifact(a2a.Artifact{
		ID:    "resp-1",
		Name:  "response",
		Parts: []a2a.Part{a2a.TextPart("handled")},
	})
	return nil
}

type workingSkillHandler struct{}

func (h *workingSkillHandler) Handle(_ context.Context, task *a2a.Task, msg a2a.Message) error {
	task.AddArtifact(a2a.Artifact{
		ID:    "resp-1",
		Name:  "response",
		Parts: []a2a.Part{a2a.TextPart("working on it")},
	})
	task.Transition(a2a.TaskStateInputRequired, "need more info")
	return nil
}

func newTestA2AHandler() *A2AHandler {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := a2a.NewAgentService(taskRepo, pushRepo, &fakeA2ALogger{}, sf)
	svc.RegisterSkill("catalog", &echoA2ASkillHandler{})
	svc.RegisterSkill("cart", &echoA2ASkillHandler{})
	svc.RegisterSkill("checkout", &echoA2ASkillHandler{})
	svc.RegisterSkill("order", &echoA2ASkillHandler{})
	svc.RegisterSkill("identity", &echoA2ASkillHandler{})
	return NewA2AHandler(svc, "http://localhost:8080")
}

func TestAgentCardEndpoint(t *testing.T) {
	h := newTestA2AHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/.well-known/a2a/agent-card", nil)
	h.AgentCard(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var card a2a.AgentCard
	if err := json.Unmarshal(w.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}

	if card.Name != "Mall E-Commerce Agent" {
		t.Errorf("expected name, got %s", card.Name)
	}
	if len(card.Skills) != 5 {
		t.Errorf("expected 5 skills, got %d", len(card.Skills))
	}
	if !card.Capabilities.Streaming {
		t.Error("expected streaming capability")
	}
}

func TestA2ASendMessageRPC(t *testing.T) {
	h := newTestA2AHandler()

	body := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/send",
		"params": map[string]any{
			"message": map[string]any{
				"role":  "user",
				"parts": []map[string]any{{"type": "text", "text": "catalog search for shoes"}},
			},
		},
		"id": 1,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	h.ServeJSONRPC(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp struct {
		JSONRPC string         `json:"jsonrpc"`
		Result  map[string]any `json:"result"`
		Error   any            `json:"error"`
		ID      int            `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
	status := resp.Result["status"].(map[string]any)
	if status["state"] != "completed" {
		t.Errorf("expected completed, got %s", status["state"])
	}
}

func TestA2AUnknownMethod(t *testing.T) {
	h := newTestA2AHandler()

	body := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/unknown",
		"id":      1,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	h.ServeJSONRPC(w, r)

	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestA2AGetTaskRPC(t *testing.T) {
	h := newTestA2AHandler()

	createBody := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/send",
		"params": map[string]any{
			"message": map[string]any{
				"role":  "user",
				"parts": []map[string]any{{"type": "text", "text": "catalog search"}},
			},
		},
		"id": 1,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(createBody))
	h.ServeJSONRPC(w, r)

	var createResp struct {
		Result map[string]any `json:"result"`
	}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	taskID := int64(createResp.Result["id"].(float64))

	getBody := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/get",
		"params":  map[string]any{"id": taskID},
		"id":      2,
	})

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(getBody))
	h.ServeJSONRPC(w2, r2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestA2ACancelTaskRPC(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	taskRepo := newFakeTaskRepo()
	pushRepo := newFakePushConfigRepo()
	svc := a2a.NewAgentService(taskRepo, pushRepo, &fakeA2ALogger{}, sf)
	svc.RegisterSkill("checkout", &workingSkillHandler{})
	h := NewA2AHandler(svc, "http://localhost:8080")

	id, _ := sf.NextID()
	task := a2a.NewTask(kernel.ID(id), 1, "checkout", "")
	taskRepo.Save(context.Background(), task)

	cancelBody := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/cancel",
		"params":  map[string]any{"id": id},
		"id":      2,
	})

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(cancelBody))
	h.ServeJSONRPC(w2, r2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}

	var cancelResp struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	json.Unmarshal(w2.Body.Bytes(), &cancelResp)
	if cancelResp.Result["status"].(map[string]any)["state"] != "canceled" {
		t.Errorf("expected canceled, got %v", cancelResp.Result["status"])
	}
}

func TestA2AListTasksRPC(t *testing.T) {
	h := newTestA2AHandler()

	for i := 0; i < 3; i++ {
		body := toJSON(map[string]any{
			"jsonrpc": "2.0",
			"method":  "tasks/send",
			"params": map[string]any{
				"message": map[string]any{
					"role":  "user",
					"parts": []map[string]any{{"type": "text", "text": "catalog search"}},
				},
			},
			"id": i + 1,
		})
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
		h.ServeJSONRPC(w, r)
	}

	listBody := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/list",
		"params":  map[string]any{},
		"id":      10,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(listBody))
	h.ServeJSONRPC(w, r)

	var listResp struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Error != nil {
		t.Fatalf("unexpected error: %v", listResp.Error)
	}
	tasks := listResp.Result["tasks"].([]any)
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestA2APushConfigRPC(t *testing.T) {
	h := newTestA2AHandler()

	createBody := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/send",
		"params": map[string]any{
			"message": map[string]any{
				"role":  "user",
				"parts": []map[string]any{{"type": "text", "text": "catalog search"}},
			},
		},
		"id": 1,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(createBody))
	h.ServeJSONRPC(w, r)

	var createResp struct {
		Result map[string]any `json:"result"`
	}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	taskID := int64(createResp.Result["id"].(float64))

	pushBody := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "pushConfig/create",
		"params": map[string]any{
			"taskId": taskID,
			"url":    "https://example.com/webhook",
			"authInfo": map[string]string{
				"scheme": "bearer", "credentials": "tok-123",
			},
		},
		"id": 2,
	})

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(pushBody))
	h.ServeJSONRPC(w2, r2)

	var pushResp struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	json.Unmarshal(w2.Body.Bytes(), &pushResp)
	if pushResp.Error != nil {
		t.Fatalf("unexpected error: %v", pushResp.Error)
	}
	if pushResp.Result["url"] != "https://example.com/webhook" {
		t.Errorf("expected correct url, got %v", pushResp.Result["url"])
	}
}

func TestA2AAgentCardRPC(t *testing.T) {
	h := newTestA2AHandler()

	body := toJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "agent/getCard",
		"id":      1,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	h.ServeJSONRPC(w, r)

	var resp struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.Result["name"] != "Mall E-Commerce Agent" {
		t.Errorf("expected correct name, got %v", resp.Result["name"])
	}
}
