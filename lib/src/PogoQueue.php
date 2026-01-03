<?php

namespace Pogo\Queue;

use BadMethodCallException;
use Illuminate\Contracts\Queue\Queue as QueueContract;
use Illuminate\Queue\Queue;
use Pogo\Queue\Exceptions\QueueFullException;
use RuntimeException;

class PogoQueue extends Queue implements QueueContract
{
    public function size($queue = null)
    {
        return 0;
    }

    public function push($job, $data = '', $queue = null)
    {
        return $this->pushRaw($this->createPayload($job, $queue, $data), $queue);
    }

    public function pushRaw($payload, $queue = null, array $options = [])
    {
        if (!function_exists('pogo_queue')) {
            throw new RuntimeException("Pogo Queue extension is not enabled.");
        }

        if (!$this->dispatchToExtension($payload)) {
            throw new QueueFullException("FrankenPHP in-memory queue is full. Job rejected.");
        }
    }

    /**
     * Dispatch the payload to the FrankenPHP extension.
     *
     * @param string $payload
     * @return bool
     */
    protected function dispatchToExtension(string $payload): bool
    {
        return \pogo_queue($payload);
    }

    public function later($delay, $job, $data = '', $queue = null)
    {
        throw new BadMethodCallException("Pogo Queue does not support delayed jobs. Use a persistent driver for scheduled tasks.");
    }

    public function pop($queue = null)
    {
        return null;
    }
}