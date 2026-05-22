package queue

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
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

func TestMemoryBackendLifecycle(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend(
		backendConfig{MaxMessages: 10, MaxTotalBytes: 1 << 20},
		[]string{"default"},
		defaultMaxMessageBytes,
		time.Minute,
		3,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)

	id, code, err := backend.Enqueue(ctx, "default", "payload", 0)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if code != dispatchResultAccepted || id == "" {
		t.Fatalf("unexpected enqueue result: id=%q code=%d", id, code)
	}

	delivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if delivery.Payload != "payload" || delivery.Attempts != 1 {
		t.Fatalf("unexpected delivery: %#v", delivery)
	}

	code, err = backend.Release(ctx, "default", delivery.ID, 0)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("release failed: code=%d err=%v", code, err)
	}

	delivery, err = backend.Reserve(ctx, []string{"default"}, "consumer", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("reserve after release failed: %v", err)
	}
	if delivery.Attempts != 2 {
		t.Fatalf("expected second attempt, got %d", delivery.Attempts)
	}

	code, err = backend.Ack(ctx, "default", delivery.ID)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("ack failed: code=%d err=%v", code, err)
	}

	stats, err := backend.Stats(ctx, "default")
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.Pending != 0 || stats.Reserved != 0 {
		t.Fatalf("expected empty queue, got %#v", stats)
	}
}

func TestMemoryBackendRejectsOversizedPayload(t *testing.T) {
	backend := newMemoryBackend(
		backendConfig{MaxMessages: 10, MaxTotalBytes: 1 << 20},
		[]string{"default"},
		4,
		time.Minute,
		3,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)

	_, code, err := backend.Enqueue(context.Background(), "default", "payload", 0)
	if err == nil {
		t.Fatal("expected oversized payload error")
	}
	if code != dispatchResultPayloadTooLarge {
		t.Fatalf("expected payload-too-large status, got %d", code)
	}
}

func TestManagerDoesNotReserveBeforeStart(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend(
		backendConfig{MaxMessages: 10, MaxTotalBytes: 1 << 20},
		[]string{"default"},
		defaultMaxMessageBytes,
		time.Minute,
		3,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
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
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
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

func TestRedisBackendLifecycleWhenConfigured(t *testing.T) {
	redisURL := os.Getenv("POGO_REDIS_URL")
	if redisURL == "" {
		t.Skip("POGO_REDIS_URL is not set")
	}

	ctx := context.Background()
	backend, err := newRedisBackend(
		backendConfig{
			RedisURL:  redisURL,
			KeyPrefix: "pogo-test",
			Group:     "test",
			Consumer:  "test-consumer",
		},
		[]string{"default"},
		defaultMaxMessageBytes,
		100*time.Millisecond,
		3,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)
	if err != nil {
		t.Fatalf("redis backend init failed: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.client.Del(ctx, backend.streamKey("default"), backend.delayedKey("default"), backend.failedKey("default")).Err()
		_ = backend.Close()
	})
	if err := backend.Start(ctx); err != nil {
		t.Fatalf("redis backend start failed: %v", err)
	}

	if _, code, err := backend.Enqueue(ctx, "default", "payload", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue failed: code=%d err=%v", code, err)
	}

	delivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis reserve failed: %v", err)
	}
	if delivery.Payload != "payload" {
		t.Fatalf("unexpected redis payload: %#v", delivery)
	}
	if code, err := backend.Ack(ctx, "default", delivery.ID); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis ack failed: code=%d err=%v", code, err)
	}
}

func TestRedisPromoteDelayedIsAtomicWhenConfigured(t *testing.T) {
	redisURL := os.Getenv("POGO_REDIS_URL")
	if redisURL == "" {
		t.Skip("POGO_REDIS_URL is not set")
	}

	ctx := context.Background()
	backendA, err := newRedisBackend(
		backendConfig{
			RedisURL:  redisURL,
			KeyPrefix: "pogo-test-promote-atomic",
			Group:     "test",
			Consumer:  "test-consumer-a",
		},
		[]string{"default"},
		defaultMaxMessageBytes,
		100*time.Millisecond,
		3,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)
	if err != nil {
		t.Fatalf("redis backend init failed: %v", err)
	}
	backendB, err := newRedisBackend(
		backendConfig{
			RedisURL:  redisURL,
			KeyPrefix: "pogo-test-promote-atomic",
			Group:     "test",
			Consumer:  "test-consumer-b",
		},
		[]string{"default"},
		defaultMaxMessageBytes,
		100*time.Millisecond,
		3,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)
	if err != nil {
		_ = backendA.Close()
		t.Fatalf("redis backend init failed: %v", err)
	}

	keys := []string{backendA.streamKey("default"), backendA.delayedKey("default"), backendA.failedKey("default")}
	if err := backendA.client.Del(ctx, keys...).Err(); err != nil {
		t.Fatalf("redis cleanup failed: %v", err)
	}
	t.Cleanup(func() {
		_ = backendA.client.Del(ctx, keys...).Err()
		_ = backendA.Close()
		_ = backendB.Close()
	})

	if err := backendA.Start(ctx); err != nil {
		t.Fatalf("redis backend start failed: %v", err)
	}
	if err := backendB.Start(ctx); err != nil {
		t.Fatalf("redis backend start failed: %v", err)
	}

	body, err := json.Marshal(delayedPayload{
		ID:       "delayed-atomic-test",
		Queue:    "default",
		Payload:  "payload",
		Attempts: 1,
	})
	if err != nil {
		t.Fatalf("marshal delayed payload failed: %v", err)
	}
	if err := backendA.client.ZAdd(ctx, backendA.delayedKey("default"), redis.Z{
		Score:  float64(time.Now().Add(-time.Second).UnixMilli()),
		Member: string(body),
	}).Err(); err != nil {
		t.Fatalf("seed delayed payload failed: %v", err)
	}

	const promoters = 50
	start := make(chan struct{})
	errs := make(chan error, promoters)
	var wg sync.WaitGroup
	for i := 0; i < promoters; i++ {
		backend := backendA
		if i%2 == 1 {
			backend = backendB
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := backend.promoteDelayed(ctx, "default", 100); err != nil {
				errs <- err
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("promote delayed failed: %v", err)
	}

	streamLen, err := backendA.client.XLen(ctx, backendA.streamKey("default")).Result()
	if err != nil {
		t.Fatalf("read stream length failed: %v", err)
	}
	if streamLen != 1 {
		t.Fatalf("expected one promoted stream message, got %d", streamLen)
	}

	delayedLen, err := backendA.client.ZCard(ctx, backendA.delayedKey("default")).Result()
	if err != nil {
		t.Fatalf("read delayed length failed: %v", err)
	}
	if delayedLen != 0 {
		t.Fatalf("expected delayed set to be empty, got %d", delayedLen)
	}
}

func TestRedisReserveReportsPromoteDelayedErrorsWhenConfigured(t *testing.T) {
	redisURL := os.Getenv("POGO_REDIS_URL")
	if redisURL == "" {
		t.Skip("POGO_REDIS_URL is not set")
	}

	ctx := context.Background()
	backend, err := newRedisBackend(
		backendConfig{
			RedisURL:  redisURL,
			KeyPrefix: "pogo-test-promote-error",
			Group:     "test",
			Consumer:  "test-consumer",
		},
		[]string{"default"},
		defaultMaxMessageBytes,
		100*time.Millisecond,
		3,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)
	if err != nil {
		t.Fatalf("redis backend init failed: %v", err)
	}

	keys := []string{backend.streamKey("default"), backend.delayedKey("default"), backend.failedKey("default")}
	if err := backend.client.Del(ctx, keys...).Err(); err != nil {
		t.Fatalf("redis cleanup failed: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.client.Del(ctx, keys...).Err()
		_ = backend.Close()
	})

	if err := backend.Start(ctx); err != nil {
		t.Fatalf("redis backend start failed: %v", err)
	}
	if err := backend.client.Set(ctx, backend.delayedKey("default"), "wrong-type", 0).Err(); err != nil {
		t.Fatalf("seed invalid delayed key failed: %v", err)
	}

	_, err = backend.Reserve(ctx, []string{"default"}, "consumer", 0)
	if err == nil {
		t.Fatal("expected reserve to report delayed promotion error")
	}
	if errors.Is(err, errQueueEmpty) {
		t.Fatalf("expected backend error, got queue empty: %v", err)
	}
	if got := backend.stats.backendErrors.Load(); got != 1 {
		t.Fatalf("expected one backend error, got %d", got)
	}
}
