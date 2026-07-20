<?php
/**
 * KeyAuth SaaS PHP SDK
 *
 * 异常类 - 所有 SDK 抛出的错误都使用此异常
 *
 * @author  KeyAuth SaaS Team
 * @version 0.3.6
 */

declare(strict_types=1);

namespace KeyAuth;

/**
 * Class KeyAuthError
 *
 * @package KeyAuth
 */
class KeyAuthError extends \Exception
{
    /** @var int 业务错误码（如 2001/2002/2003） */
    private $errorCode;

    /** @var int HTTP 状态码（如 401/403/500） */
    private $httpStatus;

    /**
     * KeyAuthError constructor.
     *
     * @param string $message    错误消息
     * @param int    $errorCode  业务错误码
     * @param int    $httpStatus HTTP 状态码
     * @param \Throwable|null $previous 上一个异常
     */
    public function __construct(
        string $message = '',
        int $errorCode = 0,
        int $httpStatus = 0,
        \Throwable $previous = null
    ) {
        parent::__construct($message, $errorCode, $previous);
        $this->errorCode = $errorCode;
        $this->httpStatus = $httpStatus;
    }

    /**
     * 获取业务错误码
     *
     * @return int
     */
    public function getErrorCode(): int
    {
        return $this->errorCode;
    }

    /**
     * 获取 HTTP 状态码
     *
     * @return int
     */
    public function getHttpStatus(): int
    {
        return $this->httpStatus;
    }

    /**
     * 字符串表示
     *
     * @return string
     */
    public function __toString(): string
    {
        return __CLASS__ . ": [{$this->errorCode}/{$this->httpStatus}]: {$this->message}\n";
    }
}
