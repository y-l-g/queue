package queue

import (
	"strconv"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	frankenphpCaddy "github.com/dunglas/frankenphp/caddy"
)

var (
	globalDispatcher   *dispatcher
	globalDispatcherMu sync.RWMutex
)

func init() {
	caddy.RegisterModule(Queue{})
	httpcaddyfile.RegisterGlobalOption("pogo_queue", parseGlobalOption)
}

type Queue struct {
	Size       int    `json:"size,omitempty"`
	NumThreads int    `json:"numthreads,omitempty"`
	Name       string `json:"name,omitempty"`
	Worker     string `json:"worker,omitempty"`

	dispatcher *dispatcher
}

func (Queue) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "pogo_queue",
		New: func() caddy.Module { return new(Queue) },
	}
}

func (g *Queue) Provision(ctx caddy.Context) error {
	if g.Size <= 0 {
		g.Size = 10_000
	}

	if g.Name == "" {
		g.Name = "m#Queue"
	}

	if g.Worker == "" {
		g.Worker = "queue-worker.php"
	}

	w := frankenphpCaddy.RegisterWorkers(g.Name, g.Worker, g.NumThreads)
	g.dispatcher = newDispatcher(w, ctx.Slogger(), g.Size)

	globalDispatcherMu.Lock()
	if globalDispatcher != nil {
		go globalDispatcher.shutdown()
	}
	globalDispatcher = g.dispatcher
	globalDispatcherMu.Unlock()

	return nil
}

func (g *Queue) Cleanup() error {
	if g.dispatcher != nil {
		g.dispatcher.shutdown()
	}

	globalDispatcherMu.Lock()
	if globalDispatcher == g.dispatcher {
		globalDispatcher = nil
	}
	globalDispatcherMu.Unlock()

	return nil
}

func (g *Queue) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "worker":
				if !d.NextArg() {
					return d.ArgErr()
				}
				g.Worker = d.Val()
			case "name":
				if !d.NextArg() {
					return d.ArgErr()
				}
				g.Name = d.Val()
			case "size":
				if !d.NextArg() {
					return d.ArgErr()
				}
				s, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.Errf("failed to parse size: %v", err)
				}
				g.Size = s
			case "num_threads", "min_threads":
				if !d.NextArg() {
					return d.ArgErr()
				}
				t, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.Errf("failed to parse num_threads: %v", err)
				}
				g.NumThreads = t
			default:
				return d.Errf(`unrecognized subdirective "%s"`, d.Val())
			}
		}
	}

	return nil
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
