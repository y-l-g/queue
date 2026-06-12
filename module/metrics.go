package queue

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const queueMetricsScrapeTimeout = 2 * time.Second

var queueMetrics = struct {
	sync.Mutex
	collector  *queueMetricsCollector
	registries map[*prometheus.Registry]struct{}
}{}

type queueMetricsCollector struct {
	workerReady      *prometheus.Desc
	queueReady       *prometheus.Desc
	messages         *prometheus.Desc
	events           *prometheus.Desc
	payloadLimitSize *prometheus.Desc
}

func registerQueueMetrics(registry *prometheus.Registry) error {
	if registry == nil {
		return nil
	}

	queueMetrics.Lock()
	defer queueMetrics.Unlock()

	if queueMetrics.collector == nil {
		queueMetrics.collector = newQueueMetricsCollector()
	}
	if queueMetrics.registries == nil {
		queueMetrics.registries = make(map[*prometheus.Registry]struct{})
	}
	if _, ok := queueMetrics.registries[registry]; ok {
		return nil
	}

	if err := registry.Register(queueMetrics.collector); err != nil {
		var already prometheus.AlreadyRegisteredError
		if !errors.As(err, &already) {
			return err
		}
	}
	queueMetrics.registries[registry] = struct{}{}

	return nil
}

func newQueueMetricsCollector() *queueMetricsCollector {
	const namespace, subsystem = "caddy", "pogo_queue"

	return &queueMetricsCollector{
		workerReady: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "worker_ready"),
			"Whether the queue worker pool has started and is not shutting down.",
			nil,
			nil,
		),
		queueReady: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "queue_ready"),
			"Whether the configured queue backend can report status for this queue.",
			[]string{"queue"},
			nil,
		),
		messages: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "messages"),
			"Number of queue messages by state.",
			[]string{"queue", "state"},
			nil,
		),
		events: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "events_total"),
			"Queue lifecycle and rejection events observed by the module.",
			[]string{"event"},
			nil,
		),
		payloadLimitSize: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "payload_limit_bytes"),
			"Configured maximum queue payload size in bytes.",
			[]string{"queue"},
			nil,
		),
	}
}

func (c *queueMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.workerReady
	ch <- c.queueReady
	ch <- c.messages
	ch <- c.events
	ch <- c.payloadLimitSize
}

func (c *queueMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	snapshot, ok := currentQueueMetrics()
	if !ok {
		ch <- prometheus.MustNewConstMetric(c.workerReady, prometheus.GaugeValue, 0)
		return
	}

	status := snapshot.status
	ch <- prometheus.MustNewConstMetric(c.workerReady, prometheus.GaugeValue, metricBool(status.Ready))
	for _, stat := range status.Queues {
		ch <- prometheus.MustNewConstMetric(c.queueReady, prometheus.GaugeValue, metricBool(stat.Ready), stat.Queue)
		ch <- prometheus.MustNewConstMetric(c.messages, prometheus.GaugeValue, float64(stat.Pending), stat.Queue, "pending")
		ch <- prometheus.MustNewConstMetric(c.messages, prometheus.GaugeValue, float64(stat.Delayed), stat.Queue, "delayed")
		ch <- prometheus.MustNewConstMetric(c.messages, prometheus.GaugeValue, float64(stat.Reserved), stat.Queue, "reserved")
		ch <- prometheus.MustNewConstMetric(c.messages, prometheus.GaugeValue, float64(stat.Failed), stat.Queue, "failed")
		ch <- prometheus.MustNewConstMetric(c.payloadLimitSize, prometheus.GaugeValue, float64(stat.MaxPayloadBytes), stat.Queue)
	}

	for _, event := range queueMetricEvents(snapshot.counters) {
		ch <- prometheus.MustNewConstMetric(c.events, prometheus.CounterValue, float64(event.value), event.name)
	}
}

type queueMetricsSnapshot struct {
	status   statusPayload
	counters backendCounterSnapshot
}

func currentQueueMetrics() (queueMetricsSnapshot, bool) {
	globalManagerMu.RLock()
	m := globalManager
	globalManagerMu.RUnlock()
	if m == nil {
		return queueMetricsSnapshot{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), queueMetricsScrapeTimeout)
	defer cancel()

	return queueMetricsSnapshot{
		status:   m.status(ctx, ""),
		counters: m.counters(),
	}, true
}

type queueMetricEvent struct {
	name  string
	value uint64
}

func queueMetricEvents(counters backendCounterSnapshot) []queueMetricEvent {
	return []queueMetricEvent{
		{name: "enqueued", value: counters.Enqueued},
		{name: "reserved", value: counters.Reserved},
		{name: "acked", value: counters.Acked},
		{name: "released", value: counters.Released},
		{name: "failed", value: counters.Failed},
		{name: "dropped_full", value: counters.DroppedFull},
		{name: "dropped_payload_too_large", value: counters.DroppedPayload},
		{name: "dropped_shutdown", value: counters.DroppedShutdown},
		{name: "backend_errors", value: counters.BackendErrors},
	}
}

func metricBool(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
