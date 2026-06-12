package queue

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"
)

type fakeWorkers struct {
	messages chan any
}

func newFakeWorkers(size int) *fakeWorkers {
	return &fakeWorkers{messages: make(chan any, size)}
}

func (w *fakeWorkers) SendRequest(http.ResponseWriter, *http.Request) error {
	return nil
}

func (w *fakeWorkers) SendMessage(_ context.Context, message any, _ http.ResponseWriter) (any, error) {
	w.messages <- message
	return nil, nil
}

func (w *fakeWorkers) NumThreads() int {
	return 1
}

func TestManagerDoesNotReserveBeforeStart(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend(
		backendConfig{MaxMessages: 10, MaxTotalBytes: 1 << 20},
		[]string{"default"},
		defaultMaxMessageBytes,
		time.Minute,
		3,
	)
	if _, code, err := backend.Enqueue(ctx, "default", "payload", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("enqueue failed: code=%d err=%v", code, err)
	}

	manager := newManager(
		backend,
		newFakeWorkers(1),
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
		[]string{"default"},
		"consumer",
		1,
		10*time.Millisecond,
		time.Minute,
		time.Second,
		3,
		defaultMaxMessageBytes,
	)
	defer manager.shutdown()

	time.Sleep(50 * time.Millisecond)

	stats, err := backend.Stats(ctx, "default")
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.Pending != 1 || stats.Reserved != 0 {
		t.Fatalf("manager reserved before start: pending=%d reserved=%d", stats.Pending, stats.Reserved)
	}

	status := manager.status(ctx, "")
	if status.Ready {
		t.Fatal("manager reported ready before start")
	}

	if _, code, err := manager.enqueue(ctx, "default", "second", 0); err == nil || code != dispatchResultWorkerUnavailable {
		t.Fatalf("expected worker unavailable before start, got code=%d err=%v", code, err)
	}
}

func TestManagerReservesAfterStart(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend(
		backendConfig{MaxMessages: 10, MaxTotalBytes: 1 << 20},
		[]string{"default"},
		defaultMaxMessageBytes,
		time.Minute,
		3,
	)
	if _, code, err := backend.Enqueue(ctx, "default", "payload", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("enqueue failed: code=%d err=%v", code, err)
	}

	workers := newFakeWorkers(1)
	manager := newManager(
		backend,
		workers,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
		[]string{"default"},
		"consumer",
		1,
		10*time.Millisecond,
		time.Minute,
		time.Second,
		3,
		defaultMaxMessageBytes,
	)
	defer manager.shutdown()

	manager.start()

	select {
	case <-workers.messages:
	case <-time.After(time.Second):
		t.Fatal("manager did not deliver reserved message after start")
	}

	status := manager.status(ctx, "")
	if !status.Ready {
		t.Fatal("manager did not report ready after start")
	}
}
