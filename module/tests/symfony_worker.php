<?php

declare(strict_types=1);

require __DIR__ . '/../../vendor/autoload.php';

use Pogo\Queue\Symfony\Adapter\FrankenPhpAdapter;
use Pogo\Queue\Symfony\Transport\PogoQueueTransport;
use Symfony\Component\Messenger\Envelope;
use Symfony\Component\Messenger\Transport\Serialization\PhpSerializer;

$transport = new PogoQueueTransport(new FrankenPhpAdapter(), 'default', new PhpSerializer());

$maxRequests = (int) ($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $processed = false;

    foreach ($transport->get() as $envelope) {
        if (!$envelope instanceof Envelope) {
            continue;
        }

        $message = $envelope->getMessage();
        if (!$message instanceof stdClass || !is_string($message->marker ?? null)) {
            $transport->reject($envelope);
            continue;
        }

        $transport->ack($envelope);
        file_put_contents($message->marker, 'SYMFONY_PROCESSED');
        $processed = true;
    }

    if (!$processed) {
        break;
    }
}
