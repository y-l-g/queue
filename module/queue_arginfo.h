/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: <regenerate-from-stub> */

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue, 0, 1, IS_LONG, 0)
ZEND_ARG_TYPE_INFO(0, data, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_push, 0, 2, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, queue, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, payload, IS_STRING, 0)
ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, delaySeconds, IS_LONG, 0, "0")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_ack, 0, 2, IS_LONG, 0)
ZEND_ARG_TYPE_INFO(0, queue, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, deliveryId, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_release, 0, 2, IS_LONG, 0)
ZEND_ARG_TYPE_INFO(0, queue, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, deliveryId, IS_STRING, 0)
ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, delaySeconds, IS_LONG, 0, "0")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_fail, 0, 2, IS_LONG, 0)
ZEND_ARG_TYPE_INFO(0, queue, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, deliveryId, IS_STRING, 0)
ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, reason, IS_STRING, 0, "''")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_status, 0, 0, IS_STRING, 0)
ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, queue, IS_STRING, 1, "null")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_failed, 0, 1, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, queue, IS_STRING, 0)
ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, limit, IS_LONG, 0, "100")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_retry_failed, 0, 2, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, queue, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, failedId, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_forget_failed, 0, 2, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, queue, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, failedId, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_pogo_queue_purge_failed, 0, 1, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, queue, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_FUNCTION(pogo_queue);
ZEND_FUNCTION(pogo_queue_push);
ZEND_FUNCTION(pogo_queue_ack);
ZEND_FUNCTION(pogo_queue_release);
ZEND_FUNCTION(pogo_queue_fail);
ZEND_FUNCTION(pogo_queue_status);
ZEND_FUNCTION(pogo_queue_failed);
ZEND_FUNCTION(pogo_queue_retry_failed);
ZEND_FUNCTION(pogo_queue_forget_failed);
ZEND_FUNCTION(pogo_queue_purge_failed);

static const zend_function_entry ext_functions[] = {
    ZEND_FE(pogo_queue, arginfo_pogo_queue)
    ZEND_FE(pogo_queue_push, arginfo_pogo_queue_push)
    ZEND_FE(pogo_queue_ack, arginfo_pogo_queue_ack)
    ZEND_FE(pogo_queue_release, arginfo_pogo_queue_release)
    ZEND_FE(pogo_queue_fail, arginfo_pogo_queue_fail)
    ZEND_FE(pogo_queue_status, arginfo_pogo_queue_status)
    ZEND_FE(pogo_queue_failed, arginfo_pogo_queue_failed)
    ZEND_FE(pogo_queue_retry_failed, arginfo_pogo_queue_retry_failed)
    ZEND_FE(pogo_queue_forget_failed, arginfo_pogo_queue_forget_failed)
    ZEND_FE(pogo_queue_purge_failed, arginfo_pogo_queue_purge_failed)
    ZEND_FE_END};
