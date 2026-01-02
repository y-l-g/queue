<?php

$handler = static function ($payload = null) {
    if (is_string($payload) && !empty($payload)) {
        file_put_contents($payload, 'PROCESSED');
    }
};

$maxRequests = (int) ($_SERVER['MAX_REQUESTS'] ?? 0);
for ($nbRequests = 0; !$maxRequests || $nbRequests < $maxRequests; ++$nbRequests) {
    $keepRunning = \frankenphp_handle_request($handler);
    if (!$keepRunning)
        break;
}