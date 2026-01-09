# FrankenPHP Queue

A [FrankenPHP](https://frankenphp.dev) extension that enables asynchronous message handling via high-performance, in-memory queues.

Designed as a lightweight replacement for systems like RabbitMQ or Redis, it is ideal for high-performance setups where simplicity is key and persistence is not required.

This repository contains the Go source code for the module, as well as the PHP drivers for **Laravel** (`pogo/queue`) and **Symfony** (`pogo/symfony-queue`).

## Sommaire

- [Installation (Global)](#installation-global)
  - [Get the Binary](#get-the-binary)
  - [Pre-built Binary or Docker (Recommended)](#pre-built-binary-or-docker-recommended)
  - [Compile from Source](#compile-from-source)
- [Usage with Laravel](#usage-with-laravel)
  - [Install Dependencies](#install-dependencies)
  - [Configure Environment](#configure-environment)
  - [Configure Laravel](#configure-laravel)
  - [Create Worker Script](#create-worker-script)
  - [Configure Caddyfile](#configure-caddyfile)
  - [Run Octane](#run-octane)
  - [Handling Backpressure (Laravel)](#handling-backpressure-laravel)
- [Usage with Symfony](#usage-with-symfony)
  - [Requirements](#requirements)
  - [Install Bundle](#install-bundle)
  - [Register the Transport Factory (Crucial)](#register-the-transport-factory-crucial)
  - [Configure Messenger](#configure-messenger)
  - [Create Worker Script (Symfony)](#create-worker-script-symfony)
  - [Configure Caddyfile (Symfony)](#configure-caddyfile-symfony)
  - [Run FrankenPHP](#run-frankenphp)
- [General Limitations](#general-limitations)

> [!WARNING]
>
> **VOLATILE DATA**: This is an in-memory queue.
>
> - If the server crashes or restarts, **all pending jobs are lost**.
> - **Do not use this** for critical financial transactions or data that cannot be regenerated.
>
> **NO DELAYS**: This driver does not support delayed jobs (e.g., `dispatch()->delay(...)`).
> Attempting to dispatch a delayed job will throw a `BadMethodCallException` (Laravel) or result in immediate execution.

## Installation (Global)

### Get the Binary

Regardless of the framework you use, you need a FrankenPHP binary with the `pogo_queue` module enabled.

#### Pre-built Binary or Docker (Recommended)

You can use the pre-compiled binaries or Docker images that already include the queue module.

- **Binaries:** Download from [FrankenPHP with websocket, queue, and scheduler releases](https://github.com/y-l-g/websocket/releases).
- **Docker:** Use the [docker image](https://github.com/y-l-g?tab=packages&repo_name=websocket).

#### Compile from Source

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

---

## Usage with Laravel

### Install Dependencies

Install the queue driver package:

```bash
composer require pogo/queue
```

Ensure Laravel Octane is installed and configured for FrankenPHP:

```bash
php artisan octane:install --server=frankenphp
```

### Configure Environment

Add the following variables to your `.env` file:

```dotenv
QUEUE_CONNECTION=pogo
POGO_QUEUE=default
```

*(Note: `POGO_QUEUE` is optional and defaults to 'default')*

### Configure Laravel

Add the `pogo` connection to the `connections` array in `config/queue.php`:

```php
'pogo' => [
    'driver' => 'pogo',
    'queue' => env('POGO_QUEUE', 'default'),
    'retry_after' => 90,
],
```

### Create Worker Script

Create a new file at `public/queue-worker.php`.
This script is a dedicated entry point for the queue worker process.

```php
<?php

use Laravel\Octane\ApplicationFactory;
use Laravel\Octane\FrankenPhp\FrankenPhpClient;
use Laravel\Octane\Worker;
use Illuminate\Queue\WorkerOptions;
use Pogo\Queue\Laravel\PogoJob;

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

### Configure Caddyfile

Update your `Caddyfile` (usually at the project root) to include the `pogo_queue` block.
Below is a complete example based on the official Octane configuration.

```caddy
{
    frankenphp {
        worker {
            file "public/frankenphp-worker.php"
        }
    }
    
    # Queue Configuration
    pogo_queue {
        worker public/queue-worker.php
        name myQueue
        size 10000       # Max jobs in memory. If full, dispatch throws QueueFullException.
        num_threads 32   # Number of concurrent workers (defaults to CPU count).
    }
}

:8080 {
    log {
        level INFO

        # Redact the authorization query parameter that can be set by Mercure...
        format filter {
            wrap json
            fields {
                uri query {
                    replace authorization REDACTED
                }
            }
        }
    }
    
    route {
        root * public
        encode zstd br gzip

        php_server {
            index frankenphp-worker.php
            try_files {path} frankenphp-worker.php
            resolve_root_symlink
        }
    }
}
```

### Run Octane

Start the server using the configured Caddyfile:

```bash
php artisan octane:frankenphp --caddyfile=Caddyfile 
#or
./frankenphp run --config Caddyfile
```

### Handling Backpressure (Laravel)

Since the queue has a fixed size (defined in `Caddyfile` via the `size` directive), it can fill up if workers are slower than producers. **Unlike Redis, this driver throws an exception immediately when the queue is full.**

You should handle this exception in your code:

```php
use Pogo\Queue\Laravel\Exceptions\QueueFullException;
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

---

## Usage with Symfony

### Requirements

- PHP 8.5+
- Symfony 8.0+
- FrankenPHP binary compiled with the `pogo_queue` module enabled.

### Install Bundle

```bash
composer require pogo/symfony-queue
```

### Register the Transport Factory (Crucial)

Open `config/services.yaml` and add the factory to the `services` section:

```yaml
services:
    # ... other services ...

    # Register the Pogo Transport Factory manually
    Pogo\Queue\Symfony\Transport\PogoQueueTransportFactory:
        tags: ['messenger.transport_factory']
```

### Configure Messenger

Open `config/packages/messenger.yaml` and configure the transport.

```yaml
framework:
    messenger:
        transports:
            # The DSN must start with pogo-queue://
            pogo: 'pogo-queue://default'
            
        routing:
            'App\Message\YourMessage': pogo
```

### Create Worker Script (Symfony)

Create a file named `queue-worker.php` in your **`public/`** folder.

```php
<?php

use App\Kernel;
use Symfony\Bundle\FrameworkBundle\Console\Application;
use Symfony\Component\Console\Input\ArrayInput;

// Pointing to vendor one level up from public/
if (!is_dir(__DIR__ . '/../vendor')) {
    throw new LogicException('Dependencies are missing. Try running "composer install".');
}

if (!is_file(__DIR__ . '/../vendor/autoload_runtime.php')) {
    throw new LogicException('Symfony Runtime is missing. Try running "composer require symfony/runtime".');
}

require_once __DIR__ . '/../vendor/autoload_runtime.php';

return function (array $context) {
    $kernel = new Kernel($context['APP_ENV'], (bool) $context['APP_DEBUG']);

    $app = new Application($kernel);

    // Set the default command to consume messages
    $app->setDefaultCommand('messenger:consume', true);

    $input = new ArrayInput([
        'receivers' => ['pogo'],
        '--limit' => 1000,
        '--time-limit' => 3600
    ]);

    $app->run($input);

    return $app;
};
```

### Configure Caddyfile (Symfony)

Create (or update) a `Caddyfile` at the root of your project. This configuration enables the `pogo_queue` worker and serves the Symfony application.

```caddy
{
    frankenphp
    # Configure the queue worker module
    pogo_queue {
        worker public/queue-worker.php
    }
}

:8000 {
    root public

    @phpRoute {
        not file {path}
    }
    rewrite @phpRoute index.php

    @frontController path index.php
    php @frontController

    file_server {
        hide *.php
    }
}
```

### Run FrankenPHP

Start FrankenPHP using the configuration file:

```bash
frankenphp run --config Caddyfile
```

You should see logs indicating that the worker has started:
`[OK] Consuming messages from transport "pogo".`

---

## General Limitations

1. **No Persistence**: Data is stored in RAM. **Restart = Data Loss.**
2. **No Delays**: `later()` and `delay()` are not supported and will throw a `BadMethodCallException`.
3. **No Size Inspection (Laravel)**: `Queue::size()` currently returns `0` because the extension does not expose internal metrics yet.