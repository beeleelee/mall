package kernel

import (
	"context"
	"io"
)

type StorageService interface {
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) (url string, err error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}
