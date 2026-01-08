<?php

namespace Pogo\Queue\Laravel;

use Illuminate\Container\Container;
use Illuminate\Contracts\Queue\Job as JobContract;
use Illuminate\Queue\Jobs\Job;

class PogoJob extends Job implements JobContract
{
    protected string $payload;
    protected PogoQueue $connection;

    public function __construct(Container $container, PogoQueue $connection, string $payload, string $queue)
    {
        $this->container = $container;
        $this->connection = $connection;
        $this->payload = $payload;
        $this->queue = $queue;
    }

    public function getJobId()
    {
        $decoded = json_decode($this->payload, true);
        $id = is_array($decoded) ? ($decoded['id'] ?? null) : null;

        return (is_string($id) || is_int($id)) ? $id : null;
    }

    public function getRawBody()
    {
        return $this->payload;
    }

    public function attempts()
    {
        $decoded = json_decode($this->payload, true);
        $attempts = is_array($decoded) ? ($decoded['attempts'] ?? 1) : 1;

        return is_numeric($attempts) ? (int) $attempts : 1;
    }
}
