package queue

// #include <stdlib.h>
// #include <string.h>
// #include <Zend/zend.h>
//
// char* extract_zval_string(zval* val, size_t* len) {
//     if (Z_TYPE_P(val) != IS_STRING) return NULL;
//     *len = Z_STRLEN_P(val);
//     return Z_STRVAL_P(val);
// }
import "C"
import (
	"context"
	"log/slog"
)

// export_php:function pogo_queue(mixed $data): void
func queue(data *C.zval) {
	var length C.size_t
	charPtr := C.extract_zval_string(data, &length)

	if charPtr == nil {
		if logger != nil {
			logger.Error("pogo_queue: data must be a string")
		}
		return
	}

	msg := C.GoStringN(charPtr, C.int(length))

	workerMu.Lock()
	w := worker
	l := logger
	workerMu.Unlock()

	if w == nil {
		if l != nil {
			l.Error("pogo_queue: worker pool not initialized. Check your Caddyfile 'pogo_queue' configuration.")
		} else {
			println("pogo_queue: worker pool not initialized")
		}
		return
	}

	go func(payload string) {
		_, err := w.SendMessage(context.Background(), payload, nil)

		if err != nil && l != nil {
			l.Error("pogo_queue: failed to send message", slog.Any("error", err))
		}
	}(msg)
}
