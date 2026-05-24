package order

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type Webhook struct {
	kernel.AggregateRoot
	UserID kernel.ID
	URL    string
	Secret string
	Events []string
	Active bool
}

func NewWebhook(id kernel.ID, userID kernel.ID, url, secret string, events []string) (*Webhook, error) {
	if url == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "webhook url must not be empty")
	}

	w := &Webhook{
		AggregateRoot: kernel.NewAggregateRoot(id),
		UserID:        userID,
		URL:           url,
		Secret:        secret,
		Events:        events,
		Active:        true,
	}
	return w, nil
}

type WebhookRepository interface {
	Save(ctx context.Context, webhook *Webhook) error
	FindByID(ctx context.Context, id kernel.ID) (*Webhook, error)
	FindByUserID(ctx context.Context, userID kernel.ID) ([]*Webhook, error)
	FindByEvent(ctx context.Context, event string) ([]*Webhook, error)
	Delete(ctx context.Context, id kernel.ID) error
}

type WebhookService struct {
	repo WebhookRepository
	sf   *kernel.Snowflake
}

func NewWebhookService(repo WebhookRepository, sf *kernel.Snowflake) *WebhookService {
	return &WebhookService{repo: repo, sf: sf}
}

func (s *WebhookService) Register(ctx context.Context, userID kernel.ID, url, secret string, events []string) (*Webhook, error) {
	id, err := s.sf.NextID()
	if err != nil {
		return nil, err
	}

	w, err := NewWebhook(id, userID, url, secret, events)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, w); err != nil {
		return nil, err
	}

	return w, nil
}

func (s *WebhookService) ListByUser(ctx context.Context, userID kernel.ID) ([]*Webhook, error) {
	return s.repo.FindByUserID(ctx, userID)
}

func (s *WebhookService) Delete(ctx context.Context, userID kernel.ID, id kernel.ID) error {
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if w.UserID != userID {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "webhook does not belong to user")
	}

	return s.repo.Delete(ctx, id)
}
