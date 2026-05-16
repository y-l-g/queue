<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Transport;

use Pogo\Queue\Symfony\Contract\PogoAdapter;
use Symfony\Component\Messenger\Envelope;
use Symfony\Component\Messenger\Transport\Serialization\PhpSerializer;
use Symfony\Component\Messenger\Transport\Serialization\SerializerInterface;
use Symfony\Component\Messenger\Transport\TransportInterface;

final class PogoQueueTransport implements TransportInterface
{
    private const DISPATCH_RESULT_ACCEPTED = 1;
    private const DISPATCH_RESULT_FULL = 0;
    private const DISPATCH_RESULT_WORKER_UNAVAILABLE = 2;
    private const DISPATCH_RESULT_PAYLOAD_TOO_LARGE = 3;
    private const DISPATCH_RESULT_SHUTTING_DOWN = 4;

    public function __construct(
        private readonly PogoAdapter $adapter,
        private readonly SerializerInterface $serializer = new PhpSerializer(),
    ) {}

    public function get(): iterable
    {
        $envelope = null;

        $this->adapter->handle(function (string $message) use (&$envelope) {
            try {
                $decodedEnvelope = $this->serializer->decode([
                    'body' => $message,
                ]);
            } catch (\Throwable) {
                return;
            }

            if (!$decodedEnvelope instanceof Envelope) {
                return;
            }

            $envelope = $decodedEnvelope;
        });

        if ($envelope !== null) {
            return [$envelope];
        }

        return [];
    }

    public function ack(Envelope $envelope): void
    {
        // The in-memory worker has already removed the message from the queue.
    }

    public function reject(Envelope $envelope): void
    {
        // Symfony Messenger owns retry and failure transport behavior.
    }

    public function send(Envelope $envelope): Envelope
    {
        $encoded = $this->serializer->encode($envelope);
        $payload = (string) ($encoded['body'] ?? '');
        $status = $this->adapter->push($payload);

        $this->assertDispatchSucceeded($status);

        return $envelope;
    }

    private function assertDispatchSucceeded(int $status): void
    {
        switch ($status) {
            case self::DISPATCH_RESULT_ACCEPTED:
                return;
            case self::DISPATCH_RESULT_FULL:
                throw new QueueFullException("FrankenPHP in-memory queue is full.");
            case self::DISPATCH_RESULT_WORKER_UNAVAILABLE:
                throw new QueueWorkerUnavailableException("FrankenPHP worker is unavailable.");
            case self::DISPATCH_RESULT_PAYLOAD_TOO_LARGE:
                throw new QueuePayloadTooLargeException("Payload exceeds FrankenPHP queue limits.");
            case self::DISPATCH_RESULT_SHUTTING_DOWN:
                throw new QueueShuttingDownException("FrankenPHP queue is shutting down.");
            default:
                throw new QueueDispatchException(sprintf("FrankenPHP queue dispatch failed with status code %d.", $status));
        }
    }
}
