<?php

$handler = static function ($message = null) {
    $delivery = is_string($message) ? json_decode($message, true) : null;
    if (is_array($delivery) && is_string($delivery['payload'] ?? null)) {
        file_put_contents($delivery['payload'], 'PROCESSED');
        pogo_queue_ack($delivery['queue'], $delivery['id']);
    }
};

$maxRequests = (int) ($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    if (!$keepRunning)
        break;
}
