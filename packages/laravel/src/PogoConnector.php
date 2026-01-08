<?php

namespace Pogo\Queue\Laravel;

use Illuminate\Queue\Connectors\ConnectorInterface;
use Pogo\Queue\Laravel\Adapters\FrankenPhpAdapter;

class PogoConnector implements ConnectorInterface
{
    public function connect(array $config)
    {
        return new PogoQueue(new FrankenPhpAdapter());
    }
}
