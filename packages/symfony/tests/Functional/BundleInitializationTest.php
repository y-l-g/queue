<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Tests\Functional;

use PHPUnit\Framework\TestCase;
use Pogo\Queue\Symfony\Tests\App\Kernel;
use Pogo\Queue\Symfony\Transport\PogoQueueTransportFactory;
use Symfony\Component\Messenger\Transport\TransportFactoryInterface;

final class BundleInitializationTest extends TestCase
{
    public function testBundleServicesAreRegistered(): void
    {
        $kernel = new Kernel('test', true);
        $kernel->boot();

        $container = $kernel->getContainer();

        $this->assertTrue($container->has('messenger.transport_factory'));


        $transportFactory = $container->get(PogoQueueTransportFactory::class);
        $this->assertInstanceOf(PogoQueueTransportFactory::class, $transportFactory);
        $this->assertInstanceOf(TransportFactoryInterface::class, $transportFactory);
    }
}
