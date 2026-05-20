<?php

function pogo_queue_push(string $queue, string $payload, int $delaySeconds = 0): string
{
}

function pogo_queue_ack(string $queue, string $deliveryId): int
{
}

function pogo_queue_release(string $queue, string $deliveryId, int $delaySeconds = 0): int
{
}

function pogo_queue_fail(string $queue, string $deliveryId, string $reason = ''): int
{
}
