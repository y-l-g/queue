# Pogo Queue Bundle for Symfony

A [FrankenPHP](https://frankenphp.dev) Messenger transport for Symfony.

This bundle allows you to use the experimental `pogo_queue` module from as a native Symfony Messenger transport. It provides an ultra-fast, in-memory queue system.

## Requirements

* PHP 8.5+
* Symfony 8.0
* FrankenPHP binary compiled with the `pogo_queue` module enabled.

## Installation

```bash
composer require pogo/symfony-queue
```

## Configuration

### 2. Configure Messenger

Open `config/packages/messenger.yaml` and configure the transport.

```yaml
framework:
    messenger:
        transports:
            pogo: 'pogo-queue://default'
            
        routing:
            'App\Message\YourMessage': pogo
```

in your .env

```dotenv
```

## FrankenPHP Setup

To enable the worker, you need a specific `Caddyfile` and a worker entry point script (`queue-worker.php`) at the root of your project.

### 1. The Worker Script (`queue-worker.php`)

Create a file named `queue-worker.php` in public folder.

> **Note:** We use `ArrayInput` to explicitly tell the worker which transport to consume (`pogo` in this example).

```php
<?php

use App\Kernel;
use Symfony\Bundle\FrameworkBundle\Console\Application;
use Symfony\Component\Console\Input\ArrayInput;

if (!is_dir(__DIR__ . '../vendor')) {
    throw new LogicException('Dependencies are missing. Try running "composer install".');
}

if (!is_file(__DIR__ . '../vendor/autoload_runtime.php')) {
    throw new LogicException('Symfony Runtime is missing. Try running "composer require symfony/runtime".');
}

require_once __DIR__ . '../vendor/autoload_runtime.php';

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

### 2. The Caddyfile

Create a `Caddyfile` at the root of your project. This configuration enables the `pogo_queue` worker and serves the Symfony application.

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

## Usage

Start FrankenPHP using the configuration file:

```bash
frankenphp run --config Caddyfile
```

You should see logs indicating that the worker has started:
`[OK] Consuming messages from transport "pogo".`

### Troubleshooting & Known Limitations

1. **"No transport supports Messenger DSN..."**:
    * Ensure you have registered the `FrankenPHPQueueTransportFactory` in your `config/services.yaml`.
    * Ensure your DSN in `messenger.yaml` starts exactly with `pogo-queue://`.

2. **Volatile Memory**:
    * **Warning:** This transport is in-memory. If you restart FrankenPHP, all pending messages in the queue are **lost**. Do not use this for critical data that must persist across restarts.

3. **Logs**:
    * The worker output (logs) will appear in the main FrankenPHP console window.