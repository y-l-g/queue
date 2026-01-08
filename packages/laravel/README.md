# Pogo Queue Driver for Laravel

A [FrankenPHP](https://frankenphp.dev) queue driver for Laravel. It utilizes the `pogo_queue` C-extension to handle jobs in-memory with high performance.

## Requirements

* PHP 8.4+
* Laravel 12+
* FrankenPHP with `pogo_queue` module enabled.

## Installation

```bash
composer require pogo/laravel-queue
```

## Configuration

1. Add the connection to `config/queue.php`:

```php
'pogo' => [
    'driver' => 'pogo',
    'queue' => env('POGO_QUEUE', 'default'),
],
```

2. Update your `.env` file:

```dotenv
QUEUE_CONNECTION=pogo
```

## Usage

Run your application using FrankenPHP. The queue works in-memory.

```bash
php artisan octane:start --server=frankenphp
```

> **Warning**
> This is an in-memory queue. If the server restarts, all pending jobs are lost. Do not use for critical persistent data.