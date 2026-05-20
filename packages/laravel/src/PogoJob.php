<?php

namespace Pogo\Queue\Laravel;

use Illuminate\Container\Container;
use Illuminate\Contracts\Queue\Job as JobContract;
use Illuminate\Queue\Jobs\Job;
use Pogo\Queue\Laravel\Contracts\PogoAdapter;
use Throwable;

class PogoJob extends Job implements JobContract
{
    protected string $payload;
    protected PogoQueue $connection;
    protected ?string $deliveryId;
    protected ?PogoAdapter $adapter;
    protected int $deliveryAttempts;

    public function __construct(Container $container, PogoQueue $connection, string $payload, string $queue, ?string $deliveryId = null, ?int $attempts = null, ?PogoAdapter $adapter = null)
    {
        $this->container = $container;
        $this->connection = $connection;
        $this->payload = $payload;
        $this->queue = $queue;
        $this->deliveryId = $deliveryId;
        $this->adapter = $adapter ?? $connection->getAdapter();
        $this->deliveryAttempts = $attempts ?? $this->attemptsFromPayload();
    }

    /**
     * @param array{id?: string, queue?: string, payload?: string, attempts?: int} $delivery
     */
    public static function fromDelivery(Container $container, PogoQueue $connection, array $delivery): self
    {
        return new self(
            $container,
            $connection,
            (string) ($delivery['payload'] ?? ''),
            (string) ($delivery['queue'] ?? 'default'),
            isset($delivery['id']) ? (string) $delivery['id'] : null,
            isset($delivery['attempts']) ? (int) $delivery['attempts'] : null,
            $connection->getAdapter(),
        );
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
        return $this->deliveryAttempts;
    }

    public function delete()
    {
        parent::delete();

        if ($this->deliveryId !== null && !$this->hasFailed() && !$this->isReleased()) {
            $this->adapter?->ack($this->queue, $this->deliveryId);
        }
    }

    public function release($delay = 0)
    {
        parent::release($delay);

        if ($this->deliveryId !== null) {
            $this->adapter?->release($this->queue, $this->deliveryId, $this->secondsUntil($delay));
        }
    }

    public function fail($e = null)
    {
        if ($this->deliveryId !== null) {
            $reason = $e instanceof Throwable ? $e->getMessage() : 'Job failed.';
            $this->adapter?->fail($this->queue, $this->deliveryId, $reason);
        }

        parent::fail($e);
    }

    private function attemptsFromPayload(): int
    {
        $decoded = json_decode($this->payload, true);
        $attempts = is_array($decoded) ? ($decoded['attempts'] ?? 1) : 1;

        return is_numeric($attempts) ? (int) $attempts : 1;
    }
}
