<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Transport;

use Pogo\Queue\Symfony\Contract\PogoAdapter;
use Symfony\Component\Messenger\Envelope;
use Symfony\Component\Messenger\Stamp\DelayStamp;
use Symfony\Component\Messenger\Stamp\SentForRetryStamp;
use Symfony\Component\Messenger\Stamp\TransportMessageIdStamp;
use Symfony\Component\Messenger\Transport\Serialization\PhpSerializer;
use Symfony\Component\Messenger\Transport\Serialization\SerializerInterface;
use Symfony\Component\Messenger\Transport\TransportInterface;

final class PogoQueueTransport implements TransportInterface
{
    public function __construct(
        private readonly PogoAdapter $adapter,
        private readonly string $queue = 'default',
        private readonly SerializerInterface $serializer = new PhpSerializer(),
    ) {}

    public function get(): iterable
    {
        $envelope = null;

        $this->adapter->handle(function (string $message) use (&$envelope) {
            $delivery = json_decode($message, true);
            $queue = is_array($delivery) && is_string($delivery['queue'] ?? null) ? $delivery['queue'] : $this->queue;
            $deliveryId = is_array($delivery) && is_string($delivery['id'] ?? null) ? $delivery['id'] : '';
            if (!is_array($delivery) || !is_string($delivery['payload'] ?? null) || $deliveryId === '') {
                if ($deliveryId !== '') {
                    $this->adapter->fail($queue, $deliveryId, 'Malformed queue delivery.');
                }
                return;
            }

            $attempts = (int) ($delivery['attempts'] ?? 1);

            try {
                $decodedEnvelope = $this->serializer->decode([
                    'body' => $delivery['payload'],
                ]);
            } catch (\Throwable) {
                $this->adapter->fail($queue, $deliveryId, 'Message could not be decoded.');
                return;
            }

            $envelope = $decodedEnvelope->with(
                new PogoReceivedStamp($queue, $deliveryId, $attempts),
                new TransportMessageIdStamp($deliveryId),
            );
        });

        if ($envelope !== null) {
            return [$envelope];
        }

        return [];
    }

    public function ack(Envelope $envelope): void
    {
        $stamp = $envelope->last(PogoReceivedStamp::class);
        if (!$stamp instanceof PogoReceivedStamp) {
            return;
        }

        $this->adapter->ack($stamp->queue, $stamp->deliveryId);
    }

    public function reject(Envelope $envelope): void
    {
        $stamp = $envelope->last(PogoReceivedStamp::class);
        if (!$stamp instanceof PogoReceivedStamp) {
            return;
        }

        $sentForRetry = $envelope->last(SentForRetryStamp::class);
        if ($sentForRetry instanceof SentForRetryStamp && $sentForRetry->isSent) {
            $this->adapter->ack($stamp->queue, $stamp->deliveryId);
            return;
        }

        $this->adapter->fail($stamp->queue, $stamp->deliveryId, 'Message rejected by Symfony Messenger.');
    }

    public function send(Envelope $envelope): Envelope
    {
        $encoded = $this->serializer->encode($envelope);
        $payload = $encoded['body'];
        $delay = $envelope->last(DelayStamp::class);
        $delaySeconds = $delay instanceof DelayStamp ? (int) ceil($delay->getDelay() / 1000) : 0;
        $id = $this->adapter->push($this->queue, $payload, $delaySeconds);

        return $envelope->with(new TransportMessageIdStamp($id));
    }
}
