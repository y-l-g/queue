package queue

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisPromoteDelayedIsAtomicWhenConfigured(t *testing.T) {
	ctx, backendA := newTestRedisBackendWithConfig(t, "pogo-test-promote-atomic", "test-consumer-a", 100*time.Millisecond, 3)
	_, backendB := newTestRedisBackendWithConfig(t, "pogo-test-promote-atomic", "test-consumer-b", 100*time.Millisecond, 3)

	body, err := json.Marshal(delayedPayload{
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

func TestRedisDelayedDuplicatePayloads(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-delayed-duplicates")

	const payload = "payload"
	const delay = 25 * time.Millisecond
	for i := 0; i < 2; i++ {
		if _, result, err := backend.Enqueue(ctx, "default", payload, delay); err != nil {
			t.Fatalf("enqueue delayed payload failed: %v", err)
		} else if result != dispatchResultAccepted {
			t.Fatalf("expected accepted delayed enqueue, got result %d", result)
		}
	}

	delayedLen, err := backend.client.ZCard(ctx, backend.delayedKey("default")).Result()
	if err != nil {
		t.Fatalf("read delayed length failed: %v", err)
	}
	if delayedLen != 2 {
		t.Fatalf("expected two delayed jobs, got %d", delayedLen)
	}

	time.Sleep(delay + 25*time.Millisecond)
	if err := backend.promoteDelayed(ctx, "default", 100); err != nil {
		t.Fatalf("promote delayed failed: %v", err)
	}

	messages, err := backend.client.XRange(ctx, backend.streamKey("default"), "-", "+").Result()
	if err != nil {
		t.Fatalf("read promoted stream failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected two promoted stream messages, got %d", len(messages))
	}
	if messages[0].ID == messages[1].ID {
		t.Fatalf("expected distinct promoted stream ids, got %q", messages[0].ID)
	}
}

func TestRedisMalformedDelayedEntryIsReportedAndPreserved(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-malformed-delayed")

	const malformedMember = `{"id":"legacy","attempts":1}`
	if err := backend.client.ZAdd(ctx, backend.delayedKey("default"), redis.Z{
		Score:  float64(time.Now().Add(-time.Second).UnixMilli()),
		Member: malformedMember,
	}).Err(); err != nil {
		t.Fatalf("seed malformed delayed payload failed: %v", err)
	}

	_, err := backend.Reserve(ctx, []string{"default"}, "consumer", 0)
	if err == nil {
		t.Fatal("expected reserve to report malformed delayed entry")
	}
	if errors.Is(err, errQueueEmpty) {
		t.Fatalf("expected backend error, got queue empty: %v", err)
	}
	if got := backend.stats.backendErrors.Load(); got != 1 {
		t.Fatalf("expected one backend error, got %d", got)
	}

	if _, err := backend.client.ZScore(ctx, backend.delayedKey("default"), malformedMember).Result(); err != nil {
		t.Fatalf("expected malformed delayed entry to remain for inspection: %v", err)
	}
}

func TestRedisMalformedDelayedBatchDoesNotPartiallyPromote(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-malformed-delayed-batch")

	validMember, err := json.Marshal(delayedPayload{
		Payload:  "payload",
		Attempts: 1,
	})
	if err != nil {
		t.Fatalf("marshal delayed payload failed: %v", err)
	}
	const malformedMember = `{"id":"legacy","attempts":1}`
	now := time.Now()
	if err := backend.client.ZAdd(ctx, backend.delayedKey("default"),
		redis.Z{
			Score:  float64(now.Add(-2 * time.Second).UnixMilli()),
			Member: string(validMember),
		},
		redis.Z{
			Score:  float64(now.Add(-time.Second).UnixMilli()),
			Member: malformedMember,
		},
	).Err(); err != nil {
		t.Fatalf("seed delayed payloads failed: %v", err)
	}

	_, err = backend.Reserve(ctx, []string{"default"}, "consumer", 0)
	if err == nil {
		t.Fatal("expected reserve to report malformed delayed entry")
	}
	if errors.Is(err, errQueueEmpty) {
		t.Fatalf("expected backend error, got queue empty: %v", err)
	}

	streamLen, err := backend.client.XLen(ctx, backend.streamKey("default")).Result()
	if err != nil {
		t.Fatalf("read stream length failed: %v", err)
	}
	if streamLen != 0 {
		t.Fatalf("expected no partial promotion, got %d stream messages", streamLen)
	}

	delayedLen, err := backend.client.ZCard(ctx, backend.delayedKey("default")).Result()
	if err != nil {
		t.Fatalf("read delayed length failed: %v", err)
	}
	if delayedLen != 2 {
		t.Fatalf("expected both delayed entries to remain, got %d", delayedLen)
	}
}

func TestRedisReserveReportsPromoteDelayedErrorsWhenConfigured(t *testing.T) {
	ctx, backend := newTestRedisBackend(t, "pogo-test-promote-error")

	if err := backend.client.Set(ctx, backend.delayedKey("default"), "wrong-type", 0).Err(); err != nil {
		t.Fatalf("seed invalid delayed key failed: %v", err)
	}

	_, err := backend.Reserve(ctx, []string{"default"}, "consumer", 0)
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
