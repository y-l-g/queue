<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Tests\Functional;

use PHPUnit\Framework\Attributes\RunInSeparateProcess;
use Pogo\Queue\Symfony\Tests\App\Kernel;
use Pogo\Queue\Symfony\Transport\PogoQueueTransportFactory;
use Symfony\Bundle\FrameworkBundle\Test\KernelTestCase;
use Symfony\Component\Messenger\Transport\TransportFactoryInterface;
use Symfony\Component\HttpKernel\KernelInterface;

final class BundleInitializationTest extends KernelTestCase
{
    protected static function getKernelClass(): string
    {
        return Kernel::class;
    }

    protected static function createKernel(array $options = []): KernelInterface
    {
        return new Kernel($options['environment'] ?? 'test', false);
    }

    #[RunInSeparateProcess]
    public function testBundleServicesAreRegistered(): void
    {
        try {
            self::bootKernel();
            $container = static::getContainer();

            $this->assertTrue($container->has(PogoQueueTransportFactory::class));
            $transportFactory = $container->get(PogoQueueTransportFactory::class);
            $this->assertInstanceOf(PogoQueueTransportFactory::class, $transportFactory);
            $this->assertInstanceOf(TransportFactoryInterface::class, $transportFactory);
        } finally {
            self::ensureKernelShutdown();
        }
    }
}
