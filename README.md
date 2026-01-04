# FrankenPHP Queue

A [FrankenPHP](https://frankenphp.dev) extension and Laravel driver that allows you to send messages in queues and handle them asynchronously.

It is designed as a lightweight, **in-memory** replacement for queue systems like RabbitMQ or Redis, ideal for high-performance setups where simplicity is key.

> [!WARNING]
>
> **VOLATILE DATA**: This is an in-memory queue.
>
> * If the server crashes or restarts, **all pending jobs are lost**.
> * Do not use this for critical financial transactions or data that cannot be regenerated.
>
> **NO DELAYS**: This driver does not support delayed jobs (e.g., `dispatch()->delay(...)`).
> Attempting to dispatch a delayed job will throw a `BadMethodCallException`.

## Installation

### 1. Build the Binary

Follow [the instructions to install a ZTS version of libphp and `xcaddy`](https://frankenphp.dev/docs/compile/#install-php).
Then, use [`xcaddy`](https://github.com/caddyserver/xcaddy) to build FrankenPHP with the `pogo-queue` module:

```console
CGO_ENABLED=1 \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/y-l-g/queue/module \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/caddy-cbrotli
```

Or simply use the binary or the docker image provided by this repo.

### 2. Install

```bash
composer require pogo/queue
php artisan pogo:queue:install
```

Add the `pogo_queue` block to your Global Options in the `Caddyfile`. This is an adapted copy of the official octane caddyfile.

```caddyfile
{
    {$CADDY_GLOBAL_OPTIONS}

    admin {$CADDY_SERVER_ADMIN_HOST}:{$CADDY_SERVER_ADMIN_PORT}

    frankenphp {
        worker {
            file "{$APP_PUBLIC_PATH}/frankenphp-worker.php"
            {$CADDY_SERVER_WORKER_DIRECTIVE}
            {$CADDY_SERVER_WATCH_DIRECTIVES}
        }
    }
    pogo_queue {
        worker {$APP_PUBLIC_PATH}/queue-worker.php
        name m#Queue
        size 10000       # Max jobs in memory. If full, dispatch throws QueueFullException.
        num_threads 32   # Number of concurrent workers (defaults to CPU count).
    }
}

{$CADDY_EXTRA_CONFIG}

{$CADDY_SERVER_SERVER_NAME} {
    log {
        level {$CADDY_SERVER_LOG_LEVEL}

    # Redact the authorization query parameter that can be set by Mercure...
        format filter {
            wrap {$CADDY_SERVER_LOGGER}
            fields {
                uri query {
                    replace authorization REDACTED
                }
            }
        }
    }
    route {
        root * "{$APP_PUBLIC_PATH}"
        encode zstd br gzip

        # Mercure configuration is injected here...
        {$CADDY_SERVER_EXTRA_DIRECTIVES}

        php_server {
            index frankenphp-worker.php
            try_files {path} frankenphp-worker.php
            # Required for the public/storage/ directory...
            resolve_root_symlink
        }
    }
}
```

Add the following configuration to `config/queue.php` in the `connections` array:

```php
'pogo' => [
    'driver' => 'pogo',
    'queue' => env('POGO_QUEUE', 'default'),
    'retry_after' => 90,
],
```

You can configure the connection and queue name using environment variables.

* `QUEUE_CONNECTION=pogo`
* `POGO_QUEUE=default` (Optional, defaults to 'default')

Run octane with the adapted caddyfile

```bash
php artisan octane:frankenphp --caddyfile=Caddyfile
```

## Handling Backpressure

Since the queue has a fixed size (defined in `Caddyfile`), it can fill up if workers are slower than producers.

**Unlike Redis, this driver throws an exception immediately when full.**

```php
use Pogo\Queue\Exceptions\QueueFullException;
use App\Jobs\ProcessData;

try {
    ProcessData::dispatch($data);
} catch (QueueFullException $e) {
    // The buffer is full.
    // 1. Return a 503 Service Unavailable
    abort(503, 'Server is busy, please try again later.');
    
    // 2. Or fallback to a database driver
    // ProcessData::dispatch($data)->onConnection('database');
}
```

## Limitations

1. **No Persistence**: Data is in RAM. Restart = Data Loss.
2. **No Delays**: `later()` and `delay()` are not supported and will throw an exception.
3. **No Size Inspection**: `Queue::size()` currently returns `0` as the extension does not expose metrics yet.