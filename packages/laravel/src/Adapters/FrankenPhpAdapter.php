<?php

namespace Pogo\Queue\Laravel\Adapters;

use Pogo\Queue\Laravel\Contracts\PogoAdapter;
use Pogo\Queue\Laravel\Exceptions\QueueDispatchException;
use Pogo\Queue\Laravel\Exceptions\QueueFullException;
use Pogo\Queue\Laravel\Exceptions\QueuePayloadTooLargeException;
use Pogo\Queue\Laravel\Exceptions\QueueSendException;
use Pogo\Queue\Laravel\Exceptions\QueueShuttingDownException;
use Pogo\Queue\Laravel\Exceptions\QueueWorkerUnavailableException;
use RuntimeException;

class FrankenPhpAdapter implements PogoAdapter
{
    private const DISPATCH_RESULT_FULL = 0;
    private const DISPATCH_RESULT_WORKER_UNAVAILABLE = 2;
    private const DISPATCH_RESULT_PAYLOAD_TOO_LARGE = 3;
    private const DISPATCH_RESULT_SHUTTING_DOWN = 4;
    private const DISPATCH_RESULT_BACKEND_FAILURE = 5;
    private const DISPATCH_RESULT_QUEUE_UNKNOWN = 6;

    public function push(string $queue, string $payload, int $delaySeconds = 0): string
    {
        if (!function_exists('pogo_queue_push')) {
            throw new RuntimeException("FrankenPHP 'pogo_queue_push' extension is not enabled.");
        }

        $result = json_decode(\pogo_queue_push($queue, $payload, $delaySeconds), true);
        if (!is_array($result)) {
            throw new QueueSendException('FrankenPHP queue dispatch returned an invalid response.');
        }

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

    public function status(?string $queue = null): array
    {
        if (!function_exists('pogo_queue_status')) {
            return [];
        }

        $decoded = json_decode(\pogo_queue_status($queue), true);
        return is_array($decoded) ? $decoded : [];
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
            case self::DISPATCH_RESULT_QUEUE_UNKNOWN:
            case self::DISPATCH_RESULT_BACKEND_FAILURE:
                throw new QueueDispatchException($message);
            default:
                throw new QueueSendException($message);
        }
    }
}
