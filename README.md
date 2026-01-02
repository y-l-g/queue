# FrankenPHP Queue

A [FrankenPHP](https://frankenphp.dev) extension that allows you to send messages in queues and handle them asynchronously.

It is designed as a lightweight, in-process replacement for queue systems like RabbitMQ or Redis, ideal for high-performance setups where simplicity is key.

> [!WARNING]
> This extension is an in-memory queue. Data is **volatile**: if the server crashes or restarts, pending jobs are lost.

## Installation

Follow [the instructions to install a ZTS version of libphp and `xcaddy`](https://frankenphp.dev/docs/compile/#install-php).
Then, use [`xcaddy`](https://github.com/caddyserver/xcaddy) to build FrankenPHP with the `pogo-queue` module:

```console
CGO_ENABLED=1 \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/y-l-g/queue=. \
    --with github.com/dunglas/frankenphp/caddy \
    --with github.com/dunglas/caddy-cbrotli
```

## Usage

### Register The Queue

Register the queue in your `Caddyfile`. You can control the buffer size (`size`) to handle backpressure.

```caddyfile
{
    frankenphp
    pogo_queue {
        worker queue-worker.php
        name m#Queue
        size 10000       # Size of the in-memory buffer. If full, pogo_queue returns false.
        num_threads 32   # Number of concurrent workers (defaults to CPU count)
    }
}

localhost {
    root public/
    php_server
}
```

### Write The Worker Script

Your worker script receives the message as the first argument of the handler.

```php
<?php
// queue-worker.php

$handler = static function ($message) {
    if ($message === null) {
        return;
    }
    
    // Process your message...
    error_log("Processing: " . $message);
};

$maxRequests = (int)($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    gc_collect_cycles();
    if (!$keepRunning) break;
}
```

### Dispatch Messages

The `pogo_queue` function returns a `bool` indicating if the message was successfully buffered.

```php
<?php
// public/index.php

$success = pogo_queue('Hello, Kévin!');

if ($success) {
    echo 'Data dispatched to an async worker.';
} else {
    // The queue is full or the worker is not running.
    http_response_code(503);
    echo 'Queue is full.';
}