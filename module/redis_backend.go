package queue

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisBackend struct {
	client            *redis.Client
	prefix            string
	group             string
	consumer          string
	queues            []string
	maxPayloadBytes   int
	visibilityTimeout time.Duration
	maxAttempts       int
	logger            *slog.Logger
	stats             backendCounters
	delayedID         atomic.Uint64
}

type delayedPayload struct {
	ID       string `json:"id"`
	Queue    string `json:"queue"`
	Payload  string `json:"payload"`
	Attempts int    `json:"attempts"`
}

var promoteDelayedScript = redis.NewScript(`
local items = redis.call("ZRANGEBYSCORE", KEYS[1], "-inf", ARGV[1], "LIMIT", 0, ARGV[2])
local promoted = 0

for _, item in ipairs(items) do
	local ok, delayed = pcall(cjson.decode, item)
	if ok and type(delayed) == "table" and type(delayed["payload"]) == "string" then
		local attempts = tonumber(delayed["attempts"]) or 1
		if attempts < 1 then
			attempts = 1
		end

		redis.call("XADD", KEYS[2], "*", "payload", delayed["payload"], "attempts", tostring(math.floor(attempts)))
		promoted = promoted + 1
	end

	redis.call("ZREM", KEYS[1], item)
end

return promoted
`)

func newRedisBackend(cfg backendConfig, queues []string, maxPayloadBytes int, visibilityTimeout time.Duration, maxAttempts int, logger *slog.Logger) (*redisBackend, error) {
	options, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}
	if cfg.TLS && options.TLSConfig == nil {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	options.MaxRetries = 3
	options.DialTimeout = 5 * time.Second
	options.ReadTimeout = 5 * time.Second
	options.WriteTimeout = 5 * time.Second

	return &redisBackend{
		client:            redis.NewClient(options),
		prefix:            cfg.KeyPrefix,
		group:             cfg.Group,
		consumer:          cfg.Consumer,
		queues:            queues,
		maxPayloadBytes:   maxPayloadBytes,
		visibilityTimeout: visibilityTimeout,
		maxAttempts:       maxAttempts,
		logger:            logger,
	}, nil
}

func (b *redisBackend) Start(ctx context.Context) error {
	if err := b.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	for _, queue := range b.queues {
		if err := b.ensureGroup(ctx, queue); err != nil {
			return err
		}
	}

	return nil
}

func (b *redisBackend) Enqueue(ctx context.Context, queue, payload string, delay time.Duration) (string, int, error) {
	if b.maxPayloadBytes > 0 && len(payload) > b.maxPayloadBytes {
		b.stats.payloadTooLarge.Add(1)
		return "", dispatchResultPayloadTooLarge, fmt.Errorf("payload exceeds %d bytes", b.maxPayloadBytes)
	}
	if !b.isConfiguredQueue(queue) {
		return "", dispatchResultQueueUnknown, fmt.Errorf("queue %q is not configured", queue)
	}
	if delay > 0 {
		id := fmt.Sprintf("delayed-%d-%d", time.Now().UnixNano(), b.delayedID.Add(1))
		body, err := json.Marshal(delayedPayload{ID: id, Queue: queue, Payload: payload, Attempts: 1})
		if err != nil {
			b.stats.backendErrors.Add(1)
			return "", dispatchResultBackendFailure, err
		}
		if err := b.client.ZAdd(ctx, b.delayedKey(queue), redis.Z{
			Score:  float64(time.Now().Add(delay).UnixMilli()),
			Member: string(body),
		}).Err(); err != nil {
			b.stats.backendErrors.Add(1)
			return "", dispatchResultBackendFailure, err
		}
		b.stats.enqueued.Add(1)
		return id, dispatchResultAccepted, nil
	}

	id, err := b.xadd(ctx, queue, payload, 1)
	if err != nil {
		b.stats.backendErrors.Add(1)
		return "", dispatchResultBackendFailure, err
	}
	b.stats.enqueued.Add(1)
	return id, dispatchResultAccepted, nil
}

func (b *redisBackend) Reserve(ctx context.Context, queues []string, consumer string, wait time.Duration) (*delivery, error) {
	if consumer == "" {
		consumer = b.consumer
	}

	for _, queue := range queues {
		if err := b.promoteDelayed(ctx, queue, 100); err != nil {
			b.stats.backendErrors.Add(1)
			return nil, err
		}
		if msg, ok, err := b.claimStale(ctx, queue, consumer); err != nil {
			b.stats.backendErrors.Add(1)
			return nil, err
		} else if ok {
			b.stats.reserved.Add(1)
			return b.deliveryFromMessage(queue, msg)
		}
	}

	streams := make([]string, 0, len(queues)*2)
	for _, queue := range queues {
		if !b.isConfiguredQueue(queue) {
			continue
		}
		streams = append(streams, b.streamKey(queue))
	}
	if len(streams) == 0 {
		return nil, errQueueEmpty
	}
	for range streams {
		streams = append(streams, ">")
	}

	result, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    b.group,
		Consumer: consumer,
		Streams:  streams,
		Count:    1,
		Block:    wait,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errQueueEmpty
		}
		b.stats.backendErrors.Add(1)
		return nil, err
	}

	for _, stream := range result {
		queue := b.queueFromStreamKey(stream.Stream)
		for _, msg := range stream.Messages {
			b.stats.reserved.Add(1)
			return b.deliveryFromMessage(queue, msg)
		}
	}

	return nil, errQueueEmpty
}

func (b *redisBackend) Ack(ctx context.Context, queue, id string) (int, error) {
	if !b.isConfiguredQueue(queue) {
		return dispatchResultQueueUnknown, fmt.Errorf("queue %q is not configured", queue)
	}
	stream := b.streamKey(queue)
	if err := b.client.XAck(ctx, stream, b.group, id).Err(); err != nil {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, err
	}
	_ = b.client.XDel(ctx, stream, id).Err()
	b.stats.acked.Add(1)
	return dispatchResultAccepted, nil
}

func (b *redisBackend) Release(ctx context.Context, queue, id string, delay time.Duration) (int, error) {
	msg, err := b.getMessage(ctx, queue, id)
	if err != nil {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, err
	}

	payload, attempts := payloadAndAttempts(msg)
	attempts++

	if attempts > b.maxAttempts {
		if err := b.addFailed(ctx, queue, id, payload, "max attempts exceeded"); err != nil {
			b.stats.backendErrors.Add(1)
			return dispatchResultBackendFailure, err
		}
		if err := b.ackAndDelete(ctx, queue, id); err != nil {
			b.stats.backendErrors.Add(1)
			return dispatchResultBackendFailure, err
		}
		b.stats.failed.Add(1)
		return dispatchResultAccepted, nil
	}

	if delay > 0 {
		body, err := json.Marshal(delayedPayload{ID: id, Queue: queue, Payload: payload, Attempts: attempts})
		if err != nil {
			b.stats.backendErrors.Add(1)
			return dispatchResultBackendFailure, err
		}
		if err := b.client.ZAdd(ctx, b.delayedKey(queue), redis.Z{
			Score:  float64(time.Now().Add(delay).UnixMilli()),
			Member: string(body),
		}).Err(); err != nil {
			b.stats.backendErrors.Add(1)
			return dispatchResultBackendFailure, err
		}
	} else if _, err := b.xadd(ctx, queue, payload, attempts); err != nil {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, err
	}

	if err := b.ackAndDelete(ctx, queue, id); err != nil {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, err
	}

	b.stats.released.Add(1)
	return dispatchResultAccepted, nil
}

func (b *redisBackend) Fail(ctx context.Context, queue, id, reason string) (int, error) {
	msg, err := b.getMessage(ctx, queue, id)
	if err != nil {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, err
	}
	payload, _ := payloadAndAttempts(msg)
	if err := b.addFailed(ctx, queue, id, payload, reason); err != nil {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, err
	}
	if err := b.ackAndDelete(ctx, queue, id); err != nil {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, err
	}
	b.stats.failed.Add(1)
	return dispatchResultAccepted, nil
}

func (b *redisBackend) Stats(ctx context.Context, queue string) (queueStats, error) {
	stream := b.streamKey(queue)
	pending, err := b.client.XLen(ctx, stream).Result()
	if err != nil {
		return queueStats{}, err
	}
	delayed, err := b.client.ZCard(ctx, b.delayedKey(queue)).Result()
	if err != nil {
		return queueStats{}, err
	}
	failed, err := b.client.XLen(ctx, b.failedKey(queue)).Result()
	if err != nil {
		return queueStats{}, err
	}
	reserved := int64(0)
	if p, err := b.client.XPending(ctx, stream, b.group).Result(); err == nil && p != nil {
		reserved = p.Count
	}

	return queueStats{
		Queue:           queue,
		Ready:           true,
		Pending:         pending,
		Delayed:         delayed,
		Reserved:        reserved,
		Failed:          failed,
		Enqueued:        b.stats.enqueued.Load(),
		ReservedTotal:   b.stats.reserved.Load(),
		Acked:           b.stats.acked.Load(),
		Released:        b.stats.released.Load(),
		FailedTotal:     b.stats.failed.Load(),
		DroppedFull:     b.stats.droppedFull.Load(),
		DroppedPayload:  b.stats.payloadTooLarge.Load(),
		BackendErrors:   b.stats.backendErrors.Load(),
		MaxPayloadBytes: b.maxPayloadBytes,
	}, nil
}

func (b *redisBackend) Close() error {
	return b.client.Close()
}

func (b *redisBackend) ensureGroup(ctx context.Context, queue string) error {
	err := b.client.XGroupCreateMkStream(ctx, b.streamKey(queue), b.group, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return fmt.Errorf("failed to create redis stream group for queue %q: %w", queue, err)
}

func (b *redisBackend) xadd(ctx context.Context, queue, payload string, attempts int) (string, error) {
	if err := b.ensureGroup(ctx, queue); err != nil {
		return "", err
	}
	return b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: b.streamKey(queue),
		Values: map[string]any{
			"payload":  payload,
			"attempts": attempts,
		},
	}).Result()
}

func (b *redisBackend) promoteDelayed(ctx context.Context, queue string, limit int64) error {
	if limit <= 0 {
		return nil
	}

	_, err := promoteDelayedScript.Run(
		ctx,
		b.client,
		[]string{b.delayedKey(queue), b.streamKey(queue)},
		strconv.FormatInt(time.Now().UnixMilli(), 10),
		strconv.FormatInt(limit, 10),
	).Result()
	return err
}

func (b *redisBackend) claimStale(ctx context.Context, queue, consumer string) (redis.XMessage, bool, error) {
	if b.visibilityTimeout <= 0 {
		return redis.XMessage{}, false, nil
	}
	messages, _, err := b.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   b.streamKey(queue),
		Group:    b.group,
		Consumer: consumer,
		MinIdle:  b.visibilityTimeout,
		Start:    "0-0",
		Count:    1,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return redis.XMessage{}, false, nil
		}
		return redis.XMessage{}, false, err
	}
	if len(messages) == 0 {
		return redis.XMessage{}, false, nil
	}
	return messages[0], true, nil
}

func (b *redisBackend) getMessage(ctx context.Context, queue, id string) (redis.XMessage, error) {
	if !b.isConfiguredQueue(queue) {
		return redis.XMessage{}, fmt.Errorf("queue %q is not configured", queue)
	}
	messages, err := b.client.XRange(ctx, b.streamKey(queue), id, id).Result()
	if err != nil {
		return redis.XMessage{}, err
	}
	if len(messages) == 0 {
		return redis.XMessage{}, fmt.Errorf("delivery %q was not found", id)
	}
	return messages[0], nil
}

func (b *redisBackend) addFailed(ctx context.Context, queue, originalID, payload, reason string) error {
	_, err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: b.failedKey(queue),
		Values: map[string]any{
			"original_id": originalID,
			"payload":     payload,
			"reason":      reason,
			"failed_at":   time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Result()
	return err
}

func (b *redisBackend) ackAndDelete(ctx context.Context, queue, id string) error {
	stream := b.streamKey(queue)
	if err := b.client.XAck(ctx, stream, b.group, id).Err(); err != nil {
		return err
	}
	_ = b.client.XDel(ctx, stream, id).Err()
	return nil
}

func (b *redisBackend) deliveryFromMessage(queue string, msg redis.XMessage) (*delivery, error) {
	payload, attempts := payloadAndAttempts(msg)
	if payload == "" {
		return nil, fmt.Errorf("redis stream message %q has no payload", msg.ID)
	}
	return &delivery{
		ID:       msg.ID,
		Queue:    queue,
		Payload:  payload,
		Attempts: attempts,
	}, nil
}

func payloadAndAttempts(msg redis.XMessage) (string, int) {
	payload, _ := msg.Values["payload"].(string)
	attempts := 1
	switch value := msg.Values["attempts"].(type) {
	case string:
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			attempts = parsed
		}
	case int:
		if value > 0 {
			attempts = value
		}
	case int64:
		if value > 0 {
			attempts = int(value)
		}
	}
	return payload, attempts
}

func (b *redisBackend) streamKey(queue string) string {
	return b.prefix + ":" + queue + ":stream"
}

func (b *redisBackend) delayedKey(queue string) string {
	return b.prefix + ":" + queue + ":delayed"
}

func (b *redisBackend) failedKey(queue string) string {
	return b.prefix + ":" + queue + ":failed"
}

func (b *redisBackend) queueFromStreamKey(key string) string {
	prefix := b.prefix + ":"
	suffix := ":stream"
	return strings.TrimSuffix(strings.TrimPrefix(key, prefix), suffix)
}

func (b *redisBackend) isConfiguredQueue(queue string) bool {
	for _, configured := range b.queues {
		if configured == queue {
			return true
		}
	}
	return false
}
