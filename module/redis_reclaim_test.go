package queue

import (
	"errors"
	"testing"
	"time"
)

func TestRedisStaleClaimIncrementsAttemptsWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackendWithConfig(t, "pogo-test-stale-attempts", "test-consumer", time.Millisecond, 5)

	if _, code, err := backend.Enqueue(ctx, "default", "payload", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue failed: code=%d err=%v", code, err)
	}
	first, err := backend.Reserve(ctx, []string{"default"}, "first-consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis first reserve failed: %v", err)
	}
	if first.Attempts != 1 {
		t.Fatalf("expected first attempt, got %d", first.Attempts)
	}

	time.Sleep(10 * time.Millisecond)

	reclaimed, err := backend.Reserve(ctx, []string{"default"}, "second-consumer", 5*time.Millisecond)
	if err != nil {
		t.Fatalf("redis stale reserve failed: %v", err)
	}
	if reclaimed.ID != first.ID {
		t.Fatalf("expected stale claim for %q, got %q", first.ID, reclaimed.ID)
	}
	if reclaimed.Attempts != 2 {
		t.Fatalf("expected reclaimed attempt 2, got %d", reclaimed.Attempts)
	}

	if code, err := backend.Release(ctx, "default", reclaimed.ID, 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis release after stale claim failed: code=%d err=%v", code, err)
	}

	next, err := backend.Reserve(ctx, []string{"default"}, "third-consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis reserve after stale release failed: %v", err)
	}
	if next.Attempts != 3 {
		t.Fatalf("expected released stale delivery to become attempt 3, got %d", next.Attempts)
	}
	if code, err := backend.Ack(ctx, "default", next.ID); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis ack failed: code=%d err=%v", code, err)
	}
}

func TestRedisStaleClaimFailsAfterMaxAttemptsWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackendWithConfig(t, "pogo-test-stale-max-attempts", "test-consumer", time.Millisecond, 1)

	if _, code, err := backend.Enqueue(ctx, "default", "payload", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue failed: code=%d err=%v", code, err)
	}
	first, err := backend.Reserve(ctx, []string{"default"}, "first-consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis first reserve failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	delivery, err := backend.Reserve(ctx, []string{"default"}, "second-consumer", 5*time.Millisecond)
	if !errors.Is(err, errQueueEmpty) {
		t.Fatalf("expected stale delivery to move to failed stream, got delivery=%#v err=%v", delivery, err)
	}

	streamLen, err := backend.client.XLen(ctx, backend.streamKey("default")).Result()
	if err != nil {
		t.Fatalf("read stream length failed: %v", err)
	}
	if streamLen != 0 {
		t.Fatalf("expected original stream to be empty, got %d messages", streamLen)
	}
	failed, err := backend.client.XRange(ctx, backend.failedKey("default"), "-", "+").Result()
	if err != nil {
		t.Fatalf("read failed stream failed: %v", err)
	}
	if len(failed) != 1 {
		t.Fatalf("expected one failed message, got %d", len(failed))
	}
	if failed[0].Values["original_id"] != first.ID || failed[0].Values["payload"] != "payload" || failed[0].Values["reason"] != "max attempts exceeded" {
		t.Fatalf("unexpected failed message: %#v", failed[0].Values)
	}
	if got := backend.stats.failed.Load(); got != 1 {
		t.Fatalf("expected one failed counter, got %d", got)
	}
}
