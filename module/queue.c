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
    "1.0.0",
    STANDARD_MODULE_PROPERTIES
};

PHP_FUNCTION(pogo_queue) {
    zval *data;
    zend_string *str;

    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_ZVAL(data)
    ZEND_PARSE_PARAMETERS_END();

    str = zval_get_string(data);

    zend_long ret = pogo_dispatch(ZSTR_VAL(str), ZSTR_LEN(str));

    zend_string_release(str);

    RETURN_LONG(ret);
}

PHP_FUNCTION(pogo_queue_status)
{
    char *stats = pogo_queue_status();
    if (stats == NULL) {
        RETURN_EMPTY_STRING();
    }

    size_t stats_len = strlen(stats);
    zend_string *statsString = zend_string_init(stats, stats_len, 0);
    free(stats);
    RETURN_STR(statsString);
}
