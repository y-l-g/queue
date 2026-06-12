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
- Deprecated the v1 `pogo_queue()` helper in favor of `pogo_queue_push()`.
