<?php

require __DIR__ . '/../../vendor/autoload.php';

use Illuminate\Container\Container;
use Pogo\Queue\Laravel\Adapters\FrankenPhpAdapter;
use Pogo\Queue\Laravel\PogoQueue;

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

$queue = new PogoQueue(new FrankenPhpAdapter(), 'default');
$queue->setContainer(new Container());
$payload = json_encode([
    'framework' => 'laravel',
    'marker' => $marker,
], JSON_THROW_ON_ERROR);

try {
    $id = $queue->pushRaw($payload);
    echo 'Laravel Dispatched ' . $id;
} catch (Throwable $e) {
    http_response_code(503);
    echo 'Rejected: ' . $e->getMessage();
}
