package notification

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	domain "github.com/beeleelee/mall/domain/notification"
)

func TestOrderEventJSON(t *testing.T) {
	data := []byte(`{"order_id": 123, "user_id": 456, "status": "confirmed"}`)
	var evt orderEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if evt.OrderID != 123 {
		t.Errorf("OrderID = %d, want 123", evt.OrderID)
	}
	if evt.UserID != 456 {
		t.Errorf("UserID = %d, want 456", evt.UserID)
	}
	if evt.Status != "confirmed" {
		t.Errorf("Status = %q, want confirmed", evt.Status)
	}
}

func TestOrderEventJSON_UnknownFieldsIgnored(t *testing.T) {
	data := []byte(`{"order_id": 1, "user_id": 2, "status": "shipped", "extra": "ignored"}`)
	var evt orderEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if evt.OrderID != 1 {
		t.Errorf("OrderID = %d", evt.OrderID)
	}
}

func TestSMTPConfig(t *testing.T) {
	cfg := SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "user",
		Password: "pass",
		From:     "noreply@example.com",
	}
	if cfg.Host != "smtp.example.com" {
		t.Errorf("Host = %q", cfg.Host)
	}
	if cfg.Port != 587 {
		t.Errorf("Port = %d", cfg.Port)
	}
}

func TestSMTPEmailSender_ConnectRefused(t *testing.T) {
	sender := NewSMTPEmailSender(SMTPConfig{
		Host: "127.0.0.1",
		Port: 1,
		From: "test@example.com",
	})

	err := sender.Send(context.Background(), domain.EmailMessage{
		To:        "recipient@example.com",
		Subject:   "Test",
		PlainBody: "Hello",
	})
	if err == nil {
		t.Fatal("expected error on connection refused")
	}
}

func TestSMTPEmailSender_MessageFormatting(t *testing.T) {
	sender := NewSMTPEmailSender(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		From: "noreply@example.com",
	})

	msgBytes := sender.buildMessage(domain.EmailMessage{
		To:        "user@example.com",
		Subject:   "Test Subject",
		PlainBody: "Hello there",
	})

	lines := strings.Split(string(msgBytes), "\r\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}

	header := strings.Join(lines[:3], "\r\n")
	if !strings.Contains(header, "From: noreply@example.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(header, "To: user@example.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(header, "Subject: Test Subject") {
		t.Error("missing Subject header")
	}

	body := strings.Join(lines[4:], "\r\n")
	if body != "Hello there" {
		t.Errorf("body = %q, want %q", body, "Hello there")
	}
}

func TestSMTPEmailSender_MessageFormatting_HTML(t *testing.T) {
	sender := NewSMTPEmailSender(SMTPConfig{
		From: "from@example.com",
	})

	msgBytes := sender.buildMessage(domain.EmailMessage{
		To:        "to@example.com",
		Subject:   "HTML Test",
		HTMLBody:  "<h1>Hello</h1>",
	})

	body := string(msgBytes)
	if !strings.Contains(body, "Content-Type: text/html") {
		t.Error("expected HTML content type")
	}
	if !strings.Contains(body, "<h1>Hello</h1>") {
		t.Error("expected HTML body content")
	}
}

func TestSMTPEmailSender_MessageFormatting_EmptyConfigFrom(t *testing.T) {
	sender := NewSMTPEmailSender(SMTPConfig{
		Host: "smtp.example.com",
		Port: 25,
	})

	msgBytes := sender.buildMessage(domain.EmailMessage{
		To:        "recipient@example.com",
		Subject:   "Empty From",
		PlainBody: "Body",
	})
	if !strings.Contains(string(msgBytes), "Subject: Empty From") {
		t.Error("expected Subject header")
	}
}
