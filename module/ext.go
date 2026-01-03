package queue

/*
#include <stdlib.h>
#include "queue.h"
*/
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

func init() {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.queue_module_entry))
}

//export pogo_dispatch
func pogo_dispatch(msg *C.char, length C.size_t) C.int {
	return dispatch(msg, length)
}
