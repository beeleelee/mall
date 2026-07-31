package subscription

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeProcessor struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (f *fakeProcessor) ProcessDueBilling(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.err
}

type workerLogger struct{}

func (workerLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (workerLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (workerLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (workerLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func TestBillingWorker_RunsPeriodically(t *testing.T) {
	p := &fakeProcessor{}
	worker := NewSubscriptionBillingWorker(p, 10*time.Millisecond, workerLogger{})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()
	<-done

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calls == 0 {
		t.Fatal("expected processor to be called at least once")
	}
}

func TestBillingWorker_LogsError(t *testing.T) {
	p := &fakeProcessor{err: errors.New("boom")}
	worker := NewSubscriptionBillingWorker(p, 10*time.Millisecond, workerLogger{})

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()
	<-done

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calls == 0 {
		t.Fatal("expected processor to be called at least once")
	}
}
