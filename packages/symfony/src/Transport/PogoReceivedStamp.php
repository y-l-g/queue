<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Transport;

use Symfony\Component\Messenger\Stamp\NonSendableStampInterface;

final class PogoReceivedStamp implements NonSendableStampInterface
{
    public function __construct(
        public readonly string $queue,
        public readonly string $deliveryId,
        public readonly int $attempts,
    ) {}
}
