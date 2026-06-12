<?php

/** @generate-class-entries */

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

function pogo_queue_status(?string $queue = null): string
{
}

function pogo_queue_failed(string $queue, int $limit = 100): string
{
}

function pogo_queue_retry_failed(string $queue, string $failedId): string
{
}

function pogo_queue_forget_failed(string $queue, string $failedId): string
{
}

function pogo_queue_purge_failed(string $queue): string
{
}
