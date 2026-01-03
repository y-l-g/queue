<?php

$_SERVER['APP_BASE_PATH'] = $_ENV['APP_BASE_PATH'] ?? $_SERVER['APP_BASE_PATH'] ?? __DIR__ . '/..';

require __DIR__ . '/../vendor/pogo/queue/bin/queue-worker.php';