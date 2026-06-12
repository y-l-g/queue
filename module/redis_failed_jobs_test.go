package queue

import (
	"context"
	"testing"
	"time"
)

func TestRedisFailedJobOperationsWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-failed-ops")

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

	failed, code, err := backend.ListFailed(ctx, "default", 10)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis list failed jobs failed: code=%d err=%v", code, err)
	}
	if len(failed) != 1 || failed[0].OriginalID != delivery.ID || failed[0].Payload != "payload" || failed[0].Reason != "boom" {
		t.Fatalf("unexpected redis failed jobs: %#v", failed)
	}

	retryID, code, err := backend.RetryFailed(ctx, "default", failed[0].ID)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis retry failed job failed: code=%d err=%v", code, err)
	}
	if retryID == "" || retryID == delivery.ID {
		t.Fatalf("expected redis retry to create a fresh delivery id, got %q", retryID)
	}
	retried, err := backend.Reserve(ctx, []string{"default"}, "consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis reserve retried job failed: %v", err)
	}
	if retried.ID != retryID || retried.Payload != "payload" || retried.Attempts != 1 {
		t.Fatalf("unexpected redis retried delivery: %#v", retried)
	}
	if code, err := backend.Ack(ctx, "default", retried.ID); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis ack retried job failed: code=%d err=%v", code, err)
	}

	first := failRedisJob(t, ctx, backend, "forget")
	second := failRedisJob(t, ctx, backend, "purge")
	if code, err := backend.ForgetFailed(ctx, "default", first); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis forget failed job failed: code=%d err=%v", code, err)
	}
	failed, code, err = backend.ListFailed(ctx, "default", 10)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis list after forget failed: code=%d err=%v", code, err)
	}
	if len(failed) != 1 || failed[0].ID != second {
		t.Fatalf("expected one remaining redis failed job %q, got %#v", second, failed)
	}

	purged, code, err := backend.PurgeFailed(ctx, "default")
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis purge failed jobs failed: count=%d code=%d err=%v", purged, code, err)
	}
	if purged != 1 {
		t.Fatalf("expected one purged redis failed job, got %d", purged)
	}
	failed, code, err = backend.ListFailed(ctx, "default", 10)
	if err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis list after purge failed: code=%d err=%v", code, err)
	}
	if len(failed) != 0 {
		t.Fatalf("expected redis failed jobs to be empty, got %#v", failed)
	}
}

func failRedisJob(t *testing.T, ctx context.Context, backend *redisBackend, payload string) string {
	t.Helper()

	if _, code, err := backend.Enqueue(ctx, "default", payload, 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis enqueue %q failed: code=%d err=%v", payload, code, err)
	}
	delivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("redis reserve %q failed: %v", payload, err)
	}
	if code, err := backend.Fail(ctx, "default", delivery.ID, payload); err != nil || code != dispatchResultAccepted {
		t.Fatalf("redis fail %q failed: code=%d err=%v", payload, code, err)
	}

	failed, code, err := backend.ListFailed(ctx, "default", 1)
	if err != nil || code != dispatchResultAccepted || len(failed) == 0 {
		t.Fatalf("redis list %q failed: code=%d err=%v failed=%#v", payload, code, err, failed)
	}
	return failed[0].ID
}
