<?php

$finder = (new PhpCsFixer\Finder())
    ->in([
        __DIR__ . '/packages/laravel/src',
        __DIR__ . '/packages/laravel/tests',
        __DIR__ . '/packages/symfony/src',
        __DIR__ . '/packages/symfony/tests',
    ])
;

return (new PhpCsFixer\Config())
    ->setRules([
        '@PER-CS' => true,
        '@PHP82Migration' => true,
    ])
    ->setFinder($finder)
;