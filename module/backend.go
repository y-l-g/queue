package queue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	dispatchResultAccepted          = 1
	dispatchResultFull              = 0
	dispatchResultWorkerUnavailable = 2
	dispatchResultPayloadTooLarge   = 3
	dispatchResultShuttingDown      = 4
	dispatchResultBackendFailure    = 5
	dispatchResultQueueUnknown      = 6
)

const (
	defaultMaxMessageBytes = 1 << 20 // 1MB
	defaultMaxAttempts     = 3
	defaultFailedJobsLimit = 100
	maxFailedJobsLimit     = 1000
)

var errQueueEmpty = errors.New("queue is empty")

type delivery struct {
	ID       string `json:"id"`
	Queue    string `json:"queue"`
	Payload  string `json:"payload"`
	Attempts int    `json:"attempts"`
}

type queueStats struct {
	Queue           string `json:"queue"`
	Ready           bool   `json:"ready"`
	Pending         int64  `json:"pending"`
	Delayed         int64  `json:"delayed"`
	Reserved        int64  `json:"reserved"`
	Failed          int64  `json:"failed"`
	Enqueued        uint64 `json:"enqueued"`
	ReservedTotal   uint64 `json:"reserved_total"`
	Acked           uint64 `json:"acked"`
	Released        uint64 `json:"released"`
	FailedTotal     uint64 `json:"failed_total"`
	DroppedFull     uint64 `json:"dropped_full"`
	DroppedPayload  uint64 `json:"dropped_payload_too_large"`
	DroppedShutdown uint64 `json:"dropped_shutdown"`
	BackendErrors   uint64 `json:"backend_errors"`
	MaxPayloadBytes int    `json:"max_payload_bytes"`
}

type failedJob struct {
	ID         string `json:"id"`
	Queue      string `json:"queue"`
	OriginalID string `json:"original_id,omitempty"`
	Payload    string `json:"payload"`
	Reason     string `json:"reason,omitempty"`
	FailedAt   string `json:"failed_at,omitempty"`
}

type backendCounterSnapshot struct {
	Enqueued        uint64
	Reserved        uint64
	Acked           uint64
	Released        uint64
	Failed          uint64
	DroppedFull     uint64
	DroppedPayload  uint64
	DroppedShutdown uint64
	BackendErrors   uint64
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

func (c *backendCounters) snapshot() backendCounterSnapshot {
	return backendCounterSnapshot{
		Enqueued:       c.enqueued.Load(),
		Reserved:       c.reserved.Load(),
		Acked:          c.acked.Load(),
		Released:       c.released.Load(),
		Failed:         c.failed.Load(),
		DroppedFull:    c.droppedFull.Load(),
		DroppedPayload: c.payloadTooLarge.Load(),
		BackendErrors:  c.backendErrors.Load(),
	}
}

type backend interface {
	Start(context.Context) error
	Enqueue(context.Context, string, string, time.Duration) (string, int, error)
	Reserve(context.Context, []string, string, time.Duration) (*delivery, error)
	Ack(context.Context, string, string) (int, error)
	Release(context.Context, string, string, time.Duration) (int, error)
	Fail(context.Context, string, string, string) (int, error)
	Stats(context.Context, string) (queueStats, error)
	Counters() backendCounterSnapshot
	ListFailed(context.Context, string, int64) ([]failedJob, int, error)
	RetryFailed(context.Context, string, string) (string, int, error)
	ForgetFailed(context.Context, string, string) (int, error)
	PurgeFailed(context.Context, string) (int64, int, error)
	Close() error
}

type backendConfig struct {
	Type          string `json:"type,omitempty"`
	RedisURL      string `json:"redis_url,omitempty"`
	KeyPrefix     string `json:"key_prefix,omitempty"`
	Group         string `json:"group,omitempty"`
	Consumer      string `json:"consumer,omitempty"`
	TLS           bool   `json:"tls,omitempty"`
	MaxMessages   int    `json:"max_messages,omitempty"`
	MaxTotalBytes int    `json:"max_total_bytes,omitempty"`
}

func newBackend(cfg backendConfig, queues []string, maxPayloadBytes int, visibilityTimeout time.Duration, maxAttempts int) (backend, error) {
	if cfg.Type == "" {
		cfg.Type = "redis"
	}

	switch cfg.Type {
	case "redis":
		if cfg.RedisURL == "" {
			cfg.RedisURL = os.Getenv("POGO_REDIS_URL")
		}
		if cfg.RedisURL == "" {
			return nil, fmt.Errorf("redis backend requires a url or POGO_REDIS_URL")
		}
		if cfg.KeyPrefix == "" {
			cfg.KeyPrefix = "pogo"
		}
		if cfg.Group == "" {
			cfg.Group = "default"
		}
		if cfg.Consumer == "" {
			host, _ := os.Hostname()
			if host == "" {
				host = "consumer"
			}
			cfg.Consumer = host + "-" + strconv.Itoa(os.Getpid())
		}
		return newRedisBackend(cfg, queues, maxPayloadBytes, visibilityTimeout, maxAttempts)
	case "memory":
		if cfg.MaxMessages <= 0 {
			cfg.MaxMessages = 1_000
		}
		if cfg.MaxTotalBytes <= 0 {
			cfg.MaxTotalBytes = 64 << 20
		}
		return newMemoryBackend(cfg, queues, maxPayloadBytes, visibilityTimeout, maxAttempts), nil
	default:
		return nil, fmt.Errorf("unsupported pogo_queue backend %q", cfg.Type)
	}
}

func splitQueueNames(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})

	queues := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		queue := strings.TrimSpace(part)
		if queue == "" {
			continue
		}
		if _, ok := seen[queue]; ok {
			continue
		}
		seen[queue] = struct{}{}
		queues = append(queues, queue)
	}

	if len(queues) == 0 {
		return []string{"default"}
	}

	return queues
}
