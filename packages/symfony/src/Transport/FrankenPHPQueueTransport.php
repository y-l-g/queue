<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Transport;

use Pogo\Queue\Symfony\Contract\PogoAdapter;
use Symfony\Component\Messenger\Envelope;
use Symfony\Component\Messenger\Exception\LogicException;
use Symfony\Component\Messenger\Transport\Serialization\PhpSerializer;
use Symfony\Component\Messenger\Transport\Serialization\SerializerInterface;
use Symfony\Component\Messenger\Transport\TransportInterface;

class FrankenPHPQueueTransport implements TransportInterface
{
    public function __construct(
        private readonly PogoAdapter $adapter,
        private readonly SerializerInterface $serializer = new PhpSerializer(),
    ) {}

    public function get(): iterable
    {
        $envelope = null;

        $this->adapter->handle(function (string $message) use (&$envelope) {
            $envelope = $this->serializer->decode([
                'body' => $message,
            ]);
        });

        if ($envelope) {
            return [$envelope];
        }

        return [];
    }

    public function ack(Envelope $envelope): void
    {
        throw new LogicException('Not implemented');
    }

    public function reject(Envelope $envelope): void
    {
        throw new LogicException('Not implemented');
    }

    public function send(Envelope $envelope): Envelope
    {
        $encoded = $this->serializer->encode($envelope);

        $this->adapter->push((string) $encoded['body']);

        return $envelope;
    }
}
