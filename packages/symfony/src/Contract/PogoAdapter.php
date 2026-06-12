<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Contract;

interface PogoAdapter
{
    public function push(string $queue, string $payload, int $delaySeconds = 0): string;

    public function ack(string $queue, string $deliveryId): void;

    public function release(string $queue, string $deliveryId, int $delaySeconds = 0): void;

    public function fail(string $queue, string $deliveryId, string $reason = ''): void;

    /**
     * @return list<array<string, mixed>>
     */
    public function failed(string $queue, int $limit = 100): array;

    public function retryFailed(string $queue, string $failedId): string;

    public function forgetFailed(string $queue, string $failedId): void;

    public function purgeFailed(string $queue): int;

    public function handle(callable $callback): bool;
}
