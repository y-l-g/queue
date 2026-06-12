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
