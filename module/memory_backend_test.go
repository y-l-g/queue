package queue

import (
	"context"
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
	)

	_, code, err := backend.Enqueue(context.Background(), "default", "payload", 0)
	if err == nil {
		t.Fatal("expected oversized payload error")
	}
	if code != dispatchResultPayloadTooLarge {
		t.Fatalf("expected payload-too-large status, got %d", code)
	}
}

func TestMemoryBackendCountsInvalidLifecycleErrors(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend(
		backendConfig{MaxMessages: 10, MaxTotalBytes: 1 << 20},
		[]string{"default"},
		defaultMaxMessageBytes,
		time.Minute,
		3,
	)

	if code, err := backend.Ack(ctx, "default", "missing"); err == nil || code != dispatchResultBackendFailure {
		t.Fatalf("expected ack failure, got code=%d err=%v", code, err)
	}
	if code, err := backend.Release(ctx, "default", "missing", 0); err == nil || code != dispatchResultBackendFailure {
		t.Fatalf("expected release failure, got code=%d err=%v", code, err)
	}
	if code, err := backend.Fail(ctx, "default", "missing", "boom"); err == nil || code != dispatchResultBackendFailure {
		t.Fatalf("expected fail failure, got code=%d err=%v", code, err)
	}
	if got := backend.stats.backendErrors.Load(); got != 3 {
		t.Fatalf("expected three backend errors, got %d", got)
	}
}

func TestMemoryBackendFailedJobOperations(t *testing.T) {
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
	delivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if code, err := backend.Fail(ctx, "default", delivery.ID, "boom"); err != nil || code != dispatchResultAccepted {
		t.Fatalf("fail failed: code=%d err=%v", code, err)
	}

	failed, code, err := backend.ListFailed(ctx, "default", 10)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("list failed jobs failed: code=%d err=%v", code, err)
	}
	if len(failed) != 1 || failed[0].ID != delivery.ID || failed[0].Payload != "payload" || failed[0].Reason != "boom" {
		t.Fatalf("unexpected failed jobs: %#v", failed)
	}

	retryID, code, err := backend.RetryFailed(ctx, "default", failed[0].ID)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("retry failed job failed: code=%d err=%v", code, err)
	}
	if retryID == "" || retryID == delivery.ID {
		t.Fatalf("expected retry to create a fresh delivery id, got %q", retryID)
	}
	retried, err := backend.Reserve(ctx, []string{"default"}, "consumer", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("reserve retried job failed: %v", err)
	}
	if retried.ID != retryID || retried.Payload != "payload" || retried.Attempts != 1 {
		t.Fatalf("unexpected retried delivery: %#v", retried)
	}
	if code, err := backend.Ack(ctx, "default", retried.ID); err != nil || code != dispatchResultAccepted {
		t.Fatalf("ack retried job failed: code=%d err=%v", code, err)
	}

	first := failMemoryJob(t, ctx, backend, "forget")
	second := failMemoryJob(t, ctx, backend, "purge")
	if code, err := backend.ForgetFailed(ctx, "default", first); err != nil || code != dispatchResultAccepted {
		t.Fatalf("forget failed job failed: code=%d err=%v", code, err)
	}
	failed, code, err = backend.ListFailed(ctx, "default", 10)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("list after forget failed: code=%d err=%v", code, err)
	}
	if len(failed) != 1 || failed[0].ID != second {
		t.Fatalf("expected one remaining failed job %q, got %#v", second, failed)
	}

	purged, code, err := backend.PurgeFailed(ctx, "default")
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("purge failed jobs failed: count=%d code=%d err=%v", purged, code, err)
	}
	if purged != 1 {
		t.Fatalf("expected one purged failed job, got %d", purged)
	}
	failed, code, err = backend.ListFailed(ctx, "default", 10)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("list after purge failed: code=%d err=%v", code, err)
	}
	if len(failed) != 0 {
		t.Fatalf("expected failed jobs to be empty, got %#v", failed)
	}
}

func failMemoryJob(t *testing.T, ctx context.Context, backend *memoryBackend, payload string) string {
	t.Helper()

	if _, code, err := backend.Enqueue(ctx, "default", payload, 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("enqueue %q failed: code=%d err=%v", payload, code, err)
	}
	delivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("reserve %q failed: %v", payload, err)
	}
	if code, err := backend.Fail(ctx, "default", delivery.ID, payload); err != nil || code != dispatchResultAccepted {
		t.Fatalf("fail %q failed: code=%d err=%v", payload, code, err)
	}

	return delivery.ID
}
