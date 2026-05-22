package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type memoryMessage struct {
	delivery
	AvailableAt time.Time
	ReservedAt  time.Time
}

type memoryQueue struct {
	ready   []*memoryMessage
	pending map[string]*memoryMessage
	failed  map[string]*memoryMessage
	delayed map[string]*memoryMessage
}

type memoryBackend struct {
	mu                sync.Mutex
	queues            map[string]*memoryQueue
	maxPayloadBytes   int
	maxMessages       int
	maxTotalBytes     int
	visibilityTimeout time.Duration
	maxAttempts       int
	nextID            atomic.Uint64
	totalBytes        int
	stats             backendCounters
}

type backendCounters struct {
	enqueued        atomic.Uint64
	reserved        atomic.Uint64
	acked           atomic.Uint64
	released        atomic.Uint64
	failed          atomic.Uint64
	droppedFull     atomic.Uint64
	payloadTooLarge atomic.Uint64
	backendErrors   atomic.Uint64
}

func newMemoryBackend(cfg backendConfig, queues []string, maxPayloadBytes int, visibilityTimeout time.Duration, maxAttempts int) *memoryBackend {
	b := &memoryBackend{
		queues:            make(map[string]*memoryQueue, len(queues)),
		maxPayloadBytes:   maxPayloadBytes,
		maxMessages:       cfg.MaxMessages,
		maxTotalBytes:     cfg.MaxTotalBytes,
		visibilityTimeout: visibilityTimeout,
		maxAttempts:       maxAttempts,
	}
	for _, queue := range queues {
		b.queues[queue] = newMemoryQueue()
	}
	return b
}

func newMemoryQueue() *memoryQueue {
	return &memoryQueue{
		pending: make(map[string]*memoryMessage),
		failed:  make(map[string]*memoryMessage),
		delayed: make(map[string]*memoryMessage),
	}
}

func (b *memoryBackend) Start(context.Context) error {
	return nil
}

func (b *memoryBackend) Enqueue(_ context.Context, queue, payload string, delay time.Duration) (string, int, error) {
	if b.maxPayloadBytes > 0 && len(payload) > b.maxPayloadBytes {
		b.stats.payloadTooLarge.Add(1)
		return "", dispatchResultPayloadTooLarge, fmt.Errorf("payload exceeds %d bytes", b.maxPayloadBytes)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	q, ok := b.queues[queue]
	if !ok {
		return "", dispatchResultQueueUnknown, fmt.Errorf("queue %q is not configured", queue)
	}

	if b.messageCountLocked() >= b.maxMessages || b.totalBytes+len(payload) > b.maxTotalBytes {
		b.stats.droppedFull.Add(1)
		return "", dispatchResultFull, fmt.Errorf("memory backend capacity exceeded")
	}

	id := fmt.Sprintf("mem-%d", b.nextID.Add(1))
	msg := &memoryMessage{
		delivery: delivery{
			ID:       id,
			Queue:    queue,
			Payload:  payload,
			Attempts: 1,
		},
		AvailableAt: time.Now().Add(delay),
	}

	b.totalBytes += len(payload)
	if delay > 0 {
		q.delayed[id] = msg
	} else {
		q.ready = append(q.ready, msg)
	}
	b.stats.enqueued.Add(1)

	return id, dispatchResultAccepted, nil
}

func (b *memoryBackend) Reserve(ctx context.Context, queues []string, _ string, wait time.Duration) (*delivery, error) {
	deadline := time.Now().Add(wait)

	for {
		b.mu.Lock()
		now := time.Now()
		for _, name := range queues {
			q := b.queues[name]
			if q == nil {
				continue
			}
			b.reclaimLocked(q, now)
			b.promoteDelayedLocked(q, now)
			if len(q.ready) == 0 {
				continue
			}

			msg := q.ready[0]
			q.ready = q.ready[1:]
			msg.ReservedAt = now
			q.pending[msg.ID] = msg
			b.stats.reserved.Add(1)
			copy := msg.delivery
			b.mu.Unlock()
			return &copy, nil
		}

		if wait <= 0 || time.Now().After(deadline) {
			b.mu.Unlock()
			return nil, errQueueEmpty
		}
		b.mu.Unlock()

		sleep := 25 * time.Millisecond
		if remaining := time.Until(deadline); remaining < sleep {
			sleep = remaining
		}
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
}

func (b *memoryBackend) Ack(_ context.Context, queue, id string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.queues[queue]
	if q == nil {
		return dispatchResultQueueUnknown, fmt.Errorf("queue %q is not configured", queue)
	}
	msg, ok := q.pending[id]
	if !ok {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, fmt.Errorf("delivery %q is not pending", id)
	}
	delete(q.pending, id)
	b.totalBytes -= len(msg.Payload)
	b.stats.acked.Add(1)
	return dispatchResultAccepted, nil
}

func (b *memoryBackend) Release(_ context.Context, queue, id string, delay time.Duration) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.queues[queue]
	if q == nil {
		return dispatchResultQueueUnknown, fmt.Errorf("queue %q is not configured", queue)
	}
	msg, ok := q.pending[id]
	if !ok {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, fmt.Errorf("delivery %q is not pending", id)
	}
	delete(q.pending, id)
	msg.Attempts++
	msg.ReservedAt = time.Time{}
	msg.AvailableAt = time.Now().Add(delay)
	if msg.Attempts > b.maxAttempts {
		q.failed[id] = msg
		b.stats.failed.Add(1)
		return dispatchResultAccepted, nil
	}
	if delay > 0 {
		q.delayed[id] = msg
	} else {
		q.ready = append(q.ready, msg)
	}
	b.stats.released.Add(1)
	return dispatchResultAccepted, nil
}

func (b *memoryBackend) Fail(_ context.Context, queue, id, _ string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.queues[queue]
	if q == nil {
		return dispatchResultQueueUnknown, fmt.Errorf("queue %q is not configured", queue)
	}
	msg, ok := q.pending[id]
	if !ok {
		b.stats.backendErrors.Add(1)
		return dispatchResultBackendFailure, fmt.Errorf("delivery %q is not pending", id)
	}
	delete(q.pending, id)
	q.failed[id] = msg
	b.stats.failed.Add(1)
	return dispatchResultAccepted, nil
}

func (b *memoryBackend) Stats(_ context.Context, queue string) (queueStats, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.queues[queue]
	if q == nil {
		return queueStats{Queue: queue, Ready: false, MaxPayloadBytes: b.maxPayloadBytes}, nil
	}
	return queueStats{
		Queue:           queue,
		Ready:           true,
		Pending:         int64(len(q.ready)),
		Delayed:         int64(len(q.delayed)),
		Reserved:        int64(len(q.pending)),
		Failed:          int64(len(q.failed)),
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

func (b *memoryBackend) Close() error {
	return nil
}

func (b *memoryBackend) reclaimLocked(q *memoryQueue, now time.Time) {
	if b.visibilityTimeout <= 0 {
		return
	}
	for id, msg := range q.pending {
		if now.Sub(msg.ReservedAt) < b.visibilityTimeout {
			continue
		}
		delete(q.pending, id)
		msg.Attempts++
		msg.ReservedAt = time.Time{}
		if msg.Attempts > b.maxAttempts {
			q.failed[id] = msg
			b.stats.failed.Add(1)
		} else {
			q.ready = append(q.ready, msg)
			b.stats.released.Add(1)
		}
	}
}

func (b *memoryBackend) promoteDelayedLocked(q *memoryQueue, now time.Time) {
	for id, msg := range q.delayed {
		if msg.AvailableAt.After(now) {
			continue
		}
		delete(q.delayed, id)
		q.ready = append(q.ready, msg)
	}
}

func (b *memoryBackend) messageCountLocked() int {
	total := 0
	for _, q := range b.queues {
		total += len(q.ready) + len(q.pending) + len(q.failed) + len(q.delayed)
	}
	return total
}
