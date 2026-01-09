<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Transport;

use Pogo\Queue\Symfony\Adapter\FrankenPhpAdapter;
use Symfony\Component\Messenger\Transport\Serialization\SerializerInterface;
use Symfony\Component\Messenger\Transport\TransportFactoryInterface;
use Symfony\Component\Messenger\Transport\TransportInterface;

final class FrankenPHPQueueTransportFactory implements TransportFactoryInterface
{
    public function createTransport(string $dsn, array $options, SerializerInterface $serializer): TransportInterface
    {
        return new FrankenPHPQueueTransport(new FrankenPhpAdapter(), $serializer);
    }

    public function supports(string $dsn, array $options): bool
    {
        return str_starts_with($dsn, 'pogo-queue://');
    }
}
