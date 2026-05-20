package queue

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dunglas/frankenphp"
)

type manager struct {
	backend           backend
	worker            frankenphp.Workers
	logger            *slog.Logger
	queues            []string
	consumer          string
	concurrency       int
	reserveTimeout    time.Duration
	visibilityTimeout time.Duration
	shutdownTimeout   time.Duration
	maxAttempts       int
	maxPayloadBytes   int

	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	closing         atomic.Bool
	droppedShutdown atomic.Uint64
	once            sync.Once
}

type deliveryEnvelope struct {
	ID       string `json:"id"`
	Queue    string `json:"queue"`
	Payload  string `json:"payload"`
	Attempts int    `json:"attempts"`
}

type pushResult struct {
	OK      bool   `json:"ok"`
	ID      string `json:"id,omitempty"`
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type statusPayload struct {
	Ready  bool         `json:"ready"`
	Queues []queueStats `json:"queues"`
}

var (
	globalManager   *manager
	globalManagerMu sync.RWMutex
)

func newManager(b backend, w frankenphp.Workers, logger *slog.Logger, queues []string, consumer string, concurrency int, reserveTimeout time.Duration, visibilityTimeout time.Duration, shutdownTimeout time.Duration, maxAttempts int, maxPayloadBytes int) *manager {
	if concurrency <= 0 {
		concurrency = 1
	}
	if reserveTimeout <= 0 {
		reserveTimeout = time.Second
	}
	if visibilityTimeout <= 0 {
		visibilityTimeout = 90 * time.Second
	}
	if shutdownTimeout <= 0 {
		shutdownTimeout = 30 * time.Second
	}
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	if maxPayloadBytes <= 0 {
		maxPayloadBytes = defaultMaxMessageBytes
	}
	if consumer == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "consumer"
		}
		consumer = fmt.Sprintf("%s-%d", host, os.Getpid())
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &manager{
		backend:           b,
		worker:            w,
		logger:            logger,
		queues:            queues,
		consumer:          consumer,
		concurrency:       concurrency,
		reserveTimeout:    reserveTimeout,
		visibilityTimeout: visibilityTimeout,
		shutdownTimeout:   shutdownTimeout,
		maxAttempts:       maxAttempts,
		maxPayloadBytes:   maxPayloadBytes,
		ctx:               ctx,
		cancel:            cancel,
	}

	for i := 0; i < concurrency; i++ {
		m.wg.Add(1)
		go m.loop(i + 1)
	}

	return m
}

func (m *manager) loop(workerIndex int) {
	defer m.wg.Done()

	consumer := fmt.Sprintf("%s-%d", m.consumer, workerIndex)
	for {
		if m.closing.Load() {
			return
		}

		msg, err := m.backend.Reserve(m.ctx, m.queues, consumer, m.reserveTimeout)
		if err != nil {
			if errors.Is(err, errQueueEmpty) || errors.Is(err, context.Canceled) {
				continue
			}
			m.logger.Error("pogo_queue: reserve failed", slog.Any("error", err))
			time.Sleep(250 * time.Millisecond)
			continue
		}

		if err := m.sendMessage(msg); err != nil {
			m.logger.Error("pogo_queue: worker delivery failed", slog.String("queue", msg.Queue), slog.String("id", msg.ID), slog.Any("error", err))
			_ = m.handleDeliveryError(msg, err)
		}
	}
}

func (m *manager) sendMessage(msg *delivery) error {
	payload, err := json.Marshal(deliveryEnvelope{
		ID:       msg.ID,
		Queue:    msg.Queue,
		Payload:  msg.Payload,
		Attempts: msg.Attempts,
	})
	if err != nil {
		return err
	}

	msgCtx, cancel := context.WithTimeout(context.Background(), m.visibilityTimeout)
	defer cancel()

	_, err = m.worker.SendMessage(msgCtx, string(payload), nil)
	return err
}

func (m *manager) handleDeliveryError(msg *delivery, err error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if msg.Attempts >= m.maxAttempts {
		_, failErr := m.backend.Fail(ctx, msg.Queue, msg.ID, err.Error())
		return failErr
	}

	_, releaseErr := m.backend.Release(ctx, msg.Queue, msg.ID, 0)
	return releaseErr
}

func (m *manager) shutdown() {
	m.once.Do(func() {
		m.closing.Store(true)
		m.cancel()

		done := make(chan struct{})
		go func() {
			m.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(m.shutdownTimeout):
			m.logger.Warn("pogo_queue: shutdown timed out with workers still running")
		}

		if err := m.backend.Close(); err != nil {
			m.logger.Error("pogo_queue: backend close failed", slog.Any("error", err))
		}
	})
}

func (m *manager) enqueue(ctx context.Context, queue, payload string, delay time.Duration) (string, int, error) {
	if m.closing.Load() {
		m.droppedShutdown.Add(1)
		return "", dispatchResultShuttingDown, fmt.Errorf("pogo_queue is shutting down")
	}
	return m.backend.Enqueue(ctx, queue, payload, delay)
}

func (m *manager) status(ctx context.Context, queue string) statusPayload {
	queues := m.queues
	if queue != "" {
		queues = []string{queue}
	}

	stats := make([]queueStats, 0, len(queues))
	for _, name := range queues {
		stat, err := m.backend.Stats(ctx, name)
		if err != nil {
			stat = queueStats{
				Queue:           name,
				Ready:           false,
				BackendErrors:   1,
				MaxPayloadBytes: m.maxPayloadBytes,
			}
		}
		stat.DroppedShutdown += m.droppedShutdown.Load()
		stats = append(stats, stat)
	}

	return statusPayload{
		Ready:  !m.closing.Load(),
		Queues: stats,
	}
}

func enqueue(queue, payload string, delaySeconds int64) *C.char {
	globalManagerMu.RLock()
	m := globalManager
	globalManagerMu.RUnlock()

	if m == nil {
		return jsonCString(pushResult{
			OK:      false,
			Code:    dispatchResultWorkerUnavailable,
			Message: "pogo_queue worker is unavailable",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, code, err := m.enqueue(ctx, queue, payload, time.Duration(delaySeconds)*time.Second)
	if err != nil || code != dispatchResultAccepted {
		message := "dispatch failed"
		if err != nil {
			message = err.Error()
		}
		return jsonCString(pushResult{OK: false, Code: code, Message: message})
	}

	return jsonCString(pushResult{OK: true, ID: id, Code: code})
}

func dispatch(charPtr *C.char, length C.size_t) C.int {
	payload, ok := goStringFromC(charPtr, length)
	if !ok {
		return dispatchResultPayloadTooLarge
	}

	globalManagerMu.RLock()
	m := globalManager
	globalManagerMu.RUnlock()
	if m == nil {
		return dispatchResultWorkerUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, code, _ := m.enqueue(ctx, "default", payload, 0)
	return C.int(code)
}

func ack(queue, id string) C.int {
	return managerAction(queue, id, func(ctx context.Context, m *manager) (int, error) {
		return m.backend.Ack(ctx, queue, id)
	})
}

func release(queue, id string, delaySeconds int64) C.int {
	return managerAction(queue, id, func(ctx context.Context, m *manager) (int, error) {
		return m.backend.Release(ctx, queue, id, time.Duration(delaySeconds)*time.Second)
	})
}

func failDelivery(queue, id, reason string) C.int {
	return managerAction(queue, id, func(ctx context.Context, m *manager) (int, error) {
		return m.backend.Fail(ctx, queue, id, reason)
	})
}

func managerAction(queue, id string, action func(context.Context, *manager) (int, error)) C.int {
	if queue == "" || id == "" {
		return dispatchResultBackendFailure
	}

	globalManagerMu.RLock()
	m := globalManager
	globalManagerMu.RUnlock()
	if m == nil {
		return dispatchResultWorkerUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	code, err := action(ctx, m)
	if err != nil {
		m.logger.Error("pogo_queue: lifecycle action failed", slog.String("queue", queue), slog.String("id", id), slog.Any("error", err))
	}
	return C.int(code)
}

func queueStatsJSON(queue string) *C.char {
	globalManagerMu.RLock()
	m := globalManager
	globalManagerMu.RUnlock()

	if m == nil {
		return jsonCString(statusPayload{Ready: false, Queues: []queueStats{}})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return jsonCString(m.status(ctx, queue))
}

func jsonCString(value any) *C.char {
	data, err := json.Marshal(value)
	if err != nil {
		return C.CString(`{"ready":false,"queues":[]}`)
	}
	return C.CString(string(data))
}

func goStringFromC(ptr *C.char, length C.size_t) (string, bool) {
	if ptr == nil {
		return "", true
	}
	if uint64(length) > math.MaxInt32 {
		return "", false
	}
	return C.GoStringN(ptr, C.int(length)), true
}
