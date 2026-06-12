# Changelog

## Unreleased

- Reworked the module around a production Redis Streams backend.
- Added explicit local-only memory backend with bounded message and byte limits.
- Added real queue names, delayed jobs, reserve/ack/release/fail lifecycle
  functions, pending-message reclaim, failed streams, and bounded shutdown.
- Updated Laravel and Symfony drivers to use the v2 delivery envelope and
  lifecycle APIs.
- Added backend tests, Redis-gated integration coverage, Composer validation,
  PHPStan stub scanning, and PR CI coverage.
- Added a production runbook covering Redis durability, at-least-once delivery,
  timeout sizing, deploy behavior, status checks, and failed-job recovery.
- Added Caddy Prometheus metrics for worker readiness, queue message states,
  lifecycle events, backend errors, dropped payloads, and payload limits.
- Added failed-job operations to list, retry, forget, and purge failed
  deliveries through the extension and framework adapters.
- Fixed Symfony Messenger retry handling so retryable failures acknowledge the
  original delivery after the retry copy is sent instead of dead-lettering it.
- Added Docker packaging guidance, a compatibility matrix, and release checksum
  helper.
- Removed the deprecated v1 `pogo_queue()` helper.
