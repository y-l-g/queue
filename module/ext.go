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

//export pogo_queue_push
func pogo_queue_push(queue *C.char, queueLength C.size_t, payload *C.char, payloadLength C.size_t, delaySeconds C.longlong) *C.char {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return jsonCString(pushResult{OK: false, Code: dispatchResultPayloadTooLarge, Message: "queue name is too large"})
	}
	message, ok := goStringFromC(payload, payloadLength)
	if !ok {
		return jsonCString(pushResult{OK: false, Code: dispatchResultPayloadTooLarge, Message: "payload is too large"})
	}
	return enqueue(queueName, message, int64(delaySeconds))
}

//export pogo_queue_ack
func pogo_queue_ack(queue *C.char, queueLength C.size_t, delivery *C.char, deliveryLength C.size_t) C.int {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return dispatchResultPayloadTooLarge
	}
	deliveryID, ok := goStringFromC(delivery, deliveryLength)
	if !ok {
		return dispatchResultPayloadTooLarge
	}
	return ack(queueName, deliveryID)
}

//export pogo_queue_release
func pogo_queue_release(queue *C.char, queueLength C.size_t, delivery *C.char, deliveryLength C.size_t, delaySeconds C.longlong) C.int {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return dispatchResultPayloadTooLarge
	}
	deliveryID, ok := goStringFromC(delivery, deliveryLength)
	if !ok {
		return dispatchResultPayloadTooLarge
	}
	return release(queueName, deliveryID, int64(delaySeconds))
}

//export pogo_queue_fail
func pogo_queue_fail(queue *C.char, queueLength C.size_t, delivery *C.char, deliveryLength C.size_t, reason *C.char, reasonLength C.size_t) C.int {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return dispatchResultPayloadTooLarge
	}
	deliveryID, ok := goStringFromC(delivery, deliveryLength)
	if !ok {
		return dispatchResultPayloadTooLarge
	}
	reasonValue, ok := goStringFromC(reason, reasonLength)
	if !ok {
		return dispatchResultPayloadTooLarge
	}
	return failDelivery(queueName, deliveryID, reasonValue)
}

//export pogo_queue_status
func pogo_queue_status(queue *C.char, queueLength C.size_t) *C.char {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return jsonCString(statusPayload{Ready: false, Queues: []queueStats{}})
	}
	return queueStatsJSON(queueName)
}
