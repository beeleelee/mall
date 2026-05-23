package kernel

import "context"

type LogField struct {
	Key   string
	Value any
}

type Logger interface {
	Debug(ctx context.Context, msg string, fields ...LogField)
	Info(ctx context.Context, msg string, fields ...LogField)
	Warn(ctx context.Context, msg string, fields ...LogField)
	Error(ctx context.Context, msg string, err error, fields ...LogField)
}

func Field(key string, value any) LogField {
	return LogField{Key: key, Value: value}
}

func Fields(m map[string]any) []LogField {
	fields := make([]LogField, 0, len(m))
	for k, v := range m {
		fields = append(fields, Field(k, v))
	}
	return fields
}
