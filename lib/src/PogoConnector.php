<?php

namespace Pogo\Queue;

use Illuminate\Queue\Connectors\ConnectorInterface;

class PogoConnector implements ConnectorInterface
{
    public function connect(array $config)
    {
        return new PogoQueue();
    }
}