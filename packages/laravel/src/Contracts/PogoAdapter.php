<?php

namespace Pogo\Queue\Laravel\Contracts;

interface PogoAdapter
{
    public function push(string $payload): int;
}
