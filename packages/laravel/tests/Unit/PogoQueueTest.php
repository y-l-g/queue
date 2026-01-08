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

class PogoQueueTest extends TestCase
{
    public function test_push_raw_dispatches_successfully()
    {
        // Arrange: Create a mock adapter that returns true (success)
        $adapter = $this->createMock(PogoAdapter::class);
        $adapter->expects($this->once())
            ->method('push')
            ->with('{"job":"Foo"}')
            ->willReturn(true);

        $queue = new PogoQueue($adapter);
        $payload = json_encode(['job' => 'Foo']);

        // Act
        $queue->pushRaw($payload);

        // Assert: No exception thrown implies success, and mock verification passes
        $this->assertTrue(true);
    }

    public function test_push_raw_throws_exception_when_queue_is_full()
    {
        $this->expectException(QueueFullException::class);
        $this->expectExceptionMessage('FrankenPHP in-memory queue is full. Job rejected.');

        // Arrange: Create a mock adapter that returns false (failure/full)
        $adapter = $this->createMock(PogoAdapter::class);
        $adapter->expects($this->once())
            ->method('push')
            ->willReturn(false);

        $queue = new PogoQueue($adapter);

        // Act
        $queue->pushRaw('{"job":"test"}');
    }

    public function test_later_throws_bad_method_call_exception()
    {
        $adapter = $this->createMock(PogoAdapter::class);
        $queue = new PogoQueue($adapter);

        $this->expectException(BadMethodCallException::class);
        $this->expectExceptionMessage('Pogo Queue does not support delayed jobs');

        $queue->later(10, 'Job');
    }

    public function test_size_returns_zero()
    {
        $adapter = $this->createMock(PogoAdapter::class);
        $queue = new PogoQueue($adapter);

        $this->assertEquals(0, $queue->size());
    }

    public function test_push_uses_create_payload()
    {
        // Arrange: We capture the payload passed to the adapter
        $adapter = $this->createMock(PogoAdapter::class);
        $capturedPayload = null;

        $adapter->method('push')
            ->willReturnCallback(function ($payload) use (&$capturedPayload) {
                $capturedPayload = $payload;
                return true;
            });

        $queue = new PogoQueue($adapter);
        $queue->setContainer($this->createMock(Container::class));

        // Act
        $queue->push('MyJob', ['data' => 123]);

        // Assert
        $this->assertNotNull($capturedPayload);
        $decoded = json_decode($capturedPayload, true);

        $this->assertEquals('MyJob', $decoded['job']);
        $this->assertEquals(123, $decoded['data']['data']);
    }
}
