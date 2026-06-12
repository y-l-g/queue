<?php

namespace Pogo\Queue\Laravel\Tests\Unit;

use Illuminate\Container\Container;
use Illuminate\Contracts\Events\Dispatcher as DispatcherContract;
use Illuminate\Events\Dispatcher;
use PHPUnit\Framework\TestCase;
use Pogo\Queue\Laravel\Contracts\PogoAdapter;
use Pogo\Queue\Laravel\PogoJob;
use Pogo\Queue\Laravel\PogoQueue;

class FakePogoAdapter implements PogoAdapter
{
    public ?string $payload = null;
    public ?string $queue = null;
    public int $delay = 0;
    public array $acked = [];
    public array $released = [];
    public array $failed = [];

    public function push(string $queue, string $payload, int $delaySeconds = 0): string
    {
        $this->queue = $queue;
        $this->payload = $payload;
        $this->delay = $delaySeconds;

        return 'job-1';
    }

    public function ack(string $queue, string $deliveryId): void
    {
        $this->acked[] = [$queue, $deliveryId];
    }

    public function release(string $queue, string $deliveryId, int $delaySeconds = 0): void
    {
        $this->released[] = [$queue, $deliveryId, $delaySeconds];
    }

    public function fail(string $queue, string $deliveryId, string $reason = ''): void
    {
        $this->failed[] = [$queue, $deliveryId, $reason];
    }

    public function status(?string $queue = null): array
    {
        return [
            'ready' => true,
            'queues' => [[
                'queue' => $queue ?? 'default',
                'pending' => 5,
                'delayed' => 2,
                'reserved' => 3,
            ]],
        ];
    }

    public function failed(string $queue, int $limit = 100): array
    {
        return [];
    }

    public function retryFailed(string $queue, string $failedId): string
    {
        return 'job-2';
    }

    public function forgetFailed(string $queue, string $failedId): void {}

    public function purgeFailed(string $queue): int
    {
        return 0;
    }
}

class PogoQueueTest extends TestCase
{
    public function test_push_raw_dispatches_to_configured_queue(): void
    {
        $adapter = new FakePogoAdapter();
        $queue = new PogoQueue($adapter, 'mail');
        $payload = json_encode(['job' => 'Foo'], JSON_THROW_ON_ERROR);

        $id = $queue->pushRaw($payload);

        $this->assertSame('job-1', $id);
        $this->assertSame('mail', $adapter->queue);
        $this->assertSame($payload, $adapter->payload);
    }

    public function test_later_dispatches_with_delay(): void
    {
        $adapter = new FakePogoAdapter();
        $queue = new PogoQueue($adapter);
        $queue->setContainer(new Container());

        $queue->later(10, 'MyJob', ['data' => 123], 'default');

        $this->assertSame(10, $adapter->delay);
    }

    public function test_size_includes_ready_and_delayed_jobs(): void
    {
        $queue = new PogoQueue(new FakePogoAdapter());

        $this->assertSame(7, $queue->size());
        $this->assertSame(5, $queue->pendingSize());
    }

    public function test_metrics_methods_use_backend_status(): void
    {
        $queue = new PogoQueue(new FakePogoAdapter());

        $this->assertSame(2, $queue->delayedSize());
        $this->assertSame(3, $queue->reservedSize());
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

    public function test_job_lifecycle_calls_backend(): void
    {
        $adapter = new FakePogoAdapter();
        $queue = new PogoQueue($adapter);
        $job = PogoJob::fromDelivery(new Container(), $queue, [
            'id' => '1-0',
            'queue' => 'default',
            'payload' => '{"job":"MyJob","attempts":2}',
            'attempts' => 2,
        ]);

        $this->assertSame(2, $job->attempts());

        $job->release(5);
        $this->assertSame([['default', '1-0', 5]], $adapter->released);

        $ackedJob = PogoJob::fromDelivery(new Container(), $queue, [
            'id' => '2-0',
            'queue' => 'default',
            'payload' => '{"job":"MyJob"}',
        ]);
        $ackedJob->delete();
        $this->assertSame([['default', '2-0']], $adapter->acked);
    }

    public function test_job_fail_calls_backend(): void
    {
        $adapter = new FakePogoAdapter();
        $container = new Container();
        $container->instance(DispatcherContract::class, new Dispatcher($container));
        $queue = new PogoQueue($adapter);
        $payload = json_encode(['job' => \stdClass::class, 'data' => []], JSON_THROW_ON_ERROR);
        $job = PogoJob::fromDelivery($container, $queue, [
            'id' => '1-0',
            'queue' => 'default',
            'payload' => $payload,
            'attempts' => 1,
        ]);

        $job->fail(new \RuntimeException('boom'));

        $this->assertSame([['default', '1-0', 'boom']], $adapter->failed);
    }
}
