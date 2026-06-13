<?php

declare(strict_types=1);

require __DIR__ . '/../../vendor/autoload.php';

use Pogo\Queue\Symfony\Adapter\FrankenPhpAdapter;
use Pogo\Queue\Symfony\Transport\PogoQueueTransport;
use Symfony\Component\Messenger\Envelope;
use Symfony\Component\Messenger\Stamp\TransportMessageIdStamp;
use Symfony\Component\Messenger\Transport\Serialization\PhpSerializer;

if (($_SERVER['REQUEST_METHOD'] ?? 'GET') !== 'POST') {
    echo 'Ready';
    return;
}

$marker = trim((string) file_get_contents('php://input'));
if ($marker === '') {
    http_response_code(400);
    echo 'Missing marker';
    return;
}

$message = new stdClass();
$message->framework = 'symfony';
$message->marker = $marker;

$transport = new PogoQueueTransport(new FrankenPhpAdapter(), 'default', new PhpSerializer());

try {
    $envelope = $transport->send(new Envelope($message));
    $stamp = $envelope->last(TransportMessageIdStamp::class);
    $id = $stamp instanceof TransportMessageIdStamp ? $stamp->getId() : 'unknown';
    echo 'Symfony Dispatched ' . $id;
} catch (Throwable $e) {
    http_response_code(503);
    echo 'Rejected: ' . $e->getMessage();
}
