package tracing

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func InjectTrace(ctx context.Context, msg *nats.Msg) {
	if msg.Header == nil {
		msg.Header = nats.Header{}
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(msg.Header))
}

func ExtractTrace(msg *nats.Msg) context.Context {
	return otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(msg.Header))
}

func ExtractFromJetStream(msg jetstream.Msg) context.Context {
	return otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(msg.Headers()))
}
