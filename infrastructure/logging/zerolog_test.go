package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"

	"github.com/beeleelee/mall/domain/kernel"
)

func parseLogLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("parse log line: %v", err)
	}
	return m
}

func TestZerologLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).With().Timestamp().Str("service", "test-svc").Logger()
	logger := &ZerologLogger{log: l, svcName: "test-svc"}

	ctx := context.Background()
	logger.Info(ctx, "hello world", kernel.Field("key1", "val1"))

	line := buf.String()
	m := parseLogLine(t, line)

	if m["message"] != "hello world" {
		t.Errorf("expected message 'hello world', got %v", m["message"])
	}
	if m["level"] != "info" {
		t.Errorf("expected level info, got %v", m["level"])
	}
	if m["service"] != "test-svc" {
		t.Errorf("expected service test-svc, got %v", m["service"])
	}
	if m["key1"] != "val1" {
		t.Errorf("expected key1=val1, got %v", m["key1"])
	}
}

func TestZerologLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).With().Timestamp().Logger()
	logger := &ZerologLogger{log: l, svcName: "test"}

	ctx := context.Background()
	logger.Debug(ctx, "debug msg")

	line := buf.String()
	m := parseLogLine(t, line)
	if m["message"] != "debug msg" {
		t.Errorf("expected 'debug msg', got %v", m["message"])
	}
}

func TestZerologLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).With().Timestamp().Logger()
	logger := &ZerologLogger{log: l, svcName: "test"}

	ctx := context.Background()
	logger.Warn(ctx, "warn msg")

	line := buf.String()
	m := parseLogLine(t, line)
	if m["level"] != "warn" {
		t.Errorf("expected level warn, got %v", m["level"])
	}
}

func TestZerologLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).With().Timestamp().Logger()
	logger := &ZerologLogger{log: l, svcName: "test"}

	ctx := context.Background()
	err := kernel.NewDomainError(kernel.ErrNotFound, "test error")
	logger.Error(ctx, "error msg", err)

	line := buf.String()
	m := parseLogLine(t, line)
	if m["message"] != "error msg" {
		t.Errorf("expected 'error msg', got %v", m["message"])
	}
	if m["error"] == nil {
		t.Error("expected error field")
	}
}

func TestZerologLogger_WithCapability(t *testing.T) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).With().Timestamp().Str("service", "test").Logger()
	base := &ZerologLogger{log: l, svcName: "test"}

	capped := base.WithCapability("dev.ucp.shopping.catalog")
	ctx := context.Background()
	capped.Info(ctx, "capped msg")

	line := buf.String()
	m := parseLogLine(t, line)
	if m["capability"] != "dev.ucp.shopping.catalog" {
		t.Errorf("expected capability field, got %v", m["capability"])
	}
}

func TestKernelFields(t *testing.T) {
	fields := []kernel.LogField{
		{Key: "key_a", Value: "val_a"},
		{Key: "key_b", Value: "42"},
	}

	m := kernelFields(fields)
	if m["key_a"] != "val_a" {
		t.Errorf("expected val_a, got %v", m["key_a"])
	}
	if v, ok := m["key_b"]; !ok || v != "42" {
		t.Errorf("expected 42, got %v", m["key_b"])
	}
}
