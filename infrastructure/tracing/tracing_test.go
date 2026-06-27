package tracing

import (
	"context"
	"net/http"
	"testing"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

func TestInjectExtractTrace(t *testing.T) {
	msg := nats.NewMsg("test.subject")

	tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	sid, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid,
		SpanID:  sid,
		Remote:  true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	InjectTrace(ctx, msg)
	if len(msg.Header) == 0 {
		t.Fatal("expected headers after InjectTrace")
	}

	extracted := ExtractTrace(msg)
	if extracted == nil {
		t.Fatal("expected non-nil context after ExtractTrace")
	}

	extractedSC := trace.SpanContextFromContext(extracted)
	if !extractedSC.IsValid() {
		t.Fatal("expected valid span context after round-trip")
	}
	if extractedSC.TraceID() != tid {
		t.Errorf("trace ID round-trip: expected %s, got %s", tid.String(), extractedSC.TraceID().String())
	}
}

func TestInjectTrace_NoHeader(t *testing.T) {
	msg := &nats.Msg{Subject: "test", Header: nil}
	ctx := context.Background()

	InjectTrace(ctx, msg)
	if msg.Header == nil {
		t.Fatal("expected header map to be initialized")
	}
}

func TestExtractTrace_NoHeaders(t *testing.T) {
	msg := &nats.Msg{Subject: "test", Header: nats.Header{}}
	ctx := ExtractTrace(msg)
	if ctx == nil {
		t.Fatal("expected non-nil context, not error")
	}
}

func TestTracingMiddleware(t *testing.T) {
	handler := TracingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context() == nil {
			t.Error("expected non-nil context")
		}
	}))

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler.ServeHTTP(nil, req)
}

func TestStartDomainSpan(t *testing.T) {
	ctx := context.Background()
	_, span := StartDomainSpan(ctx, "test-domain", "test-operation")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	span.End()
}
