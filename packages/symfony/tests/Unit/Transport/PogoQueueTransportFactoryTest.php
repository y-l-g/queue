<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Tests\Unit\Transport;

use PHPUnit\Framework\TestCase;
use Pogo\Queue\Symfony\Contract\PogoAdapter;
use Pogo\Queue\Symfony\Transport\PogoQueueTransport;
use Pogo\Queue\Symfony\Transport\PogoQueueTransportFactory;
use Pogo\Queue\Symfony\Transport\QueueDispatchException;
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

    public function __construct(
        private int $nextDispatchResult = 1,
        public ?string $nextReceivedMessage = null,
    ) {}

    public function push(string $payload): int
    {
        $this->pushedMessages[] = $payload;

        return $this->nextDispatchResult;
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
        $transport = new PogoQueueTransport($adapter, $serializer);

        $transport->send(new Envelope(new \stdClass()));

        $this->assertCount(1, $adapter->pushedMessages);
        $this->assertIsString($adapter->pushedMessages[0]);
    }

    public function testGetDecodesMessage(): void
    {
        $adapter = new FakeAdapter();
        $serializer = new FakeSerializer();
        $transport = new PogoQueueTransport($adapter, $serializer);

        $adapter->nextReceivedMessage = serialize(new \stdClass());

        $items = iterator_to_array($transport->get());
        $this->assertCount(1, $items);
        $envelope = $items[0];
        $this->assertInstanceOf(Envelope::class, $envelope);
    }

    public function testGetSkipsMalformedPayloadsWithoutThrowing(): void
    {
        $adapter = new FakeAdapter();
        $adapter->nextReceivedMessage = 'not-json';
        $transport = new PogoQueueTransport($adapter, new ThrowingSerializer());

        $items = iterator_to_array($transport->get());
        $this->assertSame([], $items);
    }

    public function testGetSkipsMalformedEnvelopePayloadWithoutThrowing(): void
    {
        $adapter = new FakeAdapter();
        $adapter->nextReceivedMessage = 'x';

        $transport = new PogoQueueTransport($adapter, new ThrowingSerializer());

        $items = iterator_to_array($transport->get());
        $this->assertSame([], $items);
    }

    public function testSendRejectsUnknownStatusFromAdapter(): void
    {
        $adapter = new FakeAdapter(nextDispatchResult: 99);
        $serializer = new FakeSerializer();
        $transport = new PogoQueueTransport($adapter, $serializer);

        $this->expectException(QueueDispatchException::class);
        $transport->send(new Envelope(new \stdClass()));
    }
}
