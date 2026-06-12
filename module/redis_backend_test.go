package queue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRedisBackendLifecycleWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test")

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
	if code, err := backend.Release(ctx, "default", delivery.ID, 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis release failed: code=%d err=%v", code, err)
	}

	delivery, err = backend.Reserve(ctx, []string{"default"}, "consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis reserve after release failed: %v", err)
	}
	if delivery.Payload != "payload" || delivery.Attempts != 2 {
		t.Fatalf("unexpected redis delivery after release: %#v", delivery)
	}
	if code, err := backend.Ack(ctx, "default", delivery.ID); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis ack failed: code=%d err=%v", code, err)
	}
}

func TestRedisBackendDeliversEmptyPayloadWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-empty-payload")

	if _, code, err := backend.Enqueue(ctx, "default", "", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue failed: code=%d err=%v", code, err)
	}
	delivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis reserve failed: %v", err)
	}
	if delivery.Payload != "" {
		t.Fatalf("expected empty payload, got %#v", delivery)
	}
	if code, err := backend.Ack(ctx, "default", delivery.ID); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis ack failed: code=%d err=%v", code, err)
	}
}

func TestRedisReserveWithZeroWaitPollsOnceWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-reserve-zero-wait")

	pollCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	delivery, err := backend.Reserve(pollCtx, []string{"default"}, "consumer", 0)
	if !errors.Is(err, errQueueEmpty) {
		t.Fatalf("expected queue empty, got delivery=%#v err=%v", delivery, err)
	}
	if err := pollCtx.Err(); err != nil {
		t.Fatalf("zero-wait reserve blocked until context expired: %v", err)
	}
}

func TestRedisStatsDoNotDoubleCountReservedMessagesWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-stats-reserved")

	if _, code, err := backend.Enqueue(ctx, "default", "payload", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue failed: code=%d err=%v", code, err)
	}
	delivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis reserve failed: %v", err)
	}

	stats, err := backend.Stats(ctx, "default")
	if err != nil {
		t.Fatalf("redis stats failed: %v", err)
	}
	if stats.Pending != 0 || stats.Reserved != 1 {
		t.Fatalf("expected pending=0 reserved=1, got pending=%d reserved=%d", stats.Pending, stats.Reserved)
	}

	if code, err := backend.Ack(ctx, "default", delivery.ID); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis ack failed: code=%d err=%v", code, err)
	}
}

func TestRedisReadPathsSkipUnknownQueuesWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-unknown-read")

	delivery, err := backend.Reserve(ctx, []string{"unknown"}, "consumer", 0)
	if !errors.Is(err, errQueueEmpty) {
		t.Fatalf("expected empty queue for unknown reserve, got delivery=%#v err=%v", delivery, err)
	}

	stats, err := backend.Stats(ctx, "unknown")
	if err != nil {
		t.Fatalf("redis stats for unknown queue failed: %v", err)
	}
	if stats.Queue != "unknown" || stats.Ready {
		t.Fatalf("expected unknown queue to be not ready, got %#v", stats)
	}
	if stats.MaxPayloadBytes != defaultMaxMessageBytes {
		t.Fatalf("expected max payload bytes to be reported, got %d", stats.MaxPayloadBytes)
	}
}

func TestRedisStatsReportsPendingErrorsWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-stats-pending-error")
	if err := backend.client.Del(ctx, backend.streamKey("default")).Err(); err != nil {
		t.Fatalf("redis stream cleanup failed: %v", err)
	}

	if _, err := backend.Stats(ctx, "default"); err == nil {
		t.Fatal("expected stats to report missing consumer group")
	}
	if got := backend.stats.backendErrors.Load(); got != 1 {
		t.Fatalf("expected one backend error, got %d", got)
	}
}
