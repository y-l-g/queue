package queue

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

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
