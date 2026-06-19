package logging

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/beeleelee/mall/domain/kernel"
	"go.opentelemetry.io/otel/trace"
)

type ZerologLogger struct {
	log    zerolog.Logger
	svcName string
}

func NewZerologLogger(serviceName string) *ZerologLogger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339Nano,
		NoColor:    true,
	}
	if os.Getenv("DEV") != "" {
		output.NoColor = false
	}

	w := io.Writer(output)
	if os.Getenv("JSON_LOG") != "" {
		w = os.Stdout
	}

	l := zerolog.New(w).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Str("service", serviceName).
		Logger()

	if os.Getenv("LOG_LEVEL") != "" {
		lvl, err := zerolog.ParseLevel(os.Getenv("LOG_LEVEL"))
		if err == nil {
			l = l.Level(lvl)
		}
	}

	return &ZerologLogger{log: l, svcName: serviceName}
}

func (l *ZerologLogger) Debug(ctx context.Context, msg string, fields ...kernel.LogField) {
	l.log.Debug().Ctx(ctx).Fields(kernelFields(fields)).Msg(msg)
}

func (l *ZerologLogger) Info(ctx context.Context, msg string, fields ...kernel.LogField) {
	l.log.Info().Ctx(ctx).Fields(kernelFields(fields)).Msg(msg)
}

func (l *ZerologLogger) Warn(ctx context.Context, msg string, fields ...kernel.LogField) {
	l.log.Warn().Ctx(ctx).Fields(kernelFields(fields)).Msg(msg)
}

func (l *ZerologLogger) Error(ctx context.Context, msg string, err error, fields ...kernel.LogField) {
	evt := l.log.Error().Ctx(ctx).Err(err).Fields(kernelFields(fields))
	evt.Msg(msg)
}

func kernelFields(fields []kernel.LogField) map[string]any {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	return m
}

func LogWithTraceID(ctx context.Context, log zerolog.Logger) zerolog.Logger {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return log.With().
			Str("trace_id", sc.TraceID().String()).
			Str("span_id", sc.SpanID().String()).
			Logger()
	}
	return log
}
