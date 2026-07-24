package storage

import (
	"strings"
	"testing"
)

func TestMinIOStorage_URL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		bucket   string
		prefix   string
		key      string
		useSSL   bool
		want     string
	}{
		{
			name:     "http no prefix",
			endpoint: "localhost:9000",
			bucket:   "mall",
			prefix:   "",
			key:      "images/1.jpg",
			useSSL:   false,
			want:     "http://localhost:9000/mall/images/1.jpg",
		},
		{
			name:     "https with prefix",
			endpoint: "s3.amazonaws.com",
			bucket:   "my-bucket",
			prefix:   "prod/",
			key:      "images/1.jpg",
			useSSL:   true,
			want:     "https://s3.amazonaws.com/my-bucket/prod/images/1.jpg",
		},
		{
			name:     "http with trailing slash prefix",
			endpoint: "play.min.io",
			bucket:   "uploads",
			prefix:   "user_uploads/",
			key:      "avatar.png",
			useSSL:   false,
			want:     "http://play.min.io/uploads/user_uploads/avatar.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &MinIOStorage{
				endpoint: tt.endpoint,
				bucket:   tt.bucket,
				prefix:   tt.prefix,
				useSSL:   tt.useSSL,
			}
			got := s.URL(tt.key)
			if got != tt.want {
				t.Errorf("URL(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestMinIOStorage_URL_EmptyKey(t *testing.T) {
	s := &MinIOStorage{
		endpoint: "localhost:9000",
		bucket:   "test",
		prefix:   "prefix/",
		useSSL:   false,
	}
	url := s.URL("")
	if !strings.HasSuffix(url, "/test/prefix/") {
		t.Errorf("unexpected URL: %s", url)
	}
}

func TestMinIOStorage_NewMinIOStorage_InvalidEndpoint(t *testing.T) {
	_, err := NewMinIOStorage("127.0.0.1:1", "key", "secret", "bucket", "", false)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}
