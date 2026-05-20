<?php

declare(strict_types=1);

namespace Pogo\Queue\Symfony\Transport;

use Symfony\Component\Messenger\Exception\TransportException;

class QueueDispatchException extends TransportException {}
