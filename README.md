# FrankenPHP Queue

A [FrankenPHP](https://frankenphp.dev) extension and Laravel driver that allows you to send messages in queues and handle them asynchronously.

It is designed as a lightweight, **in-memory** replacement for queue systems like RabbitMQ or Redis, ideal for high-performance setups where simplicity is key.

[!WARNING]
**VOLATILE DATA**: This is an in-memory queue.

> * If the server crashes or restarts, **all pending jobs are lost**.
> * Do not use this for critical financial transactions or data that cannot be regenerated.

[!WARNING]
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
    --with github.com/y-l-g/queue \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/caddy-cbrotli
```

### 2. Install the Laravel Package

```bash
composer require pogo/queue
```

### 3. Install Configuration

```bash
php artisan pogo:queue:install
```

This command will:

1. Publish `public/queue-worker.php` (The entry point for the worker).
2. Create a `Caddyfile` example.
3. Update your `.env` to set `QUEUE_CONNECTION=pogo`.

**Manual Step**: You must add the following configuration to `config/queue.php` in the `connections` array:

```php
'pogo' => [
    'driver' => 'pogo',
    'queue' => env('POGO_QUEUE', 'default'),
    'retry_after' => 90,
],
```

## Configuration

### Caddyfile (Server Side)

Configure the memory buffer and worker threads in your `Caddyfile`.

```caddyfile
{
    frankenphp
    pogo_queue {
        worker queue-worker.php
        name m#Queue
        size 10000       # Max jobs in memory. If full, dispatch throws QueueFullException.
        num_threads 32   # Number of concurrent workers (defaults to CPU count).
    }
}
```

### Laravel (Application Side)

You can configure the connection and queue name using environment variables.

* `QUEUE_CONNECTION=pogo`
* `POGO_QUEUE=default` (Optional, defaults to 'default')

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