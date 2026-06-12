package queue

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestQueueMetricsCollectorEmitsStatus(t *testing.T) {
	ctx := context.Background()
	backend := newMemoryBackend(
		backendConfig{MaxMessages: 10, MaxTotalBytes: 1 << 20},
		[]string{"default"},
		defaultMaxMessageBytes,
		time.Minute,
		3,
	)

	if _, code, err := backend.Enqueue(ctx, "default", "reserved", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("enqueue reserved failed: code=%d err=%v", code, err)
	}
	if _, err := backend.Reserve(ctx, []string{"default"}, "consumer", 10*time.Millisecond); err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if _, code, err := backend.Enqueue(ctx, "default", "failed", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("enqueue failed message failed: code=%d err=%v", code, err)
	}
	failedDelivery, err := backend.Reserve(ctx, []string{"default"}, "consumer", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("reserve failed message failed: %v", err)
	}
	if code, err := backend.Fail(ctx, "default", failedDelivery.ID, "boom"); err != nil || code != dispatchResultAccepted {
		t.Fatalf("fail delivery failed: code=%d err=%v", code, err)
	}
	if _, code, err := backend.Enqueue(ctx, "default", "pending", 0); err != nil || code != dispatchResultAccepted {
		t.Fatalf("enqueue pending failed: code=%d err=%v", code, err)
	}
	if _, code, err := backend.Enqueue(ctx, "default", "delayed", time.Hour); err != nil || code != dispatchResultAccepted {
		t.Fatalf("enqueue delayed failed: code=%d err=%v", code, err)
	}

	manager := &manager{
		backend:         backend,
		queues:          []string{"default"},
		maxPayloadBytes: defaultMaxMessageBytes,
	}
	manager.started.Store(true)
	setGlobalManagerForTest(t, manager)

	registry := prometheus.NewRegistry()
	if err := registry.Register(newQueueMetricsCollector()); err != nil {
		t.Fatalf("register metrics failed: %v", err)
	}

	expected := `
# HELP caddy_pogo_queue_events_total Queue lifecycle and rejection events observed by the module.
# TYPE caddy_pogo_queue_events_total counter
caddy_pogo_queue_events_total{event="acked"} 0
caddy_pogo_queue_events_total{event="backend_errors"} 0
caddy_pogo_queue_events_total{event="dropped_full"} 0
caddy_pogo_queue_events_total{event="dropped_payload_too_large"} 0
caddy_pogo_queue_events_total{event="dropped_shutdown"} 0
caddy_pogo_queue_events_total{event="enqueued"} 4
caddy_pogo_queue_events_total{event="failed"} 1
caddy_pogo_queue_events_total{event="released"} 0
caddy_pogo_queue_events_total{event="reserved"} 2
# HELP caddy_pogo_queue_messages Number of queue messages by state.
# TYPE caddy_pogo_queue_messages gauge
caddy_pogo_queue_messages{queue="default",state="delayed"} 1
caddy_pogo_queue_messages{queue="default",state="failed"} 1
caddy_pogo_queue_messages{queue="default",state="pending"} 1
caddy_pogo_queue_messages{queue="default",state="reserved"} 1
# HELP caddy_pogo_queue_queue_ready Whether the configured queue backend can report status for this queue.
# TYPE caddy_pogo_queue_queue_ready gauge
caddy_pogo_queue_queue_ready{queue="default"} 1
# HELP caddy_pogo_queue_worker_ready Whether the queue worker pool has started and is not shutting down.
# TYPE caddy_pogo_queue_worker_ready gauge
caddy_pogo_queue_worker_ready 1
`
	if err := testutil.GatherAndCompare(
		registry,
		strings.NewReader(expected),
		"caddy_pogo_queue_events_total",
		"caddy_pogo_queue_messages",
		"caddy_pogo_queue_queue_ready",
		"caddy_pogo_queue_worker_ready",
	); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterQueueMetricsIsIdempotent(t *testing.T) {
	registry := prometheus.NewRegistry()

	if err := registerQueueMetrics(registry); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := registerQueueMetrics(registry); err != nil {
		t.Fatalf("second register failed: %v", err)
	}
}

func setGlobalManagerForTest(t *testing.T, manager *manager) {
	t.Helper()

	globalManagerMu.Lock()
	previous := globalManager
	globalManager = manager
	globalManagerMu.Unlock()

	t.Cleanup(func() {
		globalManagerMu.Lock()
		globalManager = previous
		globalManagerMu.Unlock()
	})
}
