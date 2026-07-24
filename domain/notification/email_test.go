package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

var errMockFailure = errors.New("mock sender failure")

type mockEmailSender struct {
	sent   []EmailMessage
	failOn int
}

func (m *mockEmailSender) Send(_ context.Context, msg EmailMessage) error {
	m.sent = append(m.sent, msg)
	if m.failOn > 0 && len(m.sent) >= m.failOn {
		return errMockFailure
	}
	return nil
}

type discardLogger struct{}

func (discardLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (discardLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (discardLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (discardLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func newTestService(sender EmailSender) *NotificationService {
	return NewNotificationService(sender, discardLogger{})
}

func TestSendOrderConfirmation(t *testing.T) {
	mock := &mockEmailSender{}
	svc := newTestService(mock)

	err := svc.SendOrderConfirmation(context.Background(), "alice@example.com", "Alice", 12345, 2999)
	if err != nil {
		t.Fatalf("SendOrderConfirmation: %v", err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(mock.sent))
	}
	msg := mock.sent[0]
	if msg.To != "alice@example.com" {
		t.Errorf("To = %q", msg.To)
	}
	if msg.Subject != "Order Confirmation" {
		t.Errorf("Subject = %q", msg.Subject)
	}
}

func TestSendOrderConfirmation_IncludesOrderInfo(t *testing.T) {
	mock := &mockEmailSender{}
	svc := newTestService(mock)

	svc.SendOrderConfirmation(context.Background(), "user@test.com", "TestUser", 98765, 5000)
	msg := mock.sent[0]

	if len(msg.PlainBody) == 0 {
		t.Error("expected non-empty plain body")
	}
	if len(msg.HTMLBody) == 0 {
		t.Error("expected non-empty HTML body")
	}
}

func TestSendShippingUpdate(t *testing.T) {
	mock := &mockEmailSender{}
	svc := newTestService(mock)

	err := svc.SendShippingUpdate(context.Background(), "bob@example.com", "Bob", 123, "shipped")
	if err != nil {
		t.Fatalf("SendShippingUpdate: %v", err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(mock.sent))
	}
	msg := mock.sent[0]
	if msg.To != "bob@example.com" {
		t.Errorf("To = %q", msg.To)
	}
	if msg.Subject != "Shipping Update" {
		t.Errorf("Subject = %q", msg.Subject)
	}
}

func TestSendShippingUpdate_Delivered(t *testing.T) {
	mock := &mockEmailSender{}
	svc := newTestService(mock)

	svc.SendShippingUpdate(context.Background(), "carol@example.com", "Carol", 456, "delivered")
	msg := mock.sent[0]

	if len(msg.PlainBody) == 0 {
		t.Error("expected non-empty plain body")
	}
}

func TestSendPasswordReset(t *testing.T) {
	mock := &mockEmailSender{}
	svc := newTestService(mock)

	err := svc.SendPasswordReset(context.Background(), "dave@example.com", "Dave", "https://example.com/reset?token=abc123")
	if err != nil {
		t.Fatalf("SendPasswordReset: %v", err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(mock.sent))
	}
	msg := mock.sent[0]
	if msg.To != "dave@example.com" {
		t.Errorf("To = %q", msg.To)
	}
	if msg.Subject != "Password Reset" {
		t.Errorf("Subject = %q", msg.Subject)
	}
}

func TestSendPasswordReset_IncludesResetURL(t *testing.T) {
	mock := &mockEmailSender{}
	svc := newTestService(mock)

	svc.SendPasswordReset(context.Background(), "eve@example.com", "Eve", "https://example.com/reset?token=xyz789")
	msg := mock.sent[0]

	if len(msg.PlainBody) == 0 {
		t.Error("expected non-empty plain body")
	}
}

func TestSendOrderConfirmation_SenderFailure(t *testing.T) {
	mock := &mockEmailSender{failOn: 1}
	svc := newTestService(mock)

	err := svc.SendOrderConfirmation(context.Background(), "fail@example.com", "Fail", 1, 100)
	if err == nil {
		t.Fatal("expected error from sender failure")
	}
}

func TestEmailMessage_Fields(t *testing.T) {
	msg := EmailMessage{
		To:        "test@example.com",
		Subject:   "Test",
		PlainBody: "Hello",
		HTMLBody:  "<p>Hello</p>",
	}
	if string(msg.To) != "test@example.com" {
		t.Errorf("To = %q", msg.To)
	}
	if msg.Subject != "Test" {
		t.Errorf("Subject = %q", msg.Subject)
	}
}

func TestEmailAddress(t *testing.T) {
	addr := EmailAddress("user@domain.com")
	if string(addr) != "user@domain.com" {
		t.Errorf("EmailAddress = %q", addr)
	}
}

func TestFormatID(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{12, "12"},
		{123, "123"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{-1234, "-1,234"},
	}
	for _, tt := range tests {
		got := formatID(tt.input)
		if got != tt.want {
			t.Errorf("formatID(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "$0.00"},
		{1, "$0.01"},
		{10, "$0.10"},
		{100, "$1.00"},
		{150, "$1.50"},
		{1000, "$10.00"},
		{12345, "$123.45"},
		{1234567, "$12,345.67"},
		{-100, "-$1.00"},
		{-12345, "-$123.45"},
	}
	for _, tt := range tests {
		got := formatMoney(tt.input)
		if got != tt.want {
			t.Errorf("formatMoney(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{123, "123"},
		{-1, "-1"},
	}
	for _, tt := range tests {
		got := formatInt64(tt.input)
		if got != tt.want {
			t.Errorf("formatInt64(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
