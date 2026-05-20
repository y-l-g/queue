<?php

if (($_SERVER['REQUEST_METHOD'] ?? 'GET') !== 'POST') {
    echo 'Ready';
    return;
}

$payload = file_get_contents('php://input');
if ($payload === '') {
    http_response_code(400);
    echo 'Missing payload';
    return;
}

$result = json_decode(pogo_queue_push('default', $payload), true);

if (($result['ok'] ?? false) === true) {
    echo "Dispatched";
} else {
    http_response_code(503);
    echo "Rejected: " . ($result['message'] ?? 'unknown error');
}
