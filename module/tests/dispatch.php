<?php

$payload = file_get_contents('php://input');

$sent = pogo_queue($payload);

if ($sent === 1) {
    echo "Dispatched";
} else {
    http_response_code(503);
    echo "Rejected with status {$sent}";
}
