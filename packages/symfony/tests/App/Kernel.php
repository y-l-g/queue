<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Tests\App;

use Pogo\Queue\Symfony\PogoQueueBundle;
use Symfony\Bundle\FrameworkBundle\FrameworkBundle;
use Symfony\Bundle\FrameworkBundle\Kernel\MicroKernelTrait;
use Symfony\Component\DependencyInjection\Loader\Configurator\ContainerConfigurator;
use Symfony\Component\HttpKernel\Kernel as BaseKernel;

class Kernel extends BaseKernel
{
    use MicroKernelTrait;

    public function registerBundles(): iterable
    {
        return [
            new FrameworkBundle(),
            new PogoQueueBundle(),
        ];
    }

    protected function configureContainer(ContainerConfigurator $container): void
    {
        $container->extension('framework', [
            'test' => true,
            'messenger' => [
                'transports' => [
                    'pogo' => 'pogo-queue://default',
                ],
            ],
        ]);
    }
}
