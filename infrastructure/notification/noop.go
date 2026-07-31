package notification

import (
	"context"

	domain "github.com/beeleelee/mall/domain/notification"
)

type NoopEmailSender struct{}

func (NoopEmailSender) Send(_ context.Context, _ domain.EmailMessage) error {
	return nil
}
