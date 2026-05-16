<?php

namespace Pogo\Queue\Laravel\Tests\Unit;

use BadMethodCallException;
use Illuminate\Container\Container;
use PHPUnit\Framework\TestCase;
use Pogo\Queue\Laravel\Contracts\PogoAdapter;
use Pogo\Queue\Laravel\Exceptions\QueueFullException;
use Pogo\Queue\Laravel\Exceptions\QueuePayloadTooLargeException;
use Pogo\Queue\Laravel\Exceptions\QueueShuttingDownException;
use Pogo\Queue\Laravel\Exceptions\QueueSendException;
use Pogo\Queue\Laravel\Exceptions\QueueWorkerUnavailableException;
use Pogo\Queue\Laravel\PogoQueue;

if (!function_exists('pogo_queue')) {
    function pogo_queue($data)
    {
        return 1;
    }
}

if (!function_exists('pogo_queue_status')) {
    function pogo_queue_status()
    {
        return '{"current_depth":5,"enqueued":10,"dispatched":4,"dropped_full":0,"dropped_payload_too_large":0,"dropped_shutdown":0,"send_errors":0,"max_message_bytes":1048576}';
    }
}

class FakePogoAdapter implements PogoAdapter
{
    public ?string $payload = null;

    public function __construct(private int $result = 1)
    {
    }

    public function push(string $payload): int
    {
        $this->payload = $payload;

        return $this->result;
    }
}

class PogoQueueTest extends TestCase
{
    public function test_push_raw_dispatches_successfully(): void
    {
        $adapter = new FakePogoAdapter();
        $queue = new PogoQueue($adapter);
        $payload = json_encode(['job' => 'Foo'], JSON_THROW_ON_ERROR);

        $queue->pushRaw($payload);

        $this->assertSame($payload, $adapter->payload);
    }

    public function test_push_raw_throws_exception_when_queue_is_full(): void
    {
        $this->expectException(QueueFullException::class);
        $this->expectExceptionMessage('FrankenPHP in-memory queue is full. Job rejected.');

        $queue = new PogoQueue(new FakePogoAdapter(0));
        $queue->pushRaw('{"job":"test"}');
    }

    public function test_push_raw_throws_exception_when_worker_unavailable(): void
    {
        $this->expectException(QueueWorkerUnavailableException::class);

        $queue = new PogoQueue(new FakePogoAdapter(2));
        $queue->pushRaw('{"job":"test"}');
    }

    public function test_push_raw_throws_exception_when_payload_too_large(): void
    {
        $this->expectException(QueuePayloadTooLargeException::class);

        $queue = new PogoQueue(new FakePogoAdapter(3));
        $queue->pushRaw('{"job":"test"}');
    }

    public function test_push_raw_throws_exception_when_queue_is_shutting_down(): void
    {
        $this->expectException(QueueShuttingDownException::class);

        $queue = new PogoQueue(new FakePogoAdapter(4));
        $queue->pushRaw('{"job":"test"}');
    }

    public function test_push_raw_throws_exception_on_unknown_status(): void
    {
        $this->expectException(QueueSendException::class);

        $queue = new PogoQueue(new FakePogoAdapter(99));
        $queue->pushRaw('{"job":"test"}');
    }

    public function test_later_throws_bad_method_call_exception(): void
    {
        $queue = new PogoQueue(new FakePogoAdapter());

        $this->expectException(BadMethodCallException::class);
        $this->expectExceptionMessage('Pogo Queue does not support delayed jobs. Use a persistent driver for scheduled tasks.');

        $queue->later(10, 'Job');
    }

    public function test_size_reflects_dispatcher_depth(): void
    {
        $queue = new PogoQueue(new FakePogoAdapter());
        $this->assertSame(5, $queue->size());
        $this->assertSame(5, $queue->pendingSize());
    }

    public function test_metrics_methods_without_driver_are_defaulted_to_zero(): void
    {
        $queue = new PogoQueue(new FakePogoAdapter());

        $this->assertSame(0, $queue->delayedSize());
        $this->assertSame(0, $queue->reservedSize());
        $this->assertNull($queue->creationTimeOfOldestPendingJob());
    }

    public function test_push_uses_create_payload(): void
    {
        $adapter = new FakePogoAdapter();
        $queue = new PogoQueue($adapter);
        $queue->setContainer(new Container());

        $queue->push('MyJob', ['data' => 123]);

        $this->assertNotNull($adapter->payload);
        $decoded = json_decode($adapter->payload, true);

        $this->assertSame('MyJob', $decoded['job']);
        $this->assertSame(123, $decoded['data']['data']);
    }
}
