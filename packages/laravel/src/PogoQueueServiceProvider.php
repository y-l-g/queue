<?php

namespace Pogo\Queue\Laravel;

use Illuminate\Support\ServiceProvider;
use Illuminate\Support\Facades\Queue;
use Pogo\Queue\Laravel\PogoConnector;

class PogoQueueServiceProvider extends ServiceProvider
{
    public function boot(): void
    {
        Queue::extend('pogo', function () {
            return new PogoConnector();
        });
    }
}
