<?php

namespace Pogo\Queue;

use Illuminate\Container\Container;
use Illuminate\Contracts\Queue\Job as JobContract;
use Illuminate\Queue\Jobs\Job;

class PogoJob extends Job implements JobContract
{
    protected $payload;

    public function __construct(Container $container, PogoQueue $connection, $payload, $queue)
    {
        $this->container = $container;
        $this->connection = $connection;
        $this->payload = $payload;
        $this->queue = $queue;
    }

    public function getJobId()
    {
        return json_decode($this->payload, true)['id'] ?? null;
    }

    public function getRawBody()
    {
        return $this->payload;
    }

    public function attempts()
    {
        return json_decode($this->payload, true)['attempts'] ?? 1;
    }
}