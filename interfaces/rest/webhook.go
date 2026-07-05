package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type WebhookHandler struct {
	svc *domain.WebhookService
}

func NewWebhookHandler(svc *domain.WebhookService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

type registerWebhookRequest struct {
	URL    string   `json:"url"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events,omitempty"`
}

type webhookResponse struct {
	ID        int64    `json:"id"`
	UserID    int64    `json:"user_id"`
	URL       string   `json:"url"`
	Secret    string   `json:"secret,omitempty"`
	Events    []string `json:"events"`
	Active    bool     `json:"active"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

func buildWebhookResponse(w *domain.Webhook) webhookResponse {
	return webhookResponse{
		ID:        w.ID.Int64(),
		UserID:    w.UserID.Int64(),
		URL:       w.URL,
		Secret:    w.Secret,
		Events:    w.Events,
		Active:    w.Active,
		CreatedAt: w.CreatedAt.UnixMilli(),
		UpdatedAt: w.UpdatedAt.UnixMilli(),
	}
}

func (h *WebhookHandler) Register(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var req registerWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	if req.URL == "" {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "url is required"))
		return
	}

	webhook, err := h.svc.Register(r.Context(), userID, req.URL, req.Secret, req.Events)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(buildWebhookResponse(webhook))
}

func (h *WebhookHandler) ListByUser(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	webhooks, err := h.svc.ListByUser(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := make([]webhookResponse, len(webhooks))
	for i, w := range webhooks {
		resp[i] = buildWebhookResponse(w)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid webhook id"))
		return
	}

	if err := h.svc.Delete(r.Context(), userID, kernel.ID(id)); err != nil {
		writeDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
