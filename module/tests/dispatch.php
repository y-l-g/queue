<?php

$payload = file_get_contents('php://input');

$sent = pogo_queue($payload);

if ($sent) {
    echo "Dispatched";
} else {
    http_response_code(503);
    echo "Failed";
}