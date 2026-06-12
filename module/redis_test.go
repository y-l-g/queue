package queue

import (
	"context"
	"os"
	"testing"
	"time"
)

func newTestRedisBackend(t *testing.T, keyPrefix string) (context.Context, *redisBackend) {
	t.Helper()

	return newTestRedisBackendWithConfig(t, keyPrefix, "test-consumer", 100*time.Millisecond, 3)
}

func newTestRedisBackendWithConfig(t *testing.T, keyPrefix, consumer string, visibilityTimeout time.Duration, maxAttempts int) (context.Context, *redisBackend) {
	t.Helper()

	redisURL := os.Getenv("POGO_REDIS_URL")
	if redisURL == "" {
		t.Skip("POGO_REDIS_URL is not set")
	}

	ctx := context.Background()
	backend, err := newRedisBackend(
		backendConfig{
			RedisURL:  redisURL,
			KeyPrefix: keyPrefix,
			Group:     "test",
			Consumer:  consumer,
		},
		[]string{"default"},
		defaultMaxMessageBytes,
		visibilityTimeout,
		maxAttempts,
	)
	if err != nil {
		t.Fatalf("redis backend init failed: %v", err)
	}

	keys := []string{backend.streamKey("default"), backend.delayedKey("default"), backend.failedKey("default")}
	if err := backend.client.Del(ctx, keys...).Err(); err != nil {
		_ = backend.Close()
		t.Fatalf("redis cleanup failed: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.client.Del(ctx, keys...).Err()
		_ = backend.Close()
	})

	if err := backend.Start(ctx); err != nil {
		t.Fatalf("redis backend start failed: %v", err)
	}

	return ctx, backend
}
