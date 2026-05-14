<?php

namespace Pogo\Queue\Laravel\Tests\Unit;

use BadMethodCallException;
use Illuminate\Container\Container;
use PHPUnit\Framework\TestCase;
use Pogo\Queue\Laravel\Contracts\PogoAdapter;
use Pogo\Queue\Laravel\Exceptions\QueueFullException;
use Pogo\Queue\Laravel\PogoQueue;

// Polyfill check is still good practice to prevent fatal errors if the test suite
// accidentally loads the real adapter, even though we mock it here.
if (!function_exists('pogo_queue')) {
    function pogo_queue($data)
    {
        return false;
    }
}

class FakePogoAdapter implements PogoAdapter
{
    public ?string $payload = null;

    public function __construct(private bool $result = true)
    {
    }

    public function push(string $payload): bool
    {
        $this->payload = $payload;

        return $this->result;
    }
}

class PogoQueueTest extends TestCase
{
    public function test_push_raw_dispatches_successfully()
    {
        $adapter = new FakePogoAdapter();
        $queue = new PogoQueue($adapter);
        $payload = json_encode(['job' => 'Foo']);

        $queue->pushRaw($payload);

        $this->assertSame('{"job":"Foo"}', $adapter->payload);
    }

    public function test_push_raw_throws_exception_when_queue_is_full()
    {
        $this->expectException(QueueFullException::class);
        $this->expectExceptionMessage('FrankenPHP in-memory queue is full. Job rejected.');

        $adapter = new FakePogoAdapter(false);
        $queue = new PogoQueue($adapter);

        $queue->pushRaw('{"job":"test"}');
    }

    public function test_later_throws_bad_method_call_exception()
    {
        $queue = new PogoQueue(new FakePogoAdapter());

        $this->expectException(BadMethodCallException::class);
        $this->expectExceptionMessage('Pogo Queue does not support delayed jobs');

        $queue->later(10, 'Job');
    }

    public function test_size_returns_zero()
    {
        $queue = new PogoQueue(new FakePogoAdapter());

        $this->assertEquals(0, $queue->size());
    }

    public function test_queue_metrics_return_empty_values()
    {
        $queue = new PogoQueue(new FakePogoAdapter());

        $this->assertEquals(0, $queue->pendingSize());
        $this->assertEquals(0, $queue->delayedSize());
        $this->assertEquals(0, $queue->reservedSize());
        $this->assertNull($queue->creationTimeOfOldestPendingJob());
    }

    public function test_push_uses_create_payload()
    {
        $adapter = new FakePogoAdapter();
        $queue = new PogoQueue($adapter);
        $queue->setContainer(new Container());

        $queue->push('MyJob', ['data' => 123]);

        $this->assertNotNull($adapter->payload);
        $decoded = json_decode($adapter->payload, true);

        $this->assertEquals('MyJob', $decoded['job']);
        $this->assertEquals(123, $decoded['data']['data']);
    }
}
