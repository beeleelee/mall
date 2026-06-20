package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func StartDomainSpan(ctx context.Context, domainName, operation string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.Tracer("mall.domain." + domainName)
	ctx, span := tracer.Start(ctx, operation, trace.WithAttributes(attrs...))
	return ctx, span
}
