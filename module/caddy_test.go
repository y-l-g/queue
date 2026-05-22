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
