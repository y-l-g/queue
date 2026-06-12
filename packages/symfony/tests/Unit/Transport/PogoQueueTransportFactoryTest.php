<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Tests\Unit\Transport;

use PHPUnit\Framework\TestCase;
use Pogo\Queue\Symfony\Contract\PogoAdapter;
use Pogo\Queue\Symfony\Transport\PogoQueueTransport;
use Pogo\Queue\Symfony\Transport\PogoQueueTransportFactory;
use Symfony\Component\Messenger\Envelope;
use Symfony\Component\Messenger\Transport\Serialization\SerializerInterface;

final class FakeSerializer implements SerializerInterface
{
    public function decode(array $encodedEnvelope): Envelope
    {
        return new Envelope(unserialize($encodedEnvelope['body']));
    }

    public function encode(Envelope $envelope): array
    {
        return ['body' => serialize($envelope->getMessage())];
    }
}

final class ThrowingSerializer implements SerializerInterface
{
    public function decode(array $encodedEnvelope): Envelope
    {
        throw new \RuntimeException('bad payload');
    }

    public function encode(Envelope $envelope): array
    {
        return ['body' => serialize($envelope->getMessage())];
    }
}

final class FakeAdapter implements PogoAdapter
{
    /** @var list<string> */
    public array $pushedMessages = [];
    public array $acked = [];
    public array $failed = [];

    public function __construct(
        public ?string $nextReceivedMessage = null,
    ) {}

    public function push(string $queue, string $payload, int $delaySeconds = 0): string
    {
        $this->pushedMessages[] = $payload;

        return 'job-1';
    }

    public function ack(string $queue, string $deliveryId): void
    {
        $this->acked[] = [$queue, $deliveryId];
    }

    public function release(string $queue, string $deliveryId, int $delaySeconds = 0): void {}

    public function fail(string $queue, string $deliveryId, string $reason = ''): void
    {
        $this->failed[] = [$queue, $deliveryId, $reason];
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

    public function handle(callable $callback): bool
    {
        if ($this->nextReceivedMessage === null) {
            return false;
        }

        $callback($this->nextReceivedMessage);
        $this->nextReceivedMessage = null;

        return true;
    }
}

final class PogoQueueTransportFactoryTest extends TestCase
{
    public function testSupportsCorrectDsn(): void
    {
        $factory = new PogoQueueTransportFactory();

        $this->assertTrue($factory->supports('pogo-queue://default', []));
        $this->assertFalse($factory->supports('redis://default', []));
    }

    public function testCreateTransport(): void
    {
        $factory = new PogoQueueTransportFactory();

        $transport = $factory->createTransport('pogo-queue://default', [], new FakeSerializer());

        $this->assertInstanceOf(PogoQueueTransport::class, $transport);
    }

    public function testSendEncodesAndDispatches(): void
    {
        $adapter = new FakeAdapter();
        $serializer = new FakeSerializer();
        $transport = new PogoQueueTransport($adapter, 'default', $serializer);

        $transport->send(new Envelope(new \stdClass()));

        $this->assertCount(1, $adapter->pushedMessages);
        $this->assertIsString($adapter->pushedMessages[0]);
    }

    public function testGetDecodesMessage(): void
    {
        $adapter = new FakeAdapter();
        $serializer = new FakeSerializer();
        $transport = new PogoQueueTransport($adapter, 'default', $serializer);

        $adapter->nextReceivedMessage = json_encode([
            'id' => '1-0',
            'queue' => 'default',
            'payload' => serialize(new \stdClass()),
            'attempts' => 2,
        ], JSON_THROW_ON_ERROR);

        $items = iterator_to_array($transport->get());
        $this->assertCount(1, $items);
        $envelope = $items[0];
        $this->assertInstanceOf(Envelope::class, $envelope);
    }

    public function testGetSkipsMalformedPayloadsWithoutThrowing(): void
    {
        $adapter = new FakeAdapter();
        $adapter->nextReceivedMessage = json_encode([
            'id' => '1-0',
            'queue' => 'default',
            'payload' => 'not-json',
            'attempts' => 1,
        ], JSON_THROW_ON_ERROR);
        $transport = new PogoQueueTransport($adapter, 'default', new ThrowingSerializer());

        $items = iterator_to_array($transport->get());
        $this->assertSame([], $items);
        $this->assertSame([['default', '1-0', 'Message could not be decoded.']], $adapter->failed);
    }

    public function testGetSkipsMalformedEnvelopePayloadWithoutThrowing(): void
    {
        $adapter = new FakeAdapter();
        $adapter->nextReceivedMessage = 'x';

        $transport = new PogoQueueTransport($adapter, 'default', new ThrowingSerializer());

        $items = iterator_to_array($transport->get());
        $this->assertSame([], $items);
    }

    public function testGetFailsMalformedDeliveryWhenItHasDeliveryId(): void
    {
        $adapter = new FakeAdapter();
        $adapter->nextReceivedMessage = json_encode([
            'id' => '1-0',
            'queue' => 'default',
            'attempts' => 1,
        ], JSON_THROW_ON_ERROR);

        $transport = new PogoQueueTransport($adapter, 'default', new FakeSerializer());

        $items = iterator_to_array($transport->get());
        $this->assertSame([], $items);
        $this->assertSame([['default', '1-0', 'Malformed queue delivery.']], $adapter->failed);
    }

    public function testAckCallsBackend(): void
    {
        $adapter = new FakeAdapter();
        $serializer = new FakeSerializer();
        $transport = new PogoQueueTransport($adapter, 'default', $serializer);

        $adapter->nextReceivedMessage = json_encode([
            'id' => '1-0',
            'queue' => 'default',
            'payload' => serialize(new \stdClass()),
            'attempts' => 1,
        ], JSON_THROW_ON_ERROR);

        $items = iterator_to_array($transport->get());
        $transport->ack($items[0]);

        $this->assertSame([['default', '1-0']], $adapter->acked);
    }
}
