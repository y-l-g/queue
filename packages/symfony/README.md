# Pogo Queue Bundle for Symfony

A [FrankenPHP](https://frankenphp.dev) Messenger transport for Symfony.

## Requirements

* PHP 8.4+
* Symfony 8.0+
* FrankenPHP with `pogo_queue` module enabled.

## Installation

```bash
composer require pogo/symfony-queue
```

## Configuration

1. Enable the bundle in `config/bundles.php` (if not using Flex):

```php
return [
    // ...
    Pogo\Queue\Symfony\PogoQueueBundle::class => ['all' => true],
];
```

2. Configure the transport in `config/packages/messenger.yaml`:

```yaml
framework:
    messenger:
        transports:
            pogo: 'pogo-queue://default'
            
        routing:
            'App\Message\YourMessage': pogo
```

## Usage

Start your Symfony application with FrankenPHP.

```bash
frankenphp run --config Caddyfile
```

> **Warning**
> This transport is volatile (in-memory). Messages are lost on server restart.