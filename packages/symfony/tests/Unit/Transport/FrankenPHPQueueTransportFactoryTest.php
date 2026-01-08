<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Tests\Unit\Transport;

use PHPUnit\Framework\TestCase;
use Pogo\Queue\Symfony\Transport\FrankenPHPQueueTransport;
use Pogo\Queue\Symfony\Transport\FrankenPHPQueueTransportFactory;
use Symfony\Component\Messenger\Transport\Serialization\SerializerInterface;

final class FrankenPHPQueueTransportFactoryTest extends TestCase
{
    public function testSupportsCorrectDsn(): void
    {
        $factory = new FrankenPHPQueueTransportFactory();

        $this->assertTrue($factory->supports('frankenphp-queue://default', []));
        $this->assertFalse($factory->supports('redis://default', []));
    }

    public function testCreateTransport(): void
    {
        $factory = new FrankenPHPQueueTransportFactory();
        $serializer = $this->createMock(SerializerInterface::class);

        $transport = $factory->createTransport('frankenphp-queue://default', [], $serializer);

        $this->assertInstanceOf(FrankenPHPQueueTransport::class, $transport);
    }
}
