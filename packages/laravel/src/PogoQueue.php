<?php

namespace Pogo\Queue\Laravel;

use BadMethodCallException;
use Illuminate\Contracts\Queue\Queue as QueueContract;
use Illuminate\Queue\Queue;
use Pogo\Queue\Laravel\Contracts\PogoAdapter;
use Pogo\Queue\Laravel\Exceptions\QueueFullException;
use Pogo\Queue\Laravel\Exceptions\QueuePayloadTooLargeException;
use Pogo\Queue\Laravel\Exceptions\QueueSendException;
use Pogo\Queue\Laravel\Exceptions\QueueShuttingDownException;
use Pogo\Queue\Laravel\Exceptions\QueueWorkerUnavailableException;

class PogoQueue extends Queue implements QueueContract
{
    private const DISPATCH_RESULT_ACCEPTED = 1;
    private const DISPATCH_RESULT_FULL = 0;
    private const DISPATCH_RESULT_WORKER_UNAVAILABLE = 2;
    private const DISPATCH_RESULT_PAYLOAD_TOO_LARGE = 3;
    private const DISPATCH_RESULT_SHUTTING_DOWN = 4;

    protected PogoAdapter $adapter;

    public function __construct(PogoAdapter $adapter)
    {
        $this->adapter = $adapter;
    }

    public function size($queue = null)
    {
        return (int) ($this->queueStats()['current_depth'] ?? 0);
    }

    public function pendingSize($queue = null)
    {
        return $this->size($queue);
    }

    public function delayedSize($queue = null)
    {
        return 0;
    }

    public function reservedSize($queue = null)
    {
        return 0;
    }

    public function creationTimeOfOldestPendingJob($queue = null)
    {
        return null;
    }

    public function push($job, $data = '', $queue = null)
    {
        return $this->pushRaw($this->createPayload($job, $queue ?? 'default', $data), $queue);
    }

    public function pushRaw($payload, $queue = null, array $options = [])
    {
        $result = $this->adapter->push($payload);
        if ($result !== self::DISPATCH_RESULT_ACCEPTED) {
            $this->throwOnDispatchFailure($result);
        }
    }

    public function later($delay, $job, $data = '', $queue = null)
    {
        throw new BadMethodCallException("Pogo Queue does not support delayed jobs. Use a persistent driver for scheduled tasks.");
    }

    public function pop($queue = null)
    {
        return null;
    }

    private function throwOnDispatchFailure(int $status): void
    {
        switch ($status) {
            case self::DISPATCH_RESULT_FULL:
                throw new QueueFullException("FrankenPHP in-memory queue is full. Job rejected.");
            case self::DISPATCH_RESULT_WORKER_UNAVAILABLE:
                throw new QueueWorkerUnavailableException("FrankenPHP worker is unavailable.");
            case self::DISPATCH_RESULT_PAYLOAD_TOO_LARGE:
                throw new QueuePayloadTooLargeException("FrankenPHP rejected the payload because it is larger than the queue limits.");
            case self::DISPATCH_RESULT_SHUTTING_DOWN:
                throw new QueueShuttingDownException("FrankenPHP queue is shutting down.");
            default:
                throw new QueueSendException(sprintf("FrankenPHP queue dispatch failed with status code %d.", $status));
        }
    }

    private function queueStats(): array
    {
        if (!function_exists('pogo_queue_status')) {
            return [];
        }

        $payload = \pogo_queue_status();
        $decoded = json_decode($payload, true);
        if (!is_array($decoded)) {
            return [];
        }

        return $decoded;
    }
}
