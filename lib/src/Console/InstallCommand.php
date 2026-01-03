<?php

namespace Pogo\Queue\Console;

use Illuminate\Console\Command;
use Illuminate\Support\Str;

class InstallCommand extends Command
{
    protected $signature = 'pogo:queue:install';
    protected $description = 'Install the Pogo Queue components and configuration';

    public function handle()
    {
        $this->info('Installing Pogo Queue...');

        $this->publishStubs();
        $this->updateEnvFile();

        $this->newLine();
        $this->info('Installation complete.');

        $this->displayConfigInstructions();
    }

    protected function publishStubs()
    {
        if (!file_exists(public_path('queue-worker.php'))) {
            copy(__DIR__ . '/../../stubs/queue-worker.php', public_path('queue-worker.php'));
            $this->comment('Created public/queue-worker.php');
        } else {
            $this->warn('public/queue-worker.php already exists.');
        }

        if (!file_exists(base_path('Caddyfile'))) {
            copy(__DIR__ . '/../../stubs/Caddyfile', base_path('Caddyfile'));
            $this->comment('Created Caddyfile example.');
        } else {
            $this->warn('Caddyfile already exists. Please verify the configuration.');
        }
    }

    protected function updateEnvFile()
    {
        $envPath = base_path('.env');

        if (!file_exists($envPath)) {
            return;
        }

        $content = file_get_contents($envPath);
        $updated = false;

        if (!Str::contains($content, 'QUEUE_CONNECTION=')) {
            $content .= "\nQUEUE_CONNECTION=pogo\n";
            $updated = true;
            $this->info('Added QUEUE_CONNECTION=pogo to .env file.');
        } elseif (preg_match('/^QUEUE_CONNECTION=(?!pogo).*/m', $content)) {
            $this->warn('QUEUE_CONNECTION is set to something else in .env. Please update it to "pogo" manually if desired.');
        }

        if ($updated) {
            file_put_contents($envPath, $content);
        }
    }

    protected function displayConfigInstructions()
    {
        $this->newLine();
        $this->warn('ACTION REQUIRED: Configure config/queue.php');
        $this->line('Please add the following connection to your config/queue.php file in the "connections" array:');
        $this->newLine();

        $this->line('<fg=gray>' . $this->getConfigSnippet() . '</>');

        $this->newLine();
        $this->comment('Run Octane with: php artisan octane:start --server=frankenphp --caddyfile=Caddyfile');
    }

    protected function getConfigSnippet()
    {
        return <<<PHP
        'pogo' => [
            'driver' => 'pogo',
            'queue' => env('POGO_QUEUE', 'default'),
            'retry_after' => 90,
        ],
PHP;
    }
}