package queue

import (
	"slices"
	"testing"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func TestUnmarshalCaddyfileQueuesAcceptsMultipleArgs(t *testing.T) {
	d := caddyfile.NewTestDispenser(`pogo_queue {
		queues default mail notifications
	}`)
	var q Queue

	if err := q.UnmarshalCaddyfile(d); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	want := []string{"default", "mail", "notifications"}
	if !slices.Equal(q.Queues, want) {
		t.Fatalf("expected queues %v, got %v", want, q.Queues)
	}
}

func TestUnmarshalCaddyfileParsesBackendBlock(t *testing.T) {
	d := caddyfile.NewTestDispenser(`pogo_queue {
		backend redis {
			url redis://localhost:6379/0
			key_prefix app
			group workers
			consumer node-1
			tls true
		}
	}`)
	var q Queue

	if err := q.UnmarshalCaddyfile(d); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if q.Backend.Type != "redis" {
		t.Fatalf("expected redis backend, got %q", q.Backend.Type)
	}
	if q.Backend.RedisURL != "redis://localhost:6379/0" {
		t.Fatalf("unexpected redis URL: %q", q.Backend.RedisURL)
	}
	if q.Backend.KeyPrefix != "app" || q.Backend.Group != "workers" || q.Backend.Consumer != "node-1" || !q.Backend.TLS {
		t.Fatalf("unexpected backend config: %#v", q.Backend)
	}
}

func TestUnmarshalCaddyfileRejectsExtraSingleDirectiveArgs(t *testing.T) {
	tests := []string{
		`pogo_queue {
			worker queue-worker.php extra
		}`,
		`pogo_queue {
			concurrency 1 2
		}`,
		`pogo_queue {
			visibility_timeout 90s 120s
		}`,
		`pogo_queue {
			backend redis extra {
				url redis://localhost:6379/0
			}
		}`,
		`pogo_queue {
			backend redis {
				url redis://localhost:6379/0 extra
			}
		}`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			d := caddyfile.NewTestDispenser(input)
			var q Queue

			if err := q.UnmarshalCaddyfile(d); err == nil {
				t.Fatal("expected unmarshal to fail")
			}
		})
	}
}

func TestUnmarshalCaddyfileRejectsLegacyAliases(t *testing.T) {
	tests := []string{
		`pogo_queue {
			num_threads 2
		}`,
		`pogo_queue {
			min_threads 2
		}`,
		`pogo_queue {
			max_message_bytes 1024
		}`,
		`pogo_queue {
			size 10
		}`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			d := caddyfile.NewTestDispenser(input)
			var q Queue

			if err := q.UnmarshalCaddyfile(d); err == nil {
				t.Fatal("expected unmarshal to fail")
			}
		})
	}
}
