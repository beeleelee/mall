package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/notification"
)

type NotificationMCPHandler struct {
	svc   *domain.NotificationService
	tools []ToolDefinition
}

func NewNotificationMCPHandler(svc *domain.NotificationService) *NotificationMCPHandler {
	return &NotificationMCPHandler{
		svc:   svc,
		tools: notificationTools,
	}
}

var notificationTools = []ToolDefinition{
	{
		Name:        "list_notifications",
		Description: "List in-app notifications for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID to list notifications for"},
				"limit":   {Type: "number", Description: "Optional maximum number of notifications to return"},
			},
		},
	},
	{
		Name:        "mark_notification_read",
		Description: "Mark a single notification as read",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"notification_id": {Type: "number", Description: "Notification ID to mark read"},
				"user_id":         {Type: "number", Description: "User ID who owns the notification"},
			},
		},
	},
	{
		Name:        "mark_all_notifications_read",
		Description: "Mark all notifications for a user as read",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID to mark all notifications read for"},
			},
		},
	},
	{
		Name:        "get_unread_notification_count",
		Description: "Get the unread notification count for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID to count unread notifications for"},
			},
		},
	},
	{
		Name:        "get_notification_preferences",
		Description: "Get notification preferences for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID to get preferences for"},
			},
		},
	},
	{
		Name:        "update_notification_preferences",
		Description: "Update notification preferences for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id":        {Type: "number", Description: "User ID to update preferences for"},
				"email_enabled":  {Type: "boolean", Description: "Optional: enable or disable email notifications"},
				"in_app_enabled": {Type: "boolean", Description: "Optional: enable or disable in-app notifications"},
				"types":          {Type: "array", Description: "Optional: list of enabled notification types (e.g. order, shipping, subscription)"},
			},
		},
	},
}

func (h *NotificationMCPHandler) ListTools() []ToolDefinition {
	return h.tools
}

func (h *NotificationMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "list_notifications":
		return h.callList(ctx, raw)
	case "mark_notification_read":
		return h.callMarkRead(ctx, raw)
	case "mark_all_notifications_read":
		return h.callMarkAllRead(ctx, raw)
	case "get_unread_notification_count":
		return h.callUnreadCount(ctx, raw)
	case "get_notification_preferences":
		return h.callGetPreferences(ctx, raw)
	case "update_notification_preferences":
		return h.callUpdatePreferences(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type notificationListArgs struct {
	UserID int64 `json:"user_id"`
	Limit  int   `json:"limit"`
}

type notificationIDArgs struct {
	NotificationID int64 `json:"notification_id"`
	UserID         int64 `json:"user_id"`
}

type userIDArgs struct {
	UserID int64 `json:"user_id"`
}

type updateNotificationPrefsArgs struct {
	UserID       int64                     `json:"user_id"`
	EmailEnabled *bool                     `json:"email_enabled"`
	InAppEnabled *bool                     `json:"in_app_enabled"`
	Types        []domain.NotificationType `json:"types"`
}

func buildNotificationMap(n *domain.Notification) map[string]any {
	return map[string]any{
		"id":         n.ID.Int64(),
		"user_id":    n.UserID.Int64(),
		"type":       string(n.Type),
		"title":      n.Title,
		"body":       n.Body,
		"read":       n.Read,
		"created_at": n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func buildPreferencesMap(p *domain.NotificationPreferences) map[string]any {
	types := []string{}
	if p.Types != nil {
		for _, t := range *p.Types {
			types = append(types, string(t))
		}
	}
	return map[string]any{
		"user_id":        p.UserID.Int64(),
		"email_enabled":  p.EmailEnabled,
		"in_app_enabled": p.InAppEnabled,
		"types":          types,
	}
}

func (h *NotificationMCPHandler) callList(ctx context.Context, raw json.RawMessage) (any, error) {
	var args notificationListArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	notifs, err := h.svc.ListByUser(ctx, kernel.ID(args.UserID), args.Limit)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(notifs))
	for _, n := range notifs {
		result = append(result, buildNotificationMap(n))
	}
	count, err := h.svc.UnreadCount(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}
	return map[string]any{"notifications": result, "unread_count": count}, nil
}

func (h *NotificationMCPHandler) callMarkRead(ctx context.Context, raw json.RawMessage) (any, error) {
	var args notificationIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.NotificationID <= 0 || args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "notification_id and user_id are required")
	}
	if err := h.svc.MarkRead(ctx, kernel.ID(args.NotificationID), kernel.ID(args.UserID)); err != nil {
		return nil, err
	}
	return map[string]any{"status": "marked_read"}, nil
}

func (h *NotificationMCPHandler) callMarkAllRead(ctx context.Context, raw json.RawMessage) (any, error) {
	var args userIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	if err := h.svc.MarkAllRead(ctx, kernel.ID(args.UserID)); err != nil {
		return nil, err
	}
	return map[string]any{"status": "marked_read"}, nil
}

func (h *NotificationMCPHandler) callUnreadCount(ctx context.Context, raw json.RawMessage) (any, error) {
	var args userIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	count, err := h.svc.UnreadCount(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}
	return map[string]any{"unread_count": count}, nil
}

func (h *NotificationMCPHandler) callGetPreferences(ctx context.Context, raw json.RawMessage) (any, error) {
	var args userIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	prefs, err := h.svc.GetPreferences(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}
	return buildPreferencesMap(prefs), nil
}

func (h *NotificationMCPHandler) callUpdatePreferences(ctx context.Context, raw json.RawMessage) (any, error) {
	var args updateNotificationPrefsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	var types *[]domain.NotificationType
	if args.Types != nil {
		types = &args.Types
	}
	prefs, err := h.svc.UpdatePreferences(ctx, kernel.ID(args.UserID), args.EmailEnabled, args.InAppEnabled, types)
	if err != nil {
		return nil, err
	}
	return buildPreferencesMap(prefs), nil
}
