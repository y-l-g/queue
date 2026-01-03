<?php

namespace Pogo\Queue\Tests\Unit;

use PHPUnit\Framework\TestCase;
use Pogo\Queue\PogoQueue;
use Pogo\Queue\Exceptions\QueueFullException;
use BadMethodCallException;

// Polyfill for standard PHP environments (CI/CD) where the extension is missing.
// This ensures pushRaw passes the function_exists check.
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
        $queue = new class extends PogoQueue {
            protected function dispatchToExtension(string $payload): bool
            {
                return true;
            }
        };

        $payload = json_encode(['job' => 'Foo']);

        // Should not throw exception
        $queue->pushRaw($payload);

        $this->assertTrue(true);
    }

    public function test_push_raw_throws_exception_when_queue_is_full()
    {
        $this->expectException(QueueFullException::class);
        $this->expectExceptionMessage('FrankenPHP in-memory queue is full. Job rejected.');

        $queue = new class extends PogoQueue {
            protected function dispatchToExtension(string $payload): bool
            {
                return false;
            }
        };

        $queue->pushRaw('{"job":"test"}');
    }

    public function test_later_throws_bad_method_call_exception()
    {
        $queue = new PogoQueue();

        $this->expectException(BadMethodCallException::class);
        $this->expectExceptionMessage('Pogo Queue does not support delayed jobs');

        $queue->later(10, 'Job');
    }

    public function test_size_returns_zero()
    {
        $queue = new PogoQueue();
        $this->assertEquals(0, $queue->size());
    }

    public function test_push_uses_create_payload()
    {
        // We use a mock to intercept the call to pushRaw and inspect logic
        $queue = new class extends PogoQueue {
            public $lastPayload;

            // Override pushRaw to bypass the extension check entirely for this specific test
            // as we are testing the interaction between push() and pushRaw()
            public function pushRaw($payload, $queue = null, array $options = [])
            {
                $this->lastPayload = $payload;
            }
        };

        $queue->setContainer($this->createMock(\Illuminate\Container\Container::class));

        $queue->push('MyJob', ['data' => 123]);

        $decoded = json_decode($queue->lastPayload, true);
        $this->assertEquals('MyJob', $decoded['job']);
        $this->assertEquals(123, $decoded['data']['data']);
    }
}