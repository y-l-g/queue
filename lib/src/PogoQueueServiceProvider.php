<?php

namespace Pogo\Queue;

use Illuminate\Support\ServiceProvider;
use Illuminate\Support\Facades\Queue;

class PogoQueueServiceProvider extends ServiceProvider
{
    public function boot(): void
    {
        Queue::extend('pogo', function () {
            return new PogoConnector();
        });

        // if ($this->app->runningInConsole()) {
        //     $this->commands([
        //         InstallCommand::class,
        //     ]);
        // }
    }
}
