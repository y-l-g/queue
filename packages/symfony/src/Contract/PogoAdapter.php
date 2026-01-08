<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Contract;

interface PogoAdapter
{
    public function push(string $payload): bool;

    public function handle(callable $callback): bool;
}
