<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Contract;

interface PogoAdapter
{
    public function push(string $queue, string $payload, int $delaySeconds = 0): string;

    public function ack(string $queue, string $deliveryId): void;

    public function release(string $queue, string $deliveryId, int $delaySeconds = 0): void;

    public function fail(string $queue, string $deliveryId, string $reason = ''): void;

    public function handle(callable $callback): bool;
}
