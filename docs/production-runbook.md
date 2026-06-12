# Production Runbook

This runbook covers operating FrankenPHP Queue with the Redis backend. The
memory backend is only for local development, demos, and tests.

## Operating Contract

- Redis Streams is the only production backend.
- Delivery is at least once. Exactly-once delivery is not provided.
- Job handlers must be idempotent because a job can run more than once.
- The queue module owns reservation, acknowledgement, release, delay, failed
  delivery, and pending-message reclaim semantics.
- Redis keys are derived from `key_prefix`, queue name, and purpose:
  `<prefix>:<queue>:stream`, `<prefix>:<queue>:delayed`, and
  `<prefix>:<queue>:failed`.

## Redis

Use a Redis deployment that is treated as durable infrastructure, not as a
best-effort cache.

- Enable persistence. Use AOF, RDB snapshots, or both according to your data
  loss tolerance.
- Prefer a dedicated Redis database or key prefix for queues.
- Use `maxmemory-policy noeviction` for Redis instances that contain queue data.
  Evicting stream or sorted-set keys can drop jobs.
- Monitor Redis memory, blocked clients, replication health, command latency,
  and persistence errors.
- Use TLS when Redis traffic crosses an untrusted network.
- Treat Redis failover as an at-least-once boundary. An acknowledged job can be
  seen again if a failover loses the acknowledgement, and a just-enqueued job can
  be lost if it was not persisted or replicated before the failure.

## Timeouts And Attempts

`visibility_timeout` is the reclaim threshold for Redis pending messages and the
maximum time the module waits for the FrankenPHP worker call that runs a job.
Set it above the normal maximum job runtime, including database latency,
external API latency, and deployment jitter.

If a job runs longer than `visibility_timeout`, the module can release or fail
the delivery while the PHP handler is still running. Another worker can then
receive the same logical job. Long jobs need either a larger
`visibility_timeout` or application-level idempotency that makes duplicate runs
safe.

Guidance:

- `visibility_timeout`: greater than the longest expected successful job.
- `shutdown_timeout`: long enough for normal in-flight jobs to finish during
  deploys, or short enough that operators accept Redis reclaiming them later.
- `reserve_timeout`: keep short enough that shutdowns are responsive.
- `max_attempts`: match the job's failure policy. The first delivery is attempt
  1; releases and Redis reclaims increase the attempt count. Deliveries move to
  the failed stream after the configured limit is exceeded.

## Duplicate Delivery Scenarios

Design handlers so these cases are safe:

- The PHP process crashes after a side effect but before acknowledgement.
- A job exceeds `visibility_timeout`.
- A deploy reaches `shutdown_timeout` while a job is still running.
- Redis failover loses a recent acknowledgement.
- A handler throws after partially completing external work.
- An operator manually replays a failed job.

Common idempotency patterns are database unique constraints, idempotency keys
sent to external APIs, outbox tables, compare-and-swap state transitions, and
short-lived application locks around non-repeatable work.

## Deploys

During a Caddy or FrankenPHP deploy, the old process stops reserving new work and
waits up to `shutdown_timeout` for in-flight deliveries. Deliveries that are not
acknowledged remain pending in Redis and can be reclaimed by another consumer
after `visibility_timeout`.

Keep these settings stable during rolling deploys:

- `key_prefix`
- queue names
- Redis consumer `group`

Changing any of them creates a separate queue namespace or consumer group. That
is a migration, not a routine deploy.

Use a unique Redis `consumer` value per running process or container. If omitted,
the module derives one from the host and process id.

## Metrics, Status, And Alerting

When Caddy metrics are enabled, the module registers Prometheus metrics in the
same registry as Caddy's built-in metrics:

- `caddy_pogo_queue_worker_ready`
- `caddy_pogo_queue_queue_ready{queue}`
- `caddy_pogo_queue_messages{queue,state}`
- `caddy_pogo_queue_events_total{event}`
- `caddy_pogo_queue_payload_limit_bytes{queue}`

The `state` label is one of `pending`, `delayed`, `reserved`, or `failed`. The
`event` label covers `enqueued`, `reserved`, `acked`, `released`, `failed`,
`dropped_full`, `dropped_payload_too_large`, `dropped_shutdown`, and
`backend_errors`.

You can also expose `pogo_queue_status()` from an authenticated health endpoint
or framework command that runs in the same FrankenPHP binary as the queue
module.

Important fields:

- `ready`: the worker pool has started and is not shutting down.
- `pending`: ready stream messages not currently owned by the consumer group.
- `delayed`: delayed messages waiting in the sorted set.
- `reserved`: pending messages owned by the consumer group.
- `failed`: messages in the failed stream.
- `backend_errors`: Redis or lifecycle errors observed by the module.
- `dropped_payload_too_large`: payloads rejected by `max_payload_bytes`.
- `dropped_shutdown`: dispatches rejected while the module was shutting down.

Alert on sustained growth in `failed`, `reserved`, `backend_errors`,
`dropped_payload_too_large`, or `dropped_shutdown`. Alert when `pending` or
`delayed` grows while worker throughput is flat.

Redis-side checks for the default key prefix and queue:

```bash
redis-cli XLEN pogo:default:stream
redis-cli XPENDING pogo:default:stream default
redis-cli ZCARD pogo:default:delayed
redis-cli XLEN pogo:default:failed
```

## Failed Jobs

Failed deliveries are written to `<prefix>:<queue>:failed` with the original
stream id, payload, failure reason, and failure timestamp. They are not removed
automatically.

Inspect recent failures:

```bash
redis-cli XREVRANGE pogo:default:failed + - COUNT 10
```

Prefer replaying work through the framework or application command that created
the job, because that keeps business validation and audit logging in one place.
For emergency manual replay, inspect the failed payload first, then re-enqueue
the payload and remove the failed record only after the replay is accepted:

```bash
redis-cli XADD pogo:default:stream '*' payload "$PAYLOAD" attempts 1
redis-cli XDEL pogo:default:failed "$FAILED_ID"
```

Use manual replay carefully. It bypasses framework-specific failed-job tooling
and can duplicate side effects if the original handler partially succeeded.

## Incident Checklist

1. Confirm the module is ready with `pogo_queue_status()`.
2. Check Redis reachability, memory, persistence, and replication health.
3. Compare `pending`, `reserved`, `delayed`, and `failed` counts.
4. Inspect application logs for handler exceptions and worker delivery errors.
5. If `reserved` grows, check job runtime against `visibility_timeout`.
6. If `failed` grows, inspect the failed stream and decide whether to replay,
   fix the handler, or purge only after external audit requirements are met.
