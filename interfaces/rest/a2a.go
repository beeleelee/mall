package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/beeleelee/mall/domain/a2a"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type A2AHandler struct {
	svc     *a2a.AgentService
	baseURL string
}

func NewA2AHandler(svc *a2a.AgentService, baseURL string) *A2AHandler {
	return &A2AHandler{svc: svc, baseURL: baseURL}
}

func (h *A2AHandler) AgentCard(w http.ResponseWriter, r *http.Request) {
	card := a2a.DefaultAgentCard(h.baseURL)
	writeJSON(w, http.StatusOK, card)
}

func (h *A2AHandler) ExtendedAgentCard(w http.ResponseWriter, r *http.Request) {
	card := a2a.DefaultAgentCard(h.baseURL)
	card.Skills = append(card.Skills, a2a.AgentSkill{
		ID:          "admin",
		Name:        "Admin Management",
		Description: "Manage products, inventory, orders, and users (admin only)",
		Tags:        []string{"admin", "management"},
		Examples:    []string{"List all orders", "Create a product", "Activate user"},
	})
	writeJSON(w, http.StatusOK, card)
}

func (h *A2AHandler) ServeJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeA2AError(w, nil, -32600, "only POST is accepted")
		return
	}

	var req a2aRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeA2AError(w, nil, -32700, "parse error: invalid JSON")
		return
	}

	if req.JSONRPC != "2.0" {
		writeA2AError(w, req.ID, -32600, "invalid jsonrpc version")
		return
	}

	userID := extractUserID(r)

	switch req.Method {
	case "tasks/send":
		h.handleSendMessage(w, r, req, userID)
	case "tasks/get":
		h.handleGetTask(w, r, req, userID)
	case "tasks/list":
		h.handleListTasks(w, r, req, userID)
	case "tasks/cancel":
		h.handleCancelTask(w, r, req, userID)
	case "tasks/sendStream":
		h.handleSendStream(w, r, req, userID)
	case "tasks/subscribe":
		h.handleSubscribeTask(w, r, req, userID)
	case "pushConfig/create":
		h.handleCreatePushConfig(w, r, req, userID)
	case "pushConfig/get":
		h.handleGetPushConfig(w, r, req, userID)
	case "pushConfig/list":
		h.handleListPushConfigs(w, r, req, userID)
	case "pushConfig/delete":
		h.handleDeletePushConfig(w, r, req, userID)
	case "agent/getCard":
		writeA2AResult(w, req.ID, a2a.DefaultAgentCard(h.baseURL))
	case "agent/getExtendedCard":
		card := a2a.DefaultAgentCard(h.baseURL)
		card.Skills = append(card.Skills, a2a.AgentSkill{
			ID: "admin", Name: "Admin Management",
			Description: "Manage products, inventory, orders, and users (admin only)",
			Tags:        []string{"admin", "management"},
		})
		writeA2AResult(w, req.ID, card)
	default:
		writeA2AError(w, req.ID, -32601, "method not found: "+req.Method)
	}
}

func (h *A2AHandler) handleSendMessage(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		Message   a2a.Message `json:"message"`
		ContextID string      `json:"contextId,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params: "+err.Error())
		return
	}

	task, err := h.svc.SendMessage(r.Context(), userID, params.Message, params.ContextID)
	if err != nil {
		writeA2AError(w, req.ID, -32000, err.Error())
		return
	}

	writeA2AResult(w, req.ID, taskToRPC(task))
}

func (h *A2AHandler) handleGetTask(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	task, err := h.svc.GetTask(r.Context(), kernel.ID(params.ID))
	if err != nil {
		writeA2AError(w, req.ID, -32000, err.Error())
		return
	}

	writeA2AResult(w, req.ID, taskToRPC(task))
}

func (h *A2AHandler) handleListTasks(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		SkillID   string   `json:"skillId,omitempty"`
		States    []string `json:"states,omitempty"`
		PageToken string   `json:"pageToken,omitempty"`
		PageSize  int      `json:"pageSize,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	states := make([]a2a.TaskState, 0, len(params.States))
	for _, s := range params.States {
		states = append(states, a2a.TaskState(s))
	}

	tasks, nextToken, err := h.svc.ListTasks(r.Context(), userID, params.SkillID, states, params.PageToken, params.PageSize)
	if err != nil {
		writeA2AError(w, req.ID, -32000, err.Error())
		return
	}

	rpcTasks := make([]any, 0, len(tasks))
	for _, t := range tasks {
		rpcTasks = append(rpcTasks, taskToRPC(t))
	}

	writeA2AResult(w, req.ID, map[string]any{
		"tasks":         rpcTasks,
		"nextPageToken": nextToken,
	})
}

func (h *A2AHandler) handleCancelTask(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	task, err := h.svc.CancelTask(r.Context(), kernel.ID(params.ID))
	if err != nil {
		writeA2AError(w, req.ID, -32000, err.Error())
		return
	}

	writeA2AResult(w, req.ID, taskToRPC(task))
}

func (h *A2AHandler) handleSendStream(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		Message   a2a.Message `json:"message"`
		ContextID string      `json:"contextId,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeA2AError(w, req.ID, -32000, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	task, err := h.svc.SendMessage(r.Context(), userID, params.Message, params.ContextID)
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]any{"error": err.Error()}))
		flusher.Flush()
		return
	}

	data := toJSON(a2a.StreamResponse{Task: task})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	if task.Status.State == a2a.TaskStateWorking || task.Status.State == a2a.TaskStateSubmitted {
		for i := 0; i < 30; i++ {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(500 * time.Millisecond):
			}

			updated, err := h.svc.GetTask(r.Context(), task.ID)
			if err != nil {
				continue
			}

			if updated.Status.State != task.Status.State {
				updateData := toJSON(a2a.StreamResponse{
					TaskStatusUpdate: &a2a.TaskStatusUpdateEvent{
						TaskID: task.ID.String(),
						Status: updated.Status,
					},
				})
				fmt.Fprintf(w, "data: %s\n\n", updateData)
				flusher.Flush()

				if updated.Status.State.IsTerminal() || updated.Status.State == a2a.TaskStateInputRequired {
					break
				}
			}
		}
	}

	fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]string{"event": "complete"}))
	flusher.Flush()
}

func (h *A2AHandler) handleSubscribeTask(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeA2AError(w, req.ID, -32000, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	task, err := h.svc.GetTask(r.Context(), kernel.ID(params.ID))
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]any{"error": err.Error()}))
		flusher.Flush()
		return
	}

	initialData := toJSON(a2a.StreamResponse{Task: task})
	fmt.Fprintf(w, "data: %s\n\n", initialData)
	flusher.Flush()

	if task.Status.State.IsTerminal() {
		fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]string{"event": "complete"}))
		flusher.Flush()
		return
	}

	for i := 0; i < 60; i++ {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(1 * time.Second):
		}

		updated, err := h.svc.GetTask(r.Context(), task.ID)
		if err != nil {
			continue
		}

		if updated.Status.State != task.Status.State {
			updateData := toJSON(a2a.StreamResponse{
				TaskStatusUpdate: &a2a.TaskStatusUpdateEvent{
					TaskID: task.ID.String(),
					Status: updated.Status,
				},
			})
			fmt.Fprintf(w, "data: %s\n\n", updateData)
			flusher.Flush()

			if updated.Status.State.IsTerminal() || updated.Status.State == a2a.TaskStateInputRequired {
				break
			}
		}
	}

	fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]string{"event": "complete"}))
	flusher.Flush()
}

func (h *A2AHandler) handleCreatePushConfig(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		TaskID int64        `json:"taskId"`
		URL    string       `json:"url"`
		Auth   a2a.AuthInfo `json:"authInfo,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	config, err := h.svc.CreatePushNotificationConfig(r.Context(), kernel.ID(params.TaskID), params.URL, params.Auth)
	if err != nil {
		writeA2AError(w, req.ID, -32000, err.Error())
		return
	}

	writeA2AResult(w, req.ID, map[string]any{
		"id":     config.ID.Int64(),
		"taskId": config.TaskID.Int64(),
		"url":    config.URL,
	})
}

func (h *A2AHandler) handleGetPushConfig(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	config, err := h.svc.GetPushNotificationConfig(r.Context(), kernel.ID(params.ID))
	if err != nil {
		writeA2AError(w, req.ID, -32000, err.Error())
		return
	}

	writeA2AResult(w, req.ID, map[string]any{
		"id":     config.ID.Int64(),
		"taskId": config.TaskID.Int64(),
		"url":    config.URL,
	})
}

func (h *A2AHandler) handleListPushConfigs(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		TaskID int64 `json:"taskId"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	configs, err := h.svc.ListPushNotificationConfigs(r.Context(), kernel.ID(params.TaskID))
	if err != nil {
		writeA2AError(w, req.ID, -32000, err.Error())
		return
	}

	rpcConfigs := make([]map[string]any, 0, len(configs))
	for _, c := range configs {
		rpcConfigs = append(rpcConfigs, map[string]any{
			"id":     c.ID.Int64(),
			"taskId": c.TaskID.Int64(),
			"url":    c.URL,
		})
	}

	writeA2AResult(w, req.ID, map[string]any{"pushConfigs": rpcConfigs})
}

func (h *A2AHandler) handleDeletePushConfig(w http.ResponseWriter, r *http.Request, req a2aRPCRequest, userID kernel.ID) {
	var params struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeA2AError(w, req.ID, -32602, "invalid params")
		return
	}

	if err := h.svc.DeletePushNotificationConfig(r.Context(), kernel.ID(params.ID)); err != nil {
		writeA2AError(w, req.ID, -32000, err.Error())
		return
	}

	writeA2AResult(w, req.ID, map[string]string{"status": "deleted"})
}

type a2aRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type a2aRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID any `json:"id"`
}

func writeA2AResult(w http.ResponseWriter, id any, result any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a2aRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	})
}

func writeA2AError(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(a2aRPCResponse{
		JSONRPC: "2.0",
		Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: code, Message: msg},
		ID: id,
	})
}

func taskToRPC(task *a2a.Task) map[string]any {
	artifacts := make([]map[string]any, 0, len(task.Artifacts))
	for _, a := range task.Artifacts {
		artifacts = append(artifacts, map[string]any{
			"id":    a.ID,
			"name":  a.Name,
			"parts": a.Parts,
			"index": a.Index,
		})
	}

	history := make([]map[string]any, 0, len(task.History))
	for _, m := range task.History {
		history = append(history, map[string]any{
			"role":  m.Role,
			"parts": m.Parts,
		})
	}

	result := map[string]any{
		"id":      task.ID.Int64(),
		"skillId": task.SkillID,
		"status": map[string]any{
			"state":     task.Status.State,
			"message":   task.Status.Message,
			"updatedAt": task.Status.UpdatedAt,
		},
		"artifacts": artifacts,
	}
	if task.ContextID != "" {
		result["contextId"] = task.ContextID
	}
	if len(history) > 0 {
		result["history"] = history
	}
	if task.Status.CompletedAt != nil {
		result["status"].(map[string]any)["completedAt"] = task.Status.CompletedAt
	}
	return result
}

func extractUserID(r *http.Request) kernel.ID {
	userInfo, ok := middleware.UserFromContext(r.Context())
	if ok {
		return kernel.ID(userInfo.UserID)
	}
	return 0
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ServeSSE handles the REST-style SSE streaming for a task
func (h *A2AHandler) ServeSSE(w http.ResponseWriter, r *http.Request, taskID kernel.ID) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	task, err := h.svc.GetTask(r.Context(), taskID)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	data := toJSON(a2a.StreamResponse{Task: task})
	fmt.Fprintf(w, "event: task\ndata: %s\n\n", data)
	flusher.Flush()

	if task.Status.State.IsTerminal() {
		fmt.Fprintf(w, "event: complete\ndata: {}\n\n")
		flusher.Flush()
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(w, "event: complete\ndata: {}\n\n")
			flusher.Flush()
			return
		case <-ticker.C:
			updated, err := h.svc.GetTask(ctx, taskID)
			if err != nil {
				continue
			}

			if updated.Status.State != task.Status.State {
				updateData := toJSON(a2a.StreamResponse{
					TaskStatusUpdate: &a2a.TaskStatusUpdateEvent{
						TaskID: taskID.String(),
						Status: updated.Status,
					},
				})
				fmt.Fprintf(w, "event: status\ndata: %s\n\n", updateData)
				flusher.Flush()

				if updated.Status.State.IsTerminal() || updated.Status.State == a2a.TaskStateInputRequired {
					fmt.Fprintf(w, "event: complete\ndata: {}\n\n")
					flusher.Flush()
					return
				}
			}
		}
	}
}

// ContextKey type for A2A-specific context values
type contextKey string

const UserIDKey contextKey = "a2a_user_id"

// REST-style handlers for the A2A protocol
func (h *A2AHandler) HandleSendMessageREST(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message   a2a.Message `json:"message"`
		ContextID string      `json:"contextId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID := extractUserID(r)
	task, err := h.svc.SendMessage(r.Context(), userID, req.Message, req.ContextID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, taskToRPC(task))
}

func (h *A2AHandler) HandleGetTaskREST(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/a2a/tasks/")
	idStr = strings.Split(idStr, "/")[0]
	if idStr == "" {
		http.Error(w, "missing task id", http.StatusBadRequest)
		return
	}

	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return
	}

	task, err := h.svc.GetTask(r.Context(), kernel.ID(id))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, taskToRPC(task))
}

func (h *A2AHandler) HandleCancelTaskREST(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/a2a/tasks/")
	idStr = strings.Split(idStr, "/")[0]
	if idStr == "" {
		http.Error(w, "missing task id", http.StatusBadRequest)
		return
	}

	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return
	}

	task, err := h.svc.CancelTask(r.Context(), kernel.ID(id))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, taskToRPC(task))
}

func (h *A2AHandler) HandleListTasksREST(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	skillID := r.URL.Query().Get("skillId")
	pageToken := r.URL.Query().Get("pageToken")
	pageSize := 20

	statesStr := r.URL.Query()["state"]
	states := make([]a2a.TaskState, 0, len(statesStr))
	for _, s := range statesStr {
		states = append(states, a2a.TaskState(s))
	}

	tasks, nextToken, err := h.svc.ListTasks(r.Context(), userID, skillID, states, pageToken, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	rpcTasks := make([]any, 0, len(tasks))
	for _, t := range tasks {
		rpcTasks = append(rpcTasks, taskToRPC(t))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tasks":         rpcTasks,
		"nextPageToken": nextToken,
	})
}
