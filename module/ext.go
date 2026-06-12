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

//export pogo_queue_failed
func pogo_queue_failed(queue *C.char, queueLength C.size_t, limit C.longlong) *C.char {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return jsonCString(failedJobsPayload{OK: false, Failed: []failedJob{}, Code: dispatchResultPayloadTooLarge, Message: "queue name is too large"})
	}
	return failedJobsJSON(queueName, int64(limit))
}

//export pogo_queue_retry_failed
func pogo_queue_retry_failed(queue *C.char, queueLength C.size_t, failed *C.char, failedLength C.size_t) *C.char {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return jsonCString(failedOperationResult{OK: false, Code: dispatchResultPayloadTooLarge, Message: "queue name is too large"})
	}
	failedID, ok := goStringFromC(failed, failedLength)
	if !ok {
		return jsonCString(failedOperationResult{OK: false, Code: dispatchResultPayloadTooLarge, Message: "failed id is too large"})
	}
	return retryFailedJSON(queueName, failedID)
}

//export pogo_queue_forget_failed
func pogo_queue_forget_failed(queue *C.char, queueLength C.size_t, failed *C.char, failedLength C.size_t) *C.char {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return jsonCString(failedOperationResult{OK: false, Code: dispatchResultPayloadTooLarge, Message: "queue name is too large"})
	}
	failedID, ok := goStringFromC(failed, failedLength)
	if !ok {
		return jsonCString(failedOperationResult{OK: false, Code: dispatchResultPayloadTooLarge, Message: "failed id is too large"})
	}
	return forgetFailedJSON(queueName, failedID)
}

//export pogo_queue_purge_failed
func pogo_queue_purge_failed(queue *C.char, queueLength C.size_t) *C.char {
	queueName, ok := goStringFromC(queue, queueLength)
	if !ok {
		return jsonCString(failedOperationResult{OK: false, Code: dispatchResultPayloadTooLarge, Message: "queue name is too large"})
	}
	return purgeFailedJSON(queueName)
}
