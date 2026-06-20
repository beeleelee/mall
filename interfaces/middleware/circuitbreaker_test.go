package middleware

import (
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	if !cb.Allow() {
		t.Fatal("expected allow when closed")
	}

	cb.Failure()
	cb.Failure()
	cb.Failure()

	if cb.Allow() {
		t.Fatal("expected deny when open")
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker(2, 2, 50*time.Millisecond)

	cb.Failure()
	cb.Failure()

	if cb.Allow() {
		t.Fatal("expected deny")
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("expected allow half-open")
	}

	cb.Success()
	cb.Success()

	if cb.State() != StateClosed {
		t.Fatal("expected closed after threshold successes")
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 2, 50*time.Millisecond)

	cb.Failure()
	cb.Failure()

	time.Sleep(60 * time.Millisecond)

	cb.Allow()
	cb.Failure()

	if cb.State() != StateOpen {
		t.Fatal("expected open after failure in half-open")
	}
}
