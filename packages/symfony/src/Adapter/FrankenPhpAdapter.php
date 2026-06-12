<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Adapter;

use Pogo\Queue\Symfony\Contract\PogoAdapter;
use Pogo\Queue\Symfony\Transport\QueueDispatchException;
use Pogo\Queue\Symfony\Transport\QueueFullException;
use Pogo\Queue\Symfony\Transport\QueuePayloadTooLargeException;
use Pogo\Queue\Symfony\Transport\QueueShuttingDownException;
use Pogo\Queue\Symfony\Transport\QueueWorkerUnavailableException;
use RuntimeException;

final class FrankenPhpAdapter implements PogoAdapter
{
    private const DISPATCH_RESULT_FULL = 0;
    private const DISPATCH_RESULT_WORKER_UNAVAILABLE = 2;
    private const DISPATCH_RESULT_PAYLOAD_TOO_LARGE = 3;
    private const DISPATCH_RESULT_SHUTTING_DOWN = 4;
    private const DISPATCH_RESULT_BACKEND_FAILURE = 5;

    public function push(string $queue, string $payload, int $delaySeconds = 0): string
    {
        if (!function_exists('pogo_queue_push')) {
            throw new RuntimeException("FrankenPHP 'pogo_queue_push' extension is not enabled.");
        }

        $result = $this->decodeResponse(\pogo_queue_push($queue, $payload, $delaySeconds), 'FrankenPHP queue dispatch returned an invalid response.');

        if (($result['ok'] ?? false) === true && is_string($result['id'] ?? null)) {
            return $result['id'];
        }

        $this->throwOnFailure((int) ($result['code'] ?? self::DISPATCH_RESULT_BACKEND_FAILURE), (string) ($result['message'] ?? 'Dispatch failed.'));
    }

    public function ack(string $queue, string $deliveryId): void
    {
        $this->assertLifecycleFunction('pogo_queue_ack');
        $this->throwUnlessAccepted((int) \pogo_queue_ack($queue, $deliveryId));
    }

    public function release(string $queue, string $deliveryId, int $delaySeconds = 0): void
    {
        $this->assertLifecycleFunction('pogo_queue_release');
        $this->throwUnlessAccepted((int) \pogo_queue_release($queue, $deliveryId, $delaySeconds));
    }

    public function fail(string $queue, string $deliveryId, string $reason = ''): void
    {
        $this->assertLifecycleFunction('pogo_queue_fail');
        $this->throwUnlessAccepted((int) \pogo_queue_fail($queue, $deliveryId, $reason));
    }

    public function failed(string $queue, int $limit = 100): array
    {
        $this->assertLifecycleFunction('pogo_queue_failed');

        $result = $this->decodeResponse(\pogo_queue_failed($queue, $limit), 'FrankenPHP queue failed listing returned an invalid response.');
        if (($result['ok'] ?? false) === true && is_array($result['failed'] ?? null)) {
            return array_values(array_filter($result['failed'], 'is_array'));
        }

        $this->throwOnFailure((int) ($result['code'] ?? self::DISPATCH_RESULT_BACKEND_FAILURE), (string) ($result['message'] ?? 'Failed job listing failed.'));
    }

    public function retryFailed(string $queue, string $failedId): string
    {
        $this->assertLifecycleFunction('pogo_queue_retry_failed');

        $result = $this->decodeResponse(\pogo_queue_retry_failed($queue, $failedId), 'FrankenPHP queue failed retry returned an invalid response.');
        if (($result['ok'] ?? false) === true && is_string($result['id'] ?? null)) {
            return $result['id'];
        }

        $this->throwOnFailure((int) ($result['code'] ?? self::DISPATCH_RESULT_BACKEND_FAILURE), (string) ($result['message'] ?? 'Failed job retry failed.'));
    }

    public function forgetFailed(string $queue, string $failedId): void
    {
        $this->assertLifecycleFunction('pogo_queue_forget_failed');

        $result = $this->decodeResponse(\pogo_queue_forget_failed($queue, $failedId), 'FrankenPHP queue failed forget returned an invalid response.');
        if (($result['ok'] ?? false) === true) {
            return;
        }

        $this->throwOnFailure((int) ($result['code'] ?? self::DISPATCH_RESULT_BACKEND_FAILURE), (string) ($result['message'] ?? 'Failed job forget failed.'));
    }

    public function purgeFailed(string $queue): int
    {
        $this->assertLifecycleFunction('pogo_queue_purge_failed');

        $result = $this->decodeResponse(\pogo_queue_purge_failed($queue), 'FrankenPHP queue failed purge returned an invalid response.');
        if (($result['ok'] ?? false) === true) {
            return (int) ($result['count'] ?? 0);
        }

        $this->throwOnFailure((int) ($result['code'] ?? self::DISPATCH_RESULT_BACKEND_FAILURE), (string) ($result['message'] ?? 'Failed job purge failed.'));
    }

    public function handle(callable $callback): bool
    {
        if (!function_exists('frankenphp_handle_request')) {
            throw new RuntimeException("FrankenPHP 'frankenphp_handle_request' is not available.");
        }

        return \frankenphp_handle_request($callback);
    }

    private function assertLifecycleFunction(string $function): void
    {
        if (!function_exists($function)) {
            throw new RuntimeException(sprintf("FrankenPHP '%s' extension function is not enabled.", $function));
        }
    }

    private function throwUnlessAccepted(int $status): void
    {
        if ($status === 1) {
            return;
        }

        $this->throwOnFailure($status, sprintf('FrankenPHP queue lifecycle operation failed with status code %d.', $status));
    }

    /**
     * @return array<string, mixed>
     */
    private function decodeResponse(string $response, string $message): array
    {
        $result = json_decode($response, true);
        if (!is_array($result)) {
            throw new QueueDispatchException($message);
        }

        return $result;
    }

    private function throwOnFailure(int $status, string $message): never
    {
        switch ($status) {
            case self::DISPATCH_RESULT_FULL:
                throw new QueueFullException($message);
            case self::DISPATCH_RESULT_WORKER_UNAVAILABLE:
                throw new QueueWorkerUnavailableException($message);
            case self::DISPATCH_RESULT_PAYLOAD_TOO_LARGE:
                throw new QueuePayloadTooLargeException($message);
            case self::DISPATCH_RESULT_SHUTTING_DOWN:
                throw new QueueShuttingDownException($message);
            default:
                throw new QueueDispatchException($message);
        }
    }
}
