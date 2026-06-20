package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(10, 5)
	defer rl.Stop()

	for i := 0; i < 5; i++ {
		if !rl.allow("127.0.0.1") {
			t.Fatalf("expected allow at attempt %d", i+1)
		}
	}

	if rl.allow("127.0.0.1") {
		t.Fatal("expected deny after burst exhausted")
	}
}

func TestRateLimiter_MultipleIPs(t *testing.T) {
	rl := NewRateLimiter(10, 3)
	defer rl.Stop()

	for i := 0; i < 3; i++ {
		if !rl.allow("10.0.0.1") {
			t.Fatalf("expected allow for ip1 at attempt %d", i+1)
		}
	}

	for i := 0; i < 3; i++ {
		if !rl.allow("10.0.0.2") {
			t.Fatalf("expected allow for ip2 at attempt %d", i+1)
		}
	}

	if rl.allow("10.0.0.1") {
		t.Fatal("expected deny for ip1 after burst")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(100, 2)
	defer rl.Stop()

	mw := RateLimitMiddleware(rl)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 at attempt %d, got %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}
