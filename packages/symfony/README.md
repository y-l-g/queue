# Pogo Queue Bundle for Symfony

Symfony Messenger transport for the FrankenPHP Queue module.

The transport expects a FrankenPHP binary compiled with `pogo_queue` and a
production `backend redis` Caddy configuration. Messages are delivered at least
once, so handlers must be idempotent.

## Installation

```bash
composer require pogo/symfony-queue
```

The bundle registers the Messenger transport factory automatically.

## Configuration

```yaml
framework:
  messenger:
    transports:
      pogo: "pogo-queue://default"
    routing:
      'App\Message\YourMessage': pogo
```

Delayed messages are supported through Symfony's normal `DelayStamp`.

## Worker

Run Messenger from a FrankenPHP worker entrypoint:

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

The transport acknowledges handled messages, fails rejected messages, and records
delivery metadata in `PogoReceivedStamp`.
