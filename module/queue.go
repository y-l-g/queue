package queue

import (
	"C"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp"
)

const (
	dispatchResultAccepted          = 1
	dispatchResultFull              = 0
	dispatchResultWorkerUnavailable = 2
	dispatchResultPayloadTooLarge   = 3
	dispatchResultShuttingDown      = 4
)

const defaultMaxMessageBytes = 1 << 20 // 1MB

type dispatcherStats struct {
	enqueued        atomic.Uint64
	dispatched      atomic.Uint64
	droppedFull     atomic.Uint64
	payloadTooLarge atomic.Uint64
	droppedShutdown atomic.Uint64
	sendErrors      atomic.Uint64
}

type dispatcher struct {
	worker frankenphp.Workers
	logger *slog.Logger
	queue  chan string
	ctx    context.Context
	cancel context.CancelFunc

	closing         atomic.Bool
	maxMessageBytes int
	wg              sync.WaitGroup
	stats           dispatcherStats
	once            sync.Once
}

type queueStats struct {
	Enqueued        uint64 `json:"enqueued"`
	Dispatched      uint64 `json:"dispatched"`
	DroppedFull     uint64 `json:"dropped_full"`
	DroppedPayload  uint64 `json:"dropped_payload_too_large"`
	DroppedShutdown uint64 `json:"dropped_shutdown"`
	SendErrors      uint64 `json:"send_errors"`
	CurrentDepth    uint64 `json:"current_depth"`
	MaxMessageBytes int    `json:"max_message_bytes"`
	DriverReady     bool   `json:"driver_ready"`
}

func newDispatcher(w frankenphp.Workers, l *slog.Logger, size int, maxMessageBytes int) *dispatcher {
	if size <= 0 {
		size = 10_000
	}

	if maxMessageBytes <= 0 {
		maxMessageBytes = defaultMaxMessageBytes
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := &dispatcher{
		worker:          w,
		logger:          l,
		queue:           make(chan string, size),
		ctx:             ctx,
		cancel:          cancel,
		maxMessageBytes: maxMessageBytes,
	}

	d.wg.Add(1)
	go d.loop()

	return d
}

func (d *dispatcher) loop() {
	defer d.wg.Done()

	for {
		select {
		case msg := <-d.queue:
			if err := d.sendMessage(msg); err != nil {
				d.stats.sendErrors.Add(1)
			}
		case <-d.ctx.Done():
			for {
				select {
				case msg := <-d.queue:
					d.sendMessage(msg) // best effort on shutdown
				default:
					d.logger.Info(
						"pogo_queue: shutdown complete",
						slog.Uint64("dispatched", d.stats.dispatched.Load()),
						slog.Uint64("enqueued", d.stats.enqueued.Load()),
						slog.Uint64("send_errors", d.stats.sendErrors.Load()),
					)
					return
				}
			}
		}
	}
}

func (d *dispatcher) shutdown() {
	d.once.Do(func() {
		d.closing.Store(true)
		d.cancel()
		d.wg.Wait()
	})
}

func (d *dispatcher) sendMessage(msg string) error {
	msgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := d.worker.SendMessage(msgCtx, msg, nil)
	if err != nil {
		d.logger.Error("pogo_queue: failed to send message to worker", slog.Any("error", err))
		return err
	}

	d.stats.dispatched.Add(1)

	return nil
}

func (d *dispatcher) statsSnapshot() queueStats {
	return queueStats{
		Enqueued:        d.stats.enqueued.Load(),
		Dispatched:      d.stats.dispatched.Load(),
		DroppedFull:     d.stats.droppedFull.Load(),
		DroppedPayload:  d.stats.payloadTooLarge.Load(),
		DroppedShutdown: d.stats.droppedShutdown.Load(),
		SendErrors:      d.stats.sendErrors.Load(),
		CurrentDepth:    uint64(len(d.queue)),
		MaxMessageBytes: d.maxMessageBytes,
		DriverReady:     !d.closing.Load(),
	}
}

func (d *dispatcher) trySend(msg string) int {
	if d == nil {
		return dispatchResultWorkerUnavailable
	}

	if d.closing.Load() {
		d.stats.droppedShutdown.Add(1)
		return dispatchResultShuttingDown
	}

	if d.maxMessageBytes > 0 && len(msg) > d.maxMessageBytes {
		d.stats.payloadTooLarge.Add(1)
		return dispatchResultPayloadTooLarge
	}

	select {
	case d.queue <- msg:
		d.stats.enqueued.Add(1)
		return dispatchResultAccepted
	default:
		d.stats.droppedFull.Add(1)
		d.logger.Warn("pogo_queue: buffer full, dropping message")
		return dispatchResultFull
	}
}

func dispatch(charPtr *C.char, length C.size_t) C.int {
	msg := C.GoStringN(charPtr, C.int(length))

	globalDispatcherMu.RLock()
	d := globalDispatcher
	globalDispatcherMu.RUnlock()

	if d == nil {
		return dispatchResultWorkerUnavailable
	}

	return C.int(d.trySend(msg))
}

func queueStatsJSON() *C.char {
	globalDispatcherMu.RLock()
	d := globalDispatcher
	globalDispatcherMu.RUnlock()

	if d == nil {
		return C.CString("{}")
	}

	data, err := json.Marshal(d.statsSnapshot())
	if err != nil {
		return C.CString("{}")
	}

	return C.CString(string(data))
}
