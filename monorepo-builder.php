<?php

declare(strict_types=1);

use Symplify\ComposerJsonManipulator\ValueObject\ComposerJsonSection;
use Symplify\MonorepoBuilder\Config\MBConfig;

return static function (MBConfig $mbConfig): void {
    $mbConfig->packageDirectories([
        __DIR__ . '/packages',
    ]);

    $mbConfig->packageAliasFormat('<major>.<minor>.x-dev');

    // Merge these sections from packages/ to root composer.json
    $mbConfig->dataToAppend([
        ComposerJsonSection::AUTOLOAD_DEV => [
            'psr-4' => [
                'Pogo\\Queue\\Tests\\' => 'packages/laravel/tests/',
                'Pogo\\Queue\\Symfony\\Tests\\' => 'packages/symfony/tests/',
            ],
        ],
    ]);
};