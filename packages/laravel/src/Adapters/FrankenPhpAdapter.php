<?php

namespace Pogo\Queue\Laravel\Adapters;

use Pogo\Queue\Laravel\Contracts\PogoAdapter;
use RuntimeException;

class FrankenPhpAdapter implements PogoAdapter
{
    public function push(string $payload): int
    {
        if (!function_exists('pogo_queue')) {
            throw new RuntimeException("FrankenPHP 'pogo_queue' extension is not enabled.");
        }

        return (int) \pogo_queue($payload);
    }
}
