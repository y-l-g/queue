#include <php.h>
#include <Zend/zend_exceptions.h>
#include <stdlib.h>
#include <string.h>
#include "_cgo_export.h"
#include "queue.h"
#include "queue_arginfo.h"

PHP_MINIT_FUNCTION(queue) { return SUCCESS; }

zend_module_entry queue_module_entry = {
    STANDARD_MODULE_HEADER,
    "queue",
    ext_functions,
    PHP_MINIT(queue),
    NULL,
    NULL,
    NULL,
    NULL,
    "2.0.0",
    STANDARD_MODULE_PROPERTIES
};

static void return_owned_string(zval *return_value, char *value)
{
    if (value == NULL) {
        RETVAL_EMPTY_STRING();
        return;
    }

    size_t value_len = strlen(value);
    zend_string *result = zend_string_init(value, value_len, 0);
    free(value);
    RETVAL_STR(result);
}

PHP_FUNCTION(pogo_queue) {
    char *data;
    size_t data_len;

    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_STRING(data, data_len)
    ZEND_PARSE_PARAMETERS_END();

    zend_long ret = pogo_dispatch(data, data_len);

    RETURN_LONG(ret);
}

PHP_FUNCTION(pogo_queue_push) {
    char *queue;
    size_t queue_len;
    char *payload;
    size_t payload_len;
    zend_long delay = 0;

    ZEND_PARSE_PARAMETERS_START(2, 3)
        Z_PARAM_STRING(queue, queue_len)
        Z_PARAM_STRING(payload, payload_len)
        Z_PARAM_OPTIONAL
        Z_PARAM_LONG(delay)
    ZEND_PARSE_PARAMETERS_END();

    if (delay < 0) {
        zend_throw_exception(zend_ce_value_error, "Delay must be greater than or equal to zero.", 0);
        RETURN_THROWS();
    }

    char *result = pogo_queue_push(queue, queue_len, payload, payload_len, delay);
    return_owned_string(return_value, result);
}

PHP_FUNCTION(pogo_queue_ack) {
    char *queue;
    size_t queue_len;
    char *delivery;
    size_t delivery_len;

    ZEND_PARSE_PARAMETERS_START(2, 2)
        Z_PARAM_STRING(queue, queue_len)
        Z_PARAM_STRING(delivery, delivery_len)
    ZEND_PARSE_PARAMETERS_END();

    RETURN_LONG(pogo_queue_ack(queue, queue_len, delivery, delivery_len));
}

PHP_FUNCTION(pogo_queue_release) {
    char *queue;
    size_t queue_len;
    char *delivery;
    size_t delivery_len;
    zend_long delay = 0;

    ZEND_PARSE_PARAMETERS_START(2, 3)
        Z_PARAM_STRING(queue, queue_len)
        Z_PARAM_STRING(delivery, delivery_len)
        Z_PARAM_OPTIONAL
        Z_PARAM_LONG(delay)
    ZEND_PARSE_PARAMETERS_END();

    if (delay < 0) {
        zend_throw_exception(zend_ce_value_error, "Delay must be greater than or equal to zero.", 0);
        RETURN_THROWS();
    }

    RETURN_LONG(pogo_queue_release(queue, queue_len, delivery, delivery_len, delay));
}

PHP_FUNCTION(pogo_queue_fail) {
    char *queue;
    size_t queue_len;
    char *delivery;
    size_t delivery_len;
    char *reason = "";
    size_t reason_len = 0;

    ZEND_PARSE_PARAMETERS_START(2, 3)
        Z_PARAM_STRING(queue, queue_len)
        Z_PARAM_STRING(delivery, delivery_len)
        Z_PARAM_OPTIONAL
        Z_PARAM_STRING(reason, reason_len)
    ZEND_PARSE_PARAMETERS_END();

    RETURN_LONG(pogo_queue_fail(queue, queue_len, delivery, delivery_len, reason, reason_len));
}

PHP_FUNCTION(pogo_queue_status)
{
    char *queue = NULL;
    size_t queue_len = 0;

    ZEND_PARSE_PARAMETERS_START(0, 1)
        Z_PARAM_OPTIONAL
        Z_PARAM_STRING_OR_NULL(queue, queue_len)
    ZEND_PARSE_PARAMETERS_END();

    char *stats = pogo_queue_status(queue, queue_len);
    return_owned_string(return_value, stats);
}

PHP_FUNCTION(pogo_queue_failed) {
    char *queue;
    size_t queue_len;
    zend_long limit = 100;

    ZEND_PARSE_PARAMETERS_START(1, 2)
        Z_PARAM_STRING(queue, queue_len)
        Z_PARAM_OPTIONAL
        Z_PARAM_LONG(limit)
    ZEND_PARSE_PARAMETERS_END();

    if (limit < 1 || limit > 1000) {
        zend_throw_exception(zend_ce_value_error, "Limit must be between 1 and 1000.", 0);
        RETURN_THROWS();
    }

    char *result = pogo_queue_failed(queue, queue_len, limit);
    return_owned_string(return_value, result);
}

PHP_FUNCTION(pogo_queue_retry_failed) {
    char *queue;
    size_t queue_len;
    char *failed;
    size_t failed_len;

    ZEND_PARSE_PARAMETERS_START(2, 2)
        Z_PARAM_STRING(queue, queue_len)
        Z_PARAM_STRING(failed, failed_len)
    ZEND_PARSE_PARAMETERS_END();

    char *result = pogo_queue_retry_failed(queue, queue_len, failed, failed_len);
    return_owned_string(return_value, result);
}

PHP_FUNCTION(pogo_queue_forget_failed) {
    char *queue;
    size_t queue_len;
    char *failed;
    size_t failed_len;

    ZEND_PARSE_PARAMETERS_START(2, 2)
        Z_PARAM_STRING(queue, queue_len)
        Z_PARAM_STRING(failed, failed_len)
    ZEND_PARSE_PARAMETERS_END();

    char *result = pogo_queue_forget_failed(queue, queue_len, failed, failed_len);
    return_owned_string(return_value, result);
}

PHP_FUNCTION(pogo_queue_purge_failed) {
    char *queue;
    size_t queue_len;

    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_STRING(queue, queue_len)
    ZEND_PARSE_PARAMETERS_END();

    char *result = pogo_queue_purge_failed(queue, queue_len);
    return_owned_string(return_value, result);
}
