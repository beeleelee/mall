package notification

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type EmailAddress string

type EmailMessage struct {
	To        EmailAddress
	Subject   string
	PlainBody string
	HTMLBody  string
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

type InAppWriter interface {
	Write(ctx context.Context, n *Notification) error
}

type NotificationService struct {
	sender   EmailSender
	writer   InAppWriter
	repo     NotificationRepository
	prefRepo NotificationPreferenceRepository
	sf       *kernel.Snowflake
	logger   kernel.Logger
}

type NotificationServiceOption func(*NotificationService)

func WithSnowflake(sf *kernel.Snowflake) NotificationServiceOption {
	return func(s *NotificationService) {
		s.sf = sf
	}
}

func WithNotificationRepository(r NotificationRepository) NotificationServiceOption {
	return func(s *NotificationService) {
		s.repo = r
	}
}

func WithInAppWriter(w InAppWriter) NotificationServiceOption {
	return func(s *NotificationService) {
		s.writer = w
	}
}

func WithPreferenceRepository(r NotificationPreferenceRepository) NotificationServiceOption {
	return func(s *NotificationService) {
		s.prefRepo = r
	}
}

func NewNotificationService(sender EmailSender, logger kernel.Logger, opts ...NotificationServiceOption) *NotificationService {
	svc := &NotificationService{sender: sender, logger: logger}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *NotificationService) allowsEmail(ctx context.Context, userID kernel.ID, ntype NotificationType) bool {
	if s.prefRepo == nil {
		return true
	}
	prefs, err := s.prefRepo.Get(ctx, userID)
	if err != nil {
		return true
	}
	return prefs.Allows(ChannelEmail, ntype)
}

func (s *NotificationService) allowsInApp(ctx context.Context, userID kernel.ID, ntype NotificationType) bool {
	if s.prefRepo == nil {
		return true
	}
	prefs, err := s.prefRepo.Get(ctx, userID)
	if err != nil {
		return true
	}
	return prefs.Allows(ChannelInApp, ntype)
}

func (s *NotificationService) NotifyInApp(ctx context.Context, id, userID kernel.ID, ntype NotificationType, title, body string) error {
	if s.writer == nil {
		return nil
	}
	if !s.allowsInApp(ctx, userID, ntype) {
		return nil
	}
	n, err := NewNotification(id, userID, ntype, title, body)
	if err != nil {
		return err
	}
	return s.writer.Write(ctx, n)
}

func (s *NotificationService) ListByUser(ctx context.Context, userID kernel.ID, limit int) ([]*Notification, error) {
	if s.repo == nil {
		return nil, kernel.NewDomainError(kernel.ErrInternal, "notification repository not configured")
	}
	return s.repo.FindByUserID(ctx, userID, limit)
}

func (s *NotificationService) MarkRead(ctx context.Context, id, userID kernel.ID) error {
	if s.repo == nil {
		return kernel.NewDomainError(kernel.ErrInternal, "notification repository not configured")
	}
	return s.repo.MarkRead(ctx, id, userID)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID kernel.ID) error {
	if s.repo == nil {
		return kernel.NewDomainError(kernel.ErrInternal, "notification repository not configured")
	}
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *NotificationService) UnreadCount(ctx context.Context, userID kernel.ID) (int, error) {
	if s.repo == nil {
		return 0, kernel.NewDomainError(kernel.ErrInternal, "notification repository not configured")
	}
	return s.repo.UnreadCount(ctx, userID)
}

func (s *NotificationService) GetPreferences(ctx context.Context, userID kernel.ID) (*NotificationPreferences, error) {
	if s.prefRepo == nil {
		return nil, kernel.NewDomainError(kernel.ErrInternal, "notification preference repository not configured")
	}
	return s.prefRepo.Get(ctx, userID)
}

func (s *NotificationService) UpdatePreferences(ctx context.Context, userID kernel.ID, emailEnabled, inAppEnabled *bool, types *[]NotificationType) (*NotificationPreferences, error) {
	if s.prefRepo == nil {
		return nil, kernel.NewDomainError(kernel.ErrInternal, "notification preference repository not configured")
	}

	prefs, err := s.prefRepo.Get(ctx, userID)
	if err != nil {
		if s.sf == nil {
			return nil, kernel.NewDomainError(kernel.ErrInternal, "snowflake generator not configured")
		}
		id, genErr := s.sf.NextID()
		if genErr != nil {
			return nil, genErr
		}
		prefs, err = NewNotificationPreferences(id, userID)
		if err != nil {
			return nil, err
		}
	}

	if emailEnabled != nil {
		if err := prefs.SetChannel(ChannelEmail, *emailEnabled); err != nil {
			return nil, err
		}
	}
	if inAppEnabled != nil {
		if err := prefs.SetChannel(ChannelInApp, *inAppEnabled); err != nil {
			return nil, err
		}
	}
	if types != nil {
		prefs.Types = types
		prefs.UpdatedAt = time.Now()
	}

	if err := s.prefRepo.Upsert(ctx, prefs); err != nil {
		return nil, err
	}
	return prefs, nil
}

func (s *NotificationService) SendOrderConfirmation(ctx context.Context, to EmailAddress, userName string, orderID int64, total int64) error {
	err := s.sender.Send(ctx, EmailMessage{
		To:        to,
		Subject:   "Order Confirmation",
		PlainBody: "Hi " + userName + ",\n\nYour order #" + formatID(orderID) + " has been confirmed. Total: " + formatMoney(total) + "\n\nThank you for shopping with us!",
		HTMLBody:  "<h2>Order Confirmed</h2><p>Hi " + userName + ",</p><p>Your order <strong>#" + formatID(orderID) + "</strong> has been confirmed.</p><p>Total: <strong>" + formatMoney(total) + "</strong></p><p>Thank you for shopping with us!</p>",
	})
	if err != nil {
		s.logger.Error(ctx, "failed to send order confirmation", err, kernel.Field("order_id", orderID), kernel.Field("to", string(to)))
		return err
	}
	return nil
}

func (s *NotificationService) SendShippingUpdate(ctx context.Context, to EmailAddress, userName string, orderID int64, status string) error {
	err := s.sender.Send(ctx, EmailMessage{
		To:        to,
		Subject:   "Shipping Update",
		PlainBody: "Hi " + userName + ",\n\nYour order #" + formatID(orderID) + " has been updated to: " + status + ".\n\nTrack your order on our website.",
		HTMLBody:  "<h2>Shipping Update</h2><p>Hi " + userName + ",</p><p>Your order <strong>#" + formatID(orderID) + "</strong> has been updated to: <strong>" + status + "</strong>.</p><p><a href='#'>Track your order</a></p>",
	})
	if err != nil {
		s.logger.Error(ctx, "failed to send shipping update", err, kernel.Field("order_id", orderID), kernel.Field("to", string(to)))
		return err
	}
	return nil
}

func (s *NotificationService) SendPasswordReset(ctx context.Context, to EmailAddress, userName string, resetURL string) error {
	err := s.sender.Send(ctx, EmailMessage{
		To:        to,
		Subject:   "Password Reset",
		PlainBody: "Hi " + userName + ",\n\nClick the link below to reset your password:\n" + resetURL + "\n\nThis link expires in 1 hour.\nIf you did not request this, please ignore this email.",
		HTMLBody:  "<h2>Password Reset</h2><p>Hi " + userName + ",</p><p>Click the button below to reset your password. This link expires in 1 hour.</p><p><a href='" + resetURL + "' style='background: #007bff; color: #fff; padding: 10px 20px; text-decoration: none; border-radius: 4px;'>Reset Password</a></p><p>If you did not request this, please ignore this email.</p>",
	})
	if err != nil {
		s.logger.Error(ctx, "failed to send password reset", err, kernel.Field("to", string(to)))
		return err
	}
	return nil
}

func (s *NotificationService) SendOrderConfirmationPref(ctx context.Context, userID kernel.ID, to EmailAddress, userName string, orderID int64, total int64) error {
	if !s.allowsEmail(ctx, userID, NotificationTypeOrder) {
		return nil
	}
	return s.SendOrderConfirmation(ctx, to, userName, orderID, total)
}

func (s *NotificationService) SendShippingUpdatePref(ctx context.Context, userID kernel.ID, to EmailAddress, userName string, orderID int64, status string) error {
	if !s.allowsEmail(ctx, userID, NotificationTypeShipping) {
		return nil
	}
	return s.SendShippingUpdate(ctx, to, userName, orderID, status)
}

func (s *NotificationService) SendSubscriptionRenewed(ctx context.Context, userID kernel.ID, to EmailAddress, userName string, subID int64, planName string, amount int64) error {
	if !s.allowsEmail(ctx, userID, NotificationTypeSubscription) {
		return nil
	}
	err := s.sender.Send(ctx, EmailMessage{
		To:        to,
		Subject:   "Subscription Renewed",
		PlainBody: "Hi " + userName + ",\n\nYour " + planName + " subscription (##" + formatID(subID) + ") has been renewed. Amount charged: " + formatMoney(amount) + ".\n\nThank you!",
		HTMLBody:  "<h2>Subscription Renewed</h2><p>Hi " + userName + ",</p><p>Your <strong>" + planName + "</strong> subscription has been renewed.</p><p>Amount charged: <strong>" + formatMoney(amount) + "</strong></p>",
	})
	if err != nil {
		s.logger.Error(ctx, "failed to send subscription renewal", err, kernel.Field("subscription_id", subID), kernel.Field("to", string(to)))
		return err
	}
	return nil
}

func (s *NotificationService) SendPaymentFailed(ctx context.Context, userID kernel.ID, to EmailAddress, userName string, subID int64) error {
	if !s.allowsEmail(ctx, userID, NotificationTypeSubscription) {
		return nil
	}
	err := s.sender.Send(ctx, EmailMessage{
		To:        to,
		Subject:   "Payment Failed — Action Required",
		PlainBody: "Hi " + userName + ",\n\nWe could not charge your subscription (##" + formatID(subID) + "). Please update your payment method to avoid interruption.\n\nThank you!",
		HTMLBody:  "<h2>Payment Failed</h2><p>Hi " + userName + ",</p><p>We could not charge your subscription <strong>#" + formatID(subID) + "</strong>.</p><p>Please <a href='#'>update your payment method</a> to avoid interruption.</p>",
	})
	if err != nil {
		s.logger.Error(ctx, "failed to send payment failed", err, kernel.Field("subscription_id", subID), kernel.Field("to", string(to)))
		return err
	}
	return nil
}

func (s *NotificationService) SendSubscriptionExpired(ctx context.Context, userID kernel.ID, to EmailAddress, userName string, subID int64) error {
	if !s.allowsEmail(ctx, userID, NotificationTypeSubscription) {
		return nil
	}
	err := s.sender.Send(ctx, EmailMessage{
		To:        to,
		Subject:   "Subscription Expired",
		PlainBody: "Hi " + userName + ",\n\nYour subscription (##" + formatID(subID) + ") has expired.\n\nYou can resubscribe at any time.",
		HTMLBody:  "<h2>Subscription Expired</h2><p>Hi " + userName + ",</p><p>Your subscription <strong>#" + formatID(subID) + "</strong> has expired.</p><p>You can <a href='#'>resubscribe</a> at any time.</p>",
	})
	if err != nil {
		s.logger.Error(ctx, "failed to send subscription expired", err, kernel.Field("subscription_id", subID), kernel.Field("to", string(to)))
		return err
	}
	return nil
}

func (s *NotificationService) SendRefundProcessed(ctx context.Context, userID kernel.ID, to EmailAddress, userName string, orderID int64, amount int64) error {
	if !s.allowsEmail(ctx, userID, NotificationTypeRefund) {
		return nil
	}
	err := s.sender.Send(ctx, EmailMessage{
		To:        to,
		Subject:   "Refund Processed",
		PlainBody: "Hi " + userName + ",\n\nA refund of " + formatMoney(amount) + " has been processed for your order #" + formatID(orderID) + ".\n\nThank you!",
		HTMLBody:  "<h2>Refund Processed</h2><p>Hi " + userName + ",</p><p>A refund of <strong>" + formatMoney(amount) + "</strong> has been processed for your order <strong>#" + formatID(orderID) + "</strong>.</p>",
	})
	if err != nil {
		s.logger.Error(ctx, "failed to send refund processed", err, kernel.Field("order_id", orderID), kernel.Field("to", string(to)))
		return err
	}
	return nil
}

func formatID(id int64) string {
	abs := id
	prefix := ""
	if id < 0 {
		prefix = "-"
		abs = -id
	}
	s := formatInt64(abs)
	if len(s) <= 3 {
		return prefix + s
	}
	var result []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, c)
	}
	return prefix + string(result)
}

func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		d := n % 10
		digits = string(rune('0'+d)) + digits
		n /= 10
	}
	if neg {
		digits = "-" + digits
	}
	return digits
}

func formatMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	dollars := cents / 100
	remaining := cents % 100
	s := formatID(dollars) + "."
	if remaining < 10 {
		s += "0"
	}
	s += formatInt64(remaining)
	return sign + "$" + s
}
