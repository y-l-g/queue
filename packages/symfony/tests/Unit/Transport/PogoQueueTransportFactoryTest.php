<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Tests\Unit\Transport;

use PHPUnit\Framework\TestCase;
use Pogo\Queue\Symfony\Transport\PogoQueueTransport;
use Pogo\Queue\Symfony\Transport\PogoQueueTransportFactory;
use Symfony\Component\Messenger\Envelope;
use Symfony\Component\Messenger\Transport\Serialization\SerializerInterface;

final class FakeSerializer implements SerializerInterface
{
    public function decode(array $encodedEnvelope): Envelope
    {
        return new Envelope((object) $encodedEnvelope);
    }

    public function encode(Envelope $envelope): array
    {
        return ['body' => serialize($envelope->getMessage())];
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
}
