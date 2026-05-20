package queue

import (
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	frankenphpCaddy "github.com/dunglas/frankenphp/caddy"
)

func init() {
	caddy.RegisterModule(Queue{})
	httpcaddyfile.RegisterGlobalOption("pogo_queue", parseGlobalOption)
}

type Queue struct {
	Backend           backendConfig  `json:"backend,omitempty"`
	Queues            []string       `json:"queues,omitempty"`
	Worker            string         `json:"worker,omitempty"`
	Concurrency       int            `json:"concurrency,omitempty"`
	WorkerThreads     int            `json:"worker_threads,omitempty"`
	MaxPayloadBytes   int            `json:"max_payload_bytes,omitempty"`
	VisibilityTimeout caddy.Duration `json:"visibility_timeout,omitempty"`
	ReserveTimeout    caddy.Duration `json:"reserve_timeout,omitempty"`
	ShutdownTimeout   caddy.Duration `json:"shutdown_timeout,omitempty"`
	MaxAttempts       int            `json:"max_attempts,omitempty"`

	manager *manager
}

func (Queue) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "pogo_queue",
		New: func() caddy.Module { return new(Queue) },
	}
}

func (g *Queue) Provision(ctx caddy.Context) error {
	if len(g.Queues) == 0 {
		g.Queues = []string{"default"}
	}
	if g.Worker == "" {
		g.Worker = "queue-worker.php"
	}
	if g.Concurrency <= 0 {
		g.Concurrency = 1
	}
	if g.MaxPayloadBytes <= 0 {
		g.MaxPayloadBytes = defaultMaxMessageBytes
	}
	if g.VisibilityTimeout <= 0 {
		g.VisibilityTimeout = caddy.Duration(90 * time.Second)
	}
	if g.ReserveTimeout <= 0 {
		g.ReserveTimeout = caddy.Duration(time.Second)
	}
	if g.ShutdownTimeout <= 0 {
		g.ShutdownTimeout = caddy.Duration(30 * time.Second)
	}
	if g.MaxAttempts <= 0 {
		g.MaxAttempts = defaultMaxAttempts
	}

	b, err := newBackend(
		g.Backend,
		g.Queues,
		g.MaxPayloadBytes,
		time.Duration(g.VisibilityTimeout),
		g.MaxAttempts,
		ctx.Slogger(),
	)
	if err != nil {
		return err
	}
	if err := b.Start(ctx); err != nil {
		return err
	}

	workerThreads := g.WorkerThreads
	if workerThreads <= 0 {
		workerThreads = g.Concurrency
	}
	workers := frankenphpCaddy.RegisterWorkers("pogo_queue", g.Worker, workerThreads)

	g.manager = newManager(
		b,
		workers,
		ctx.Slogger(),
		g.Queues,
		g.Backend.Consumer,
		g.Concurrency,
		time.Duration(g.ReserveTimeout),
		time.Duration(g.VisibilityTimeout),
		time.Duration(g.ShutdownTimeout),
		g.MaxAttempts,
		g.MaxPayloadBytes,
	)

	globalManagerMu.Lock()
	if globalManager != nil {
		go globalManager.shutdown()
	}
	globalManager = g.manager
	globalManagerMu.Unlock()

	return nil
}

func (g *Queue) Cleanup() error {
	if g.manager != nil {
		g.manager.shutdown()
	}

	globalManagerMu.Lock()
	if globalManager == g.manager {
		globalManager = nil
	}
	globalManagerMu.Unlock()

	return nil
}

func (g *Queue) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "backend":
				if !d.NextArg() {
					return d.ArgErr()
				}
				g.Backend.Type = d.Val()
				nesting := d.Nesting()
				for d.NextBlock(nesting) {
					if err := g.parseBackendDirective(d); err != nil {
						return err
					}
				}
			case "worker":
				if !d.NextArg() {
					return d.ArgErr()
				}
				g.Worker = d.Val()
			case "queues":
				if !d.NextArg() {
					return d.ArgErr()
				}
				g.Queues = splitQueueNames(d.Val())
			case "concurrency":
				value, err := parsePositiveIntDirective(d, "concurrency")
				if err != nil {
					return err
				}
				g.Concurrency = value
			case "worker_threads", "num_threads", "min_threads":
				value, err := parsePositiveIntDirective(d, d.Val())
				if err != nil {
					return err
				}
				g.WorkerThreads = value
			case "max_payload_bytes", "max_message_bytes":
				value, err := parsePositiveIntDirective(d, d.Val())
				if err != nil {
					return err
				}
				g.MaxPayloadBytes = value
			case "visibility_timeout":
				value, err := parseDurationDirective(d, "visibility_timeout")
				if err != nil {
					return err
				}
				g.VisibilityTimeout = caddy.Duration(value)
			case "reserve_timeout":
				value, err := parseDurationDirective(d, "reserve_timeout")
				if err != nil {
					return err
				}
				g.ReserveTimeout = caddy.Duration(value)
			case "shutdown_timeout":
				value, err := parseDurationDirective(d, "shutdown_timeout")
				if err != nil {
					return err
				}
				g.ShutdownTimeout = caddy.Duration(value)
			case "max_attempts":
				value, err := parsePositiveIntDirective(d, "max_attempts")
				if err != nil {
					return err
				}
				g.MaxAttempts = value
			case "size":
				value, err := parsePositiveIntDirective(d, "size")
				if err != nil {
					return err
				}
				g.Backend.MaxMessages = value
			default:
				return d.Errf(`unrecognized subdirective "%s"`, d.Val())
			}
		}
	}

	return nil
}

func (g *Queue) parseBackendDirective(d *caddyfile.Dispenser) error {
	switch d.Val() {
	case "url":
		if !d.NextArg() {
			return d.ArgErr()
		}
		g.Backend.RedisURL = d.Val()
	case "key_prefix":
		if !d.NextArg() {
			return d.ArgErr()
		}
		g.Backend.KeyPrefix = d.Val()
	case "group":
		if !d.NextArg() {
			return d.ArgErr()
		}
		g.Backend.Group = d.Val()
	case "consumer":
		if !d.NextArg() {
			return d.ArgErr()
		}
		g.Backend.Consumer = d.Val()
	case "tls":
		if !d.NextArg() {
			return d.ArgErr()
		}
		value, err := strconv.ParseBool(d.Val())
		if err != nil {
			return d.Errf("failed to parse tls: %v", err)
		}
		g.Backend.TLS = value
	case "max_messages":
		value, err := parsePositiveIntDirective(d, "max_messages")
		if err != nil {
			return err
		}
		g.Backend.MaxMessages = value
	case "max_total_bytes":
		value, err := parsePositiveIntDirective(d, "max_total_bytes")
		if err != nil {
			return err
		}
		g.Backend.MaxTotalBytes = value
	default:
		return d.Errf(`unrecognized backend subdirective "%s"`, d.Val())
	}
	return nil
}

func parsePositiveIntDirective(d *caddyfile.Dispenser, name string) (int, error) {
	if !d.NextArg() {
		return 0, d.ArgErr()
	}
	value, err := strconv.Atoi(d.Val())
	if err != nil || value <= 0 {
		return 0, d.Errf("failed to parse %s as a positive integer", name)
	}
	return value, nil
}

func parseDurationDirective(d *caddyfile.Dispenser, name string) (time.Duration, error) {
	if !d.NextArg() {
		return 0, d.ArgErr()
	}
	value, err := time.ParseDuration(d.Val())
	if err != nil || value <= 0 {
		return 0, d.Errf("failed to parse %s as a positive duration", name)
	}
	return value, nil
}

func parseGlobalOption(d *caddyfile.Dispenser, _ any) (any, error) {
	app := &Queue{}
	if err := app.UnmarshalCaddyfile(d); err != nil {
		return nil, err
	}

	return httpcaddyfile.App{
		Name:  "pogo_queue",
		Value: caddyconfig.JSON(app, nil),
	}, nil
}

var (
	_ caddy.Module       = (*Queue)(nil)
	_ caddy.Provisioner  = (*Queue)(nil)
	_ caddy.CleanerUpper = (*Queue)(nil)
)
