# FrankenPHP Queue

FrankenPHP Queue is a Redis Streams backed queue module for FrankenPHP with
Laravel and Symfony Messenger drivers.

Version 2 is designed for production use with at-least-once delivery: jobs are
reserved, acknowledged, retried, delayed, and moved to a failed stream when they
exceed the configured attempt limit. Job handlers must be idempotent.

## Production Model

- Production backend: Redis Streams.
- Delivery guarantee: at least once.
- Supported semantics: queue names, delayed jobs, reserve/ack/release/fail,
  bounded shutdown, pending-message reclaim, failed jobs, and status metrics.
- Local/demo backend: explicit `memory` backend only. It is not durable and must
  not be used for critical production work.

This project is not a dashboard or Horizon replacement. It is a small
FrankenPHP-native transport layer for applications that already operate Redis.

## Build

Build a FrankenPHP binary that includes this module:

```dockerfile
FROM dunglas/frankenphp:builder AS builder

COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy
COPY . /src/queue

RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx,nowatcher" \
    CGO_CFLAGS="-D_GNU_SOURCE $(php-config --includes)" \
    CGO_CPPFLAGS="$(php-config --includes)" \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/dunglas/frankenphp@v1.12.3 \
        --with github.com/dunglas/frankenphp/caddy@v1.12.3 \
        --with github.com/dunglas/caddy-cbrotli@v1.0.1 \
        --with github.com/y-l-g/queue/module=./src/queue/module

FROM dunglas/frankenphp AS runner
COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
```

## Caddy Configuration

Production Redis backend:

```caddy
{
    frankenphp

    pogo_queue {
        backend redis {
            url {$POGO_REDIS_URL}
            key_prefix pogo
            group default
            consumer {$HOSTNAME}
            tls false
        }

        worker public/queue-worker.php
        queues default,mail,notifications
        concurrency 8
        worker_threads 8
        max_payload_bytes 1048576
        visibility_timeout 90s
        shutdown_timeout 30s
        max_attempts 3
    }
}
```

Local-only memory backend:

```caddy
{
    frankenphp

    pogo_queue {
        backend memory {
            max_messages 1000
            max_total_bytes 67108864
        }

        worker public/queue-worker.php
        queues default
        concurrency 2
    }
}
```

## Laravel

Install the package:

```bash
composer require pogo/laravel-queue
```

Configure `config/queue.php`:

```php
'pogo' => [
    'driver' => 'pogo',
    'queue' => env('POGO_QUEUE', 'default'),
    'retry_after' => 90,
],
```

Set:

```dotenv
QUEUE_CONNECTION=pogo
POGO_QUEUE=default
POGO_REDIS_URL=redis://redis:6379/0
```

Create `public/queue-worker.php`:

```php
<?php

use Illuminate\Queue\WorkerOptions;
use Laravel\Octane\ApplicationFactory;
use Laravel\Octane\FrankenPhp\FrankenPhpClient;
use Laravel\Octane\Worker;
use Pogo\Queue\Laravel\PogoJob;

require_once dirname(__DIR__) . '/vendor/autoload.php';

$basePath = $_SERVER['APP_BASE_PATH'] ?? dirname(__DIR__);
$worker = tap(new Worker(new ApplicationFactory($basePath), new FrankenPhpClient()))->boot();
$options = new WorkerOptions();
$connection = $_ENV['POGO_CONNECTION'] ?? 'pogo';

try {
    while (frankenphp_handle_request(static function (string $message) use ($worker, $options, $connection): void {
        $delivery = json_decode($message, true, flags: JSON_THROW_ON_ERROR);
        $app = $worker->application();
        $queue = $app['queue']->connection($connection);
        $job = PogoJob::fromDelivery($app, $queue, $delivery);

        $app['queue.worker']->process($connection, $job, $options);
    })) {
    }
} finally {
    $worker->terminate();
}
```

## Symfony

Install the package:

```bash
composer require pogo/symfony-queue
```

Configure Messenger:

```yaml
framework:
  messenger:
    transports:
      pogo: 'pogo-queue://default'
    routing:
      'App\Message\YourMessage': pogo
```

Create `public/queue-worker.php`:

```php
<?php

use App\Kernel;
use Symfony\Bundle\FrameworkBundle\Console\Application;
use Symfony\Component\Console\Input\ArrayInput;

require_once __DIR__ . '/../vendor/autoload_runtime.php';

return static function (array $context) {
    $kernel = new Kernel($context['APP_ENV'], (bool) $context['APP_DEBUG']);
    $app = new Application($kernel);
    $app->setDefaultCommand('messenger:consume', true);
    $app->run(new ArrayInput([
        'receivers' => ['pogo'],
        '--time-limit' => 3600,
    ]));

    return $app;
};
```

The Symfony bundle registers the transport factory automatically.

## Operations

Read [the production runbook](docs/production-runbook.md) before running this in
production. The key points are:

- Redis is the only production backend. Enable persistence and use a
  non-evicting memory policy for queue data.
- Delivery is at least once. Job handlers must be idempotent.
- Configure `visibility_timeout` above the normal maximum job runtime.
- Configure `max_attempts` according to job idempotency and failure policy.
- Monitor pending, reserved, delayed, failed, backend error, and payload rejection
  counts via `pogo_queue_status()` or the Caddy Prometheus metrics registered by
  the module.
- Use the failed-job operations to inspect, retry, forget, or purge failed
  deliveries without editing Redis keys by hand.
- During deploys, FrankenPHP stops reserving new work and waits up to
  `shutdown_timeout`; unacknowledged Redis messages remain pending and can be
  reclaimed after `visibility_timeout`.

## Extension API

The framework drivers use these functions. `pogo_queue_push()` returns a JSON
status string containing `ok`, `id`, `code`, and `message` fields.

- `pogo_queue_push(string $queue, string $payload, int $delaySeconds = 0): string`
- `pogo_queue_ack(string $queue, string $deliveryId): int`
- `pogo_queue_release(string $queue, string $deliveryId, int $delaySeconds = 0): int`
- `pogo_queue_fail(string $queue, string $deliveryId, string $reason = ''): int`
- `pogo_queue_status(?string $queue = null): string`
- `pogo_queue_failed(string $queue, int $limit = 100): string`
- `pogo_queue_retry_failed(string $queue, string $failedId): string`
- `pogo_queue_forget_failed(string $queue, string $failedId): string`
- `pogo_queue_purge_failed(string $queue): string`

`pogo_queue()` remains as a deprecated v1 compatibility helper for immediate
dispatch to the `default` queue.
