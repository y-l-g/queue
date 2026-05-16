/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: <regenerate-from-stub> */

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue, 0, 1, IS_LONG, 0)
ZEND_ARG_TYPE_INFO(0, data, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_status, 0, 0, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_FUNCTION(pogo_queue);
ZEND_FUNCTION(pogo_queue_status);

static const zend_function_entry ext_functions[] = {
    ZEND_FE(pogo_queue, arginfo_pogo_queue)
    ZEND_FE(pogo_queue_status, arginfo_pogo_queue_status)
    ZEND_FE_END};
