<?php

require __DIR__ . '/../../vendor/autoload.php';

use Illuminate\Container\Container;
use Pogo\Queue\Laravel\Adapters\FrankenPhpAdapter;
use Pogo\Queue\Laravel\PogoJob;
use Pogo\Queue\Laravel\PogoQueue;

$container = new Container();
$queue = new PogoQueue(new FrankenPhpAdapter(), 'default');
$queue->setContainer($container);

$handler = static function ($message = null) use ($container, $queue) {
    $delivery = is_string($message) ? json_decode($message, true) : null;
    if (!is_array($delivery)) {
        return;
    }

    $job = PogoJob::fromDelivery($container, $queue, $delivery);
    $payload = json_decode((string) $job->getRawBody(), true);
    if (!is_array($payload) || !is_string($payload['marker'] ?? null)) {
        $job->fail(new RuntimeException('Missing Laravel smoke marker.'));
        return;
    }

    $job->delete();
    file_put_contents($payload['marker'], 'LARAVEL_PROCESSED');
};

$maxRequests = (int) ($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    if (!$keepRunning) {
        break;
    }
}
