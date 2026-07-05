package notification

import (
	"context"
	"fmt"
	"net/smtp"

	domain "github.com/beeleelee/mall/domain/notification"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPEmailSender struct {
	config SMTPConfig
}

func NewSMTPEmailSender(config SMTPConfig) *SMTPEmailSender {
	return &SMTPEmailSender{config: config}
}

func (s *SMTPEmailSender) Send(ctx context.Context, msg domain.EmailMessage) error {
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	body := msg.PlainBody
	if msg.HTMLBody != "" {
		body = msg.HTMLBody
		contentType := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
		body = contentType + body
	}

	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n", s.config.From, string(msg.To), msg.Subject)

	msgBytes := []byte(header + "\r\n" + body)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	return smtp.SendMail(addr, auth, s.config.From, []string{string(msg.To)}, msgBytes)
}
