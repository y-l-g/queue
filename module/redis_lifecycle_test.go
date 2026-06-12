package queue

import (
	"testing"
	"time"
)

func TestRedisAckRejectsUnknownDeliveryWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-ack-unknown")

	code, err := backend.Ack(ctx, "default", "0-0")
	if err == nil {
		t.Fatal("expected unknown delivery ack to fail")
	}
	if code != dispatchResultBackendFailure {
		t.Fatalf("expected backend failure status, got %d", code)
	}
	if got := backend.stats.backendErrors.Load(); got != 1 {
		t.Fatalf("expected one backend error, got %d", got)
	}
}

func TestRedisReleaseRejectsUnreservedDeliveryWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-release-unreserved")

	id, code, err := backend.Enqueue(ctx, "default", "payload", 0)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue failed: code=%d err=%v", code, err)
	}

	code, err = backend.Release(ctx, "default", id, 0)
	if err == nil {
		t.Fatal("expected unreserved delivery release to fail")
	}
	if code != dispatchResultBackendFailure {
		t.Fatalf("expected backend failure status, got %d", code)
	}

	streamLen, err := backend.client.XLen(ctx, backend.streamKey("default")).Result()
	if err != nil {
		t.Fatalf("read stream length failed: %v", err)
	}
	if streamLen != 1 {
		t.Fatalf("expected original stream message to remain, got %d messages", streamLen)
	}
	if got := backend.stats.backendErrors.Load(); got != 1 {
		t.Fatalf("expected one backend error, got %d", got)
	}
}

func TestRedisFailMovesReservedDeliveryWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-fail-reserved")

	if _, code, err := backend.Enqueue(ctx, "default", "payload", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue failed: code=%d err=%v", code, err)
	}
	delivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis reserve failed: %v", err)
	}

	if code, err := backend.Fail(ctx, "default", delivery.ID, "boom"); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis fail failed: code=%d err=%v", code, err)
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
	if failed[0].Values["original_id"] != delivery.ID || failed[0].Values["payload"] != "payload" || failed[0].Values["reason"] != "boom" {
		t.Fatalf("unexpected failed message: %#v", failed[0].Values)
	}
	if got := backend.stats.failed.Load(); got != 1 {
		t.Fatalf("expected one failed counter, got %d", got)
	}
}

func TestRedisFailRejectsUnreservedDeliveryWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-fail-unreserved")

	id, code, err := backend.Enqueue(ctx, "default", "payload", 0)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue failed: code=%d err=%v", code, err)
	}

	code, err = backend.Fail(ctx, "default", id, "boom")
	if err == nil {
		t.Fatal("expected unreserved delivery fail to fail")
	}
	if code != dispatchResultBackendFailure {
		t.Fatalf("expected backend failure status, got %d", code)
	}

	streamLen, err := backend.client.XLen(ctx, backend.streamKey("default")).Result()
	if err != nil {
		t.Fatalf("read stream length failed: %v", err)
	}
	if streamLen != 1 {
		t.Fatalf("expected original stream message to remain, got %d messages", streamLen)
	}
	failedLen, err := backend.client.XLen(ctx, backend.failedKey("default")).Result()
	if err != nil {
		t.Fatalf("read failed stream length failed: %v", err)
	}
	if failedLen != 0 {
		t.Fatalf("expected failed stream to remain empty, got %d messages", failedLen)
	}
	if got := backend.stats.backendErrors.Load(); got != 1 {
		t.Fatalf("expected one backend error, got %d", got)
	}
}
