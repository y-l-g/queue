<?php

namespace Pogo\Queue\Laravel;

use BadMethodCallException;
use Illuminate\Contracts\Queue\Queue as QueueContract;
use Illuminate\Queue\Queue;
use Pogo\Queue\Laravel\Contracts\PogoAdapter;
use Pogo\Queue\Laravel\Exceptions\QueueFullException;

class PogoQueue extends Queue implements QueueContract
{
    protected PogoAdapter $adapter;

    public function __construct(PogoAdapter $adapter)
    {
        $this->adapter = $adapter;
    }

    public function size($queue = null)
    {
        return 0;
    }

    public function push($job, $data = '', $queue = null)
    {
        return $this->pushRaw($this->createPayload($job, $queue ?? 'default', $data), $queue);
    }

    public function pushRaw($payload, $queue = null, array $options = [])
    {
        if (!$this->adapter->push($payload)) {
            throw new QueueFullException("FrankenPHP in-memory queue is full. Job rejected.");
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
}
