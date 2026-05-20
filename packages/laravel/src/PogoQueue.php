<?php

namespace Pogo\Queue\Laravel;

use Illuminate\Contracts\Queue\Queue as QueueContract;
use Illuminate\Queue\Queue;
use Pogo\Queue\Laravel\Contracts\PogoAdapter;

class PogoQueue extends Queue implements QueueContract
{
    protected PogoAdapter $adapter;
    protected string $defaultQueue;

    public function __construct(PogoAdapter $adapter, string $defaultQueue = 'default')
    {
        $this->adapter = $adapter;
        $this->defaultQueue = $defaultQueue;
    }

    public function size($queue = null)
    {
        $stats = $this->queueStats($this->getQueue($queue));
        $queueStats = $stats['queues'][0] ?? null;
        if (!is_array($queueStats)) {
            return 0;
        }

        return (int) (($queueStats['pending'] ?? 0) + ($queueStats['delayed'] ?? 0));
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
        $queue = $this->getQueue($queue);

        return $this->pushRaw($this->createPayload($job, $queue, $data), $queue);
    }

    public function pushRaw($payload, $queue = null, array $options = [])
    {
        return $this->adapter->push($this->getQueue($queue), $payload);
    }

    public function later($delay, $job, $data = '', $queue = null)
    {
        $queue = $this->getQueue($queue);

        return $this->adapter->push($queue, $this->createPayload($job, $queue, $data, $delay), $this->secondsUntil($delay));
    }

    public function pop($queue = null)
    {
        return null;
    }

    public function getAdapter(): PogoAdapter
    {
        return $this->adapter;
    }

    private function getQueue(?string $queue): string
    {
        return $queue ?: $this->defaultQueue;
    }

    private function queueStats(?string $queue = null): array
    {
        return $this->adapter->status($queue);
    }
}
