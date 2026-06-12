# Pogo Queue Driver for Laravel

Laravel queue driver for the FrankenPHP Queue module.

The driver expects a FrankenPHP binary compiled with the `pogo_queue` module and
a production `backend redis` Caddy configuration. Jobs are delivered at least
once, so handlers must be idempotent.

## Installation

```bash
composer require pogo/laravel-queue
```

## Configuration

```php
'pogo' => [
    'driver' => 'pogo',
    'queue' => env('POGO_QUEUE', 'default'),
    'retry_after' => 90,
],
```

```dotenv
QUEUE_CONNECTION=pogo
POGO_QUEUE=default
POGO_REDIS_URL=redis://redis:6379/0
```

Delayed dispatch is supported through Laravel's normal `later()` /
`dispatch()->delay()` APIs.

For production operations, Redis persistence, duplicate-delivery scenarios,
`visibility_timeout` sizing, failed-job recovery, and alerting guidance, read
the [production runbook](https://github.com/y-l-g/queue/blob/main/docs/production-runbook.md).

## Worker

Run jobs through a FrankenPHP worker entrypoint and construct `PogoJob` from the
delivery envelope sent by the module:

```php
$delivery = json_decode($message, true, flags: JSON_THROW_ON_ERROR);
$job = PogoJob::fromDelivery($app, $app['queue']->connection('pogo'), $delivery);
$app['queue.worker']->process('pogo', $job, new WorkerOptions());
```

`PogoJob::delete()`, `release()`, and `fail()` acknowledge, retry, or dead-letter
the Redis Streams delivery.

The adapter also exposes failed-job recovery primitives:

```php
$failed = $queue->getAdapter()->failed('default');
$failedId = $failed[0]['id'];

// Choose one recovery action:
$newId = $queue->getAdapter()->retryFailed('default', $failedId);
$queue->getAdapter()->forgetFailed('default', $failedId);
$purged = $queue->getAdapter()->purgeFailed('default');
```
