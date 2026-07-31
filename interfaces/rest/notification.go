package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/notification"
)

type NotificationHandler struct {
	svc *domain.NotificationService
}

func NewNotificationHandler(svc *domain.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

type notificationResponse struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
}

func buildNotificationResponse(n *domain.Notification) notificationResponse {
	return notificationResponse{
		ID:        n.ID.Int64(),
		UserID:    n.UserID.Int64(),
		Type:      string(n.Type),
		Title:     n.Title,
		Body:      n.Body,
		Read:      n.Read,
		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	notifs, err := h.svc.ListByUser(r.Context(), userID, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := make([]notificationResponse, 0, len(notifs))
	for _, n := range notifs {
		resp = append(resp, buildNotificationResponse(n))
	}

	unread, err := h.svc.UnreadCount(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"notifications": resp,
		"unread_count":  unread,
	})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid notification id"))
		return
	}

	if err := h.svc.MarkRead(r.Context(), kernel.ID(id), userID); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "marked_read"})
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if err := h.svc.MarkAllRead(r.Context(), userID); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "marked_read"})
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	count, err := h.svc.UnreadCount(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"unread_count": count})
}

type preferencesResponse struct {
	UserID       int64    `json:"user_id"`
	EmailEnabled bool     `json:"email_enabled"`
	InAppEnabled bool     `json:"in_app_enabled"`
	Types        []string `json:"types"`
}

func buildPreferencesResponse(p *domain.NotificationPreferences) preferencesResponse {
	types := []string{}
	if p.Types != nil {
		for _, t := range *p.Types {
			types = append(types, string(t))
		}
	}
	return preferencesResponse{
		UserID:       p.UserID.Int64(),
		EmailEnabled: p.EmailEnabled,
		InAppEnabled: p.InAppEnabled,
		Types:        types,
	}
}

func (h *NotificationHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	prefs, err := h.svc.GetPreferences(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildPreferencesResponse(prefs))
}

type updatePreferencesRequest struct {
	EmailEnabled *bool    `json:"email_enabled"`
	InAppEnabled *bool    `json:"in_app_enabled"`
	Types        []string `json:"types"`
}

func (h *NotificationHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var req updatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid request body"))
		return
	}

	var types *[]domain.NotificationType
	if req.Types != nil {
		list := make([]domain.NotificationType, 0, len(req.Types))
		for _, t := range req.Types {
			list = append(list, domain.NotificationType(t))
		}
		types = &list
	}

	prefs, err := h.svc.UpdatePreferences(r.Context(), userID, req.EmailEnabled, req.InAppEnabled, types)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(buildPreferencesResponse(prefs))
}
