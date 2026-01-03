<?php

namespace Pogo\Queue;

use Illuminate\Queue\Connectors\ConnectorInterface;

class PogoConnector implements ConnectorInterface
{
    /**
     * @param array<string, mixed> $config
     */
    public function connect(array $config)
    {
        return new PogoQueue();
    }
}
