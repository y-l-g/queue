package queue

// #include <stdlib.h>
// #include "queue.h"
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

func init() {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.queue_module_entry))
}

//export pogo_queue
func pogo_queue(data *C.zval) {
	queue(data)
}
