package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/beeleelee/mall/domain/kernel"
)

type MinIOStorage struct {
	client *minio.Client
	bucket string
	prefix string
	endpoint string
	useSSL   bool
}

func NewMinIOStorage(endpoint, accessKey, secretKey, bucket, prefix string, useSSL bool) (*MinIOStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	ctx := context.Background()
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		exists, errExists := client.BucketExists(ctx, bucket)
		if errExists != nil || !exists {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}

	return &MinIOStorage{
		client:   client,
		bucket:   bucket,
		prefix:   prefix,
		endpoint: endpoint,
		useSSL:   useSSL,
	}, nil
}

func (s *MinIOStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error) {
	fullKey := s.prefix + key
	_, err := s.client.PutObject(ctx, s.bucket, fullKey, reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("upload to minio: %w", err)
	}
	return s.URL(key), nil
}

func (s *MinIOStorage) Delete(ctx context.Context, key string) error {
	fullKey := s.prefix + key
	return s.client.RemoveObject(ctx, s.bucket, fullKey, minio.RemoveObjectOptions{})
}

func (s *MinIOStorage) URL(key string) string {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s%s", scheme, s.endpoint, s.bucket, s.prefix, key)
}

var _ kernel.StorageService = (*MinIOStorage)(nil)
