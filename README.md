# FrankenPHP Queue

A [FrankenPHP](https://frankenphp.dev) extension and Laravel driver that enables asynchronous message handling via queues.

Designed as a lightweight, **in-memory** replacement for systems like RabbitMQ or Redis, it is ideal for high-performance setups where simplicity is key and persistence is not required.

> [!WARNING]
>
> **VOLATILE DATA**: This is an in-memory queue.
>
> * If the server crashes or restarts, **all pending jobs are lost**.
> * **Do not use this** for critical financial transactions or data that cannot be regenerated.
>
> **NO DELAYS**: This driver does not support delayed jobs (e.g., `dispatch()->delay(...)`).
> Attempting to dispatch a delayed job will throw a `BadMethodCallException`.

## Installation

### 1. Get the Binary

You have two options to get FrankenPHP with the queue module enabled:

#### Option A: Pre-built Binary or Docker (Recommended)

You can use the pre-compiled binaries or Docker images that already include the queue module.

* **Binaries:** Download from [FrankenPHP with websocket, queue, and scheduler releases](https://github.com/y-l-g/websocket/releases).
* **Docker:** Use the [docker image](https://github.com/y-l-g?tab=packages&repo_name=websocket).

#### Option B: Compile from Source

If you prefer to build it yourself, follow [the instructions to install a ZTS version of libphp and `xcaddy`](https://frankenphp.dev/docs/compile/#install-php). Then, use `xcaddy` to build FrankenPHP with the `pogo-queue` module:

```bash
CGO_ENABLED=1 \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/y-l-g/queue/module \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/caddy-cbrotli
```

### 2. Install Laravel Dependencies

Ensure Laravel Octane is installed and configured for FrankenPHP:

```bash
php artisan octane:install --server=frankenphp
```

Install the queue driver package:

```bash
composer require pogo/queue
```

### 3. Configure Environment

Add the following variables to your `.env` file:

```dotenv
QUEUE_CONNECTION=pogo
POGO_QUEUE=default
```
*(Note: `POGO_QUEUE` is optional and defaults to 'default')*

### 4. Configure Laravel

Add the `pogo` connection to the `connections` array in `config/queue.php`:

```php
'pogo' => [
    'driver' => 'pogo',
    'queue' => env('POGO_QUEUE', 'default'),
    'retry_after' => 90,
],
```

### 5. Create Worker Script

Create a new file at `public/queue-worker.php`.
This script is a dedicated entry point for the queue worker process.

```php
<?php

use Laravel\Octane\ApplicationFactory;
use Laravel\Octane\FrankenPhp\FrankenPhpClient;
use Laravel\Octane\Worker;
use Illuminate\Queue\WorkerOptions;
use Pogo\Queue\PogoJob;

if ((!($_SERVER['FRANKENPHP_WORKER'] ?? false)) || !function_exists('frankenphp_handle_request')) {
    echo 'FrankenPHP must be in worker mode to use this script.';
    exit(1);
}

ignore_user_abort(true);

$basePath = $_SERVER['APP_BASE_PATH'] ?? $_ENV['APP_BASE_PATH'] ?? dirname(__DIR__, 4);

if (!file_exists($basePath . '/bootstrap/app.php')) {
    fwrite(STDERR, "Application path not found at: $basePath\n");
    exit(1);
}

require_once $basePath . '/vendor/autoload.php';

$frankenPhpClient = new FrankenPhpClient();

$worker = tap(new Worker(
    new ApplicationFactory($basePath),
    $frankenPhpClient
))->boot();

$requestCount = 0;
$maxRequests = $_ENV['MAX_REQUESTS'] ?? $_SERVER['MAX_REQUESTS'] ?? 1000;

// Allow configuration via environment variables
$queueConnection = $_ENV['POGO_CONNECTION'] ?? 'pogo';
$queueName = $_ENV['POGO_QUEUE'] ?? 'default';

$queueOptions = new WorkerOptions();

try {
    $handleRequest = static function ($payload) use ($worker, $queueOptions, $queueConnection, $queueName) {
        try {
            $app = $worker->application();

            // Resolve the specifically configured connection
            $connection = $app['queue']->connection($queueConnection);

            $job = new PogoJob(
                $app,
                $connection,
                $payload,
                $queueName
            );

            $app['queue.worker']->process($queueConnection, $job, $queueOptions);

        } catch (Throwable $e) {
            error_log("Worker Critical Error: " . $e->getMessage());
            if ($worker) {
                try {
                    report($e);
                } catch (Throwable $ex) {
                    // Silent fail to prevent crash loop
                }
            }
        }
    };

    while ($requestCount < $maxRequests && frankenphp_handle_request($handleRequest)) {
        $requestCount++;
    }
} finally {
    $worker?->terminate();
    gc_collect_cycles();
}
```

### 6. Configure Caddyfile

Update your `Caddyfile` (usually at the project root) to include the `pogo_queue` block.
Below is a complete example based on the official Octane configuration.

```caddy
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
    
    # Queue Configuration
    pogo_queue {
        worker {$APP_PUBLIC_PATH}/queue-worker.php
        name myQueue
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

### 7. Run Octane

Start the server using the configured Caddyfile:

```bash
php artisan octane:frankenphp --caddyfile=Caddyfile
```

---

## Handling Backpressure

Since the queue has a fixed size (defined in `Caddyfile` via the `size` directive), it can fill up if workers are slower than producers.

**Unlike Redis, this driver throws an exception immediately when the queue is full.**

You should handle this exception in your code:

```php
use Pogo\Queue\Exceptions\QueueFullException;
use App\Jobs\ProcessData;

try {
    ProcessData::dispatch($data);
} catch (QueueFullException $e) {
    // The buffer is full.
    
    // Option 1: Return a 503 Service Unavailable to the client
    abort(503, 'Server is busy, please try again later.');
    
    // Option 2: Fallback to a persistent driver (e.g., database)
    // ProcessData::dispatch($data)->onConnection('database');
}
```

## Limitations

1. **No Persistence**: Data is stored in RAM. **Restart = Data Loss.**
2. **No Delays**: `later()` and `delay()` are not supported and will throw a `BadMethodCallException`.
3. **No Size Inspection**: `Queue::size()` currently returns `0` because the extension does not expose internal metrics yet.