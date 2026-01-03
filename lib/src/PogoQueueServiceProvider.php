<?php

namespace Pogo\Queue;

use Illuminate\Support\ServiceProvider;
use Illuminate\Support\Facades\Queue;
use Pogo\Queue\Console\InstallCommand;

class PogoQueueServiceProvider extends ServiceProvider
{
    public function boot()
    {
        Queue::extend('pogo', function () {
            return new PogoConnector;
        });

        if ($this->app->runningInConsole()) {
            $this->commands([
                InstallCommand::class,
            ]);
        }
    }
}