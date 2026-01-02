#include <php.h>
#include <Zend/zend_exceptions.h>
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

    int ret = pogo_dispatch(ZSTR_VAL(str), ZSTR_LEN(str));

    zend_string_release(str);

    RETURN_BOOL(ret);
}