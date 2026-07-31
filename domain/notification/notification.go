package notification

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type NotificationType string

const (
	NotificationTypeOrder        NotificationType = "order"
	NotificationTypeShipping     NotificationType = "shipping"
	NotificationTypeSubscription NotificationType = "subscription"
	NotificationTypeRefund       NotificationType = "refund"
	NotificationTypeReview       NotificationType = "review"
	NotificationTypeAccount      NotificationType = "account"
)

type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelInApp Channel = "in_app"
)

type Notification struct {
	kernel.AggregateRoot
	UserID kernel.ID
	Type   NotificationType
	Title  string
	Body   string
	Read   bool
}

func NewNotification(id, userID kernel.ID, ntype NotificationType, title, body string) (*Notification, error) {
	if id <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "notification id must be positive")
	}
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user id must be positive")
	}
	if ntype == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "notification type is required")
	}
	if title == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "notification title is required")
	}
	n := &Notification{
		AggregateRoot: kernel.NewAggregateRoot(id),
		UserID:        userID,
		Type:          ntype,
		Title:         title,
		Body:          body,
		Read:          false,
	}
	n.touch()
	return n, nil
}

func (n *Notification) MarkRead() error {
	if n.Read {
		return kernel.NewDomainError(kernel.ErrConflict, "notification already read")
	}
	n.Read = true
	n.touch()
	return nil
}

func (n *Notification) IsUnread() bool {
	return !n.Read
}

func (n *Notification) touch() {
	n.UpdatedAt = time.Now()
}

type NotificationPreferences struct {
	kernel.Entity
	UserID       kernel.ID
	EmailEnabled bool
	InAppEnabled bool
	Types        *[]NotificationType
}

func NewNotificationPreferences(id, userID kernel.ID) (*NotificationPreferences, error) {
	if id <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "preferences id must be positive")
	}
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user id must be positive")
	}
	return &NotificationPreferences{
		Entity:       kernel.NewEntity(id),
		UserID:       userID,
		EmailEnabled: true,
		InAppEnabled: true,
		Types:        nil,
	}, nil
}

func (p *NotificationPreferences) SetChannel(channel Channel, enabled bool) error {
	switch channel {
	case ChannelEmail:
		p.EmailEnabled = enabled
	case ChannelInApp:
		p.InAppEnabled = enabled
	default:
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown notification channel: "+string(channel))
	}
	p.UpdatedAt = time.Now()
	return nil
}

func (p *NotificationPreferences) typesList() []NotificationType {
	if p.Types == nil {
		return nil
	}
	return *p.Types
}

func (p *NotificationPreferences) SetType(ntype NotificationType, enabled bool) {
	if enabled {
		list := p.typesList()
		for _, t := range list {
			if t == ntype {
				return
			}
		}
		p.Types = &list
		*p.Types = append(*p.Types, ntype)
		p.UpdatedAt = time.Now()
		return
	}
	list := p.typesList()
	filtered := list[:0]
	for _, t := range list {
		if t != ntype {
			filtered = append(filtered, t)
		}
	}
	p.Types = &filtered
	p.UpdatedAt = time.Now()
}

func (p *NotificationPreferences) Allows(channel Channel, ntype NotificationType) bool {
	switch channel {
	case ChannelEmail:
		if !p.EmailEnabled {
			return false
		}
	case ChannelInApp:
		if !p.InAppEnabled {
			return false
		}
	default:
		return false
	}
	if p.Types == nil {
		return true
	}
	for _, t := range *p.Types {
		if t == ntype {
			return true
		}
	}
	return false
}
