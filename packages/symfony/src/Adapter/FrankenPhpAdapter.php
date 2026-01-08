<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Adapter;

use Pogo\Queue\Symfony\Contract\PogoAdapter;
use RuntimeException;

final class FrankenPhpAdapter implements PogoAdapter
{
    public function push(string $payload): bool
    {
        if (!function_exists('pogo_queue')) {
            throw new RuntimeException("FrankenPHP 'pogo_queue' extension is not enabled.");
        }

        return \pogo_queue($payload);
    }

    public function handle(callable $callback): bool
    {
        if (!function_exists('frankenphp_handle_request')) {
            throw new RuntimeException("FrankenPHP 'frankenphp_handle_request' is not available.");
        }

        return \frankenphp_handle_request($callback);
    }
}
