package notification

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type EmailAddress string

type EmailMessage struct {
	To         EmailAddress
	Subject    string
	PlainBody  string
	HTMLBody   string
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

type NotificationService struct {
	sender EmailSender
	logger kernel.Logger
}

func NewNotificationService(sender EmailSender, logger kernel.Logger) *NotificationService {
	return &NotificationService{sender: sender, logger: logger}
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

func formatID(id int64) string {
	s := ""
	for i, c := range []byte(formatInt64(id)) {
		if i > 0 && i%3 == 0 {
			s = "," + s
		}
		s = string(c) + s
	}
	return s
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
	s := formatInt64(dollars) + "."
	if remaining < 10 {
		s += "0"
	}
	s += formatInt64(remaining)
	return sign + "$" + s
}
