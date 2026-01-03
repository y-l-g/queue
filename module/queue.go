package queue

import (
	"C"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dunglas/frankenphp"
)

type dispatcher struct {
	worker frankenphp.Workers
	logger *slog.Logger
	queue  chan string
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

func newDispatcher(w frankenphp.Workers, l *slog.Logger, size int) *dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	d := &dispatcher{
		worker: w,
		logger: l,
		queue:  make(chan string, size),
		ctx:    ctx,
		cancel: cancel,
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := d.worker.SendMessage(ctx, msg, nil)
			cancel()

			if err != nil {
				d.logger.Error("pogo_queue: failed to send message to worker", slog.Any("error", err))
			}
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *dispatcher) shutdown() {
	d.once.Do(func() {
		d.cancel()
		d.wg.Wait()
	})
}

func (d *dispatcher) trySend(msg string) bool {
	select {
	case d.queue <- msg:
		return true
	default:
		d.logger.Warn("pogo_queue: buffer full, dropping message")
		return false
	}
}

func dispatch(charPtr *C.char, length C.size_t) C.int {
	msg := C.GoStringN(charPtr, C.int(length))

	globalDispatcherMu.RLock()
	d := globalDispatcher
	globalDispatcherMu.RUnlock()

	if d == nil {
		return 0
	}

	if d.trySend(msg) {
		return 1
	}

	return 0
}
