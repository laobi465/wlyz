<?php
/**
 * KeyAuth SaaS PHP SDK
 *
 * 主客户端类 - 封装 9 个验证 API
 *
 * 签名算法：HMAC-SHA512/256（与后端 crypto.HMACSHA256 对齐）
 * 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
 *
 * @author  KeyAuth SaaS Team
 * @version 0.3.6
 */

declare(strict_types=1);

namespace KeyAuth;

/**
 * Class KeyAuthClient
 *
 * @package KeyAuth
 */
class KeyAuthClient
{
    /** @var string 平台 API 地址（如 https://keyauth.example.com） */
    private $apiBase;

    /** @var string 应用 AppKey（开发者后台获取） */
    private $appKey;

    /** @var string HMAC 签名密钥（开发者后台获取，AES 加密入库） */
    private $signSecret;

    /** @var int HTTP 超时（秒） */
    private $timeout;

    /** @var string User-Agent */
    private $userAgent = 'KeyAuth-PHP-SDK/0.3.6';

    /** @var string 客户端 API 路径前缀 */
    private const API_PREFIX = '/api/v1/client';

    /**
     * 构造函数
     *
     * @param string $apiBase    平台 API 地址
     * @param string $appKey     应用 AppKey
     * @param string $signSecret HMAC 签名密钥
     * @param int    $timeout    HTTP 超时秒数（默认 10）
     * @throws KeyAuthError 配置错误时抛出
     */
    public function __construct(string $apiBase, string $appKey, string $signSecret, int $timeout = 10)
    {
        $apiBase = rtrim($apiBase, '/');
        if ($apiBase === '') {
            throw new KeyAuthError('apiBase 不能为空', 1001, 0);
        }
        if ($appKey === '') {
            throw new KeyAuthError('appKey 不能为空', 1001, 0);
        }
        if ($signSecret === '') {
            throw new KeyAuthError('signSecret 不能为空', 1001, 0);
        }
        if (!extension_loaded('curl')) {
            throw new KeyAuthError('PHP cURL 扩展未安装', 1006, 0);
        }

        $this->apiBase = $apiBase;
        $this->appKey = $appKey;
        $this->signSecret = $signSecret;
        $this->timeout = $timeout;
    }

    // ============== 9 个验证 API ==============

    /**
     * 1. 登录（首次自动绑定设备）
     *
     * @param string $cardKey    卡密
     * @param string $hwid       设备指纹
     * @param string $deviceName 设备名称（可选）
     * @param string $deviceType 设备类型（可选：windows/macos/linux/android/ios/web）
     * @return array 返回数据（token / expires_at / card / device / heartbeat_*）
     * @throws KeyAuthError
     */
    public function login(
        string $cardKey,
        string $hwid,
        string $deviceName = '',
        string $deviceType = ''
    ): array {
        $payload = [
            'card_key' => $cardKey,
            'hwid' => $hwid,
        ];
        if ($deviceName !== '') {
            $payload['device_name'] = $deviceName;
        }
        if ($deviceType !== '') {
            $payload['device_type'] = $deviceType;
        }
        return $this->post('/login', $payload);
    }

    /**
     * 2. 验证卡密有效性（不绑定设备，不增加使用次数）
     *
     * @param string $cardKey 卡密
     * @param string $hwid    设备指纹
     * @return array
     * @throws KeyAuthError
     */
    public function verify(string $cardKey, string $hwid): array
    {
        return $this->post('/verify', [
            'card_key' => $cardKey,
            'hwid' => $hwid,
        ]);
    }

    /**
     * 3. 心跳保活（每隔 heartbeat_interval 秒调用一次）
     *
     * @param string $cardKey 卡密
     * @param string $hwid    设备指纹
     * @return array
     * @throws KeyAuthError
     */
    public function heartbeat(string $cardKey, string $hwid): array
    {
        return $this->post('/heartbeat', [
            'card_key' => $cardKey,
            'hwid' => $hwid,
        ]);
    }

    /**
     * 4. 手动绑定设备（多机场景）
     *
     * @param string $cardKey    卡密
     * @param string $hwid       设备指纹
     * @param string $deviceName 设备名称（可选）
     * @param string $deviceType 设备类型（可选）
     * @return array
     * @throws KeyAuthError
     */
    public function bind(
        string $cardKey,
        string $hwid,
        string $deviceName = '',
        string $deviceType = ''
    ): array {
        $payload = [
            'card_key' => $cardKey,
            'hwid' => $hwid,
        ];
        if ($deviceName !== '') {
            $payload['device_name'] = $deviceName;
        }
        if ($deviceType !== '') {
            $payload['device_type'] = $deviceType;
        }
        return $this->post('/bind', $payload);
    }

    /**
     * 5. 解绑设备（会扣时，谨慎使用）
     *
     * @param string $cardKey 卡密
     * @param string $hwid    设备指纹
     * @return array
     * @throws KeyAuthError
     */
    public function unbind(string $cardKey, string $hwid): array
    {
        return $this->post('/unbind', [
            'card_key' => $cardKey,
            'hwid' => $hwid,
        ]);
    }

    /**
     * 6. 获取云变量
     *
     * @param string $cardKey 卡密
     * @param string $varKey  变量键
     * @return array
     * @throws KeyAuthError
     */
    public function getVar(string $cardKey, string $varKey): array
    {
        return $this->post('/get_var', [
            'card_key' => $cardKey,
            'var_key' => $varKey,
        ]);
    }

    /**
     * 7. 获取应用公告
     *
     * @return array
     * @throws KeyAuthError
     */
    public function notice(): array
    {
        return $this->post('/notice', []);
    }

    /**
     * 8. 检查版本更新
     *
     * @return array
     * @throws KeyAuthError
     */
    public function version(): array
    {
        return $this->post('/version', []);
    }

    /**
     * 9. 登出（仅记录日志，不真正销毁会话）
     *
     * @param string $cardKey 卡密
     * @param string $hwid    设备指纹
     * @return array
     * @throws KeyAuthError
     */
    public function logout(string $cardKey, string $hwid): array
    {
        return $this->post('/logout', [
            'card_key' => $cardKey,
            'hwid' => $hwid,
        ]);
    }

    // ============== 内部方法 ==============

    /**
     * 发送 POST 请求并解析响应
     *
     * @param string $path    路径（不含前缀，如 /login）
     * @param array  $payload 请求体
     * @return array 响应 data 字段
     * @throws KeyAuthError
     */
    private function post(string $path, array $payload): array
    {
        $fullPath = self::API_PREFIX . $path;
        $url = $this->apiBase . $fullPath;
        $body = empty($payload) ? '{}' : json_encode($payload, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES);
        if ($body === false) {
            throw new KeyAuthError('JSON 编码失败: ' . json_last_error_msg(), 1001, 0);
        }

        $timestamp = (string) time();
        $nonce = $this->generateNonce();
        $signature = $this->sign('POST', $fullPath, $timestamp, $nonce, $body);

        $headers = [
            'Content-Type: application/json',
            'X-App-Key: ' . $this->appKey,
            'X-Timestamp: ' . $timestamp,
            'X-Nonce: ' . $nonce,
            'X-Signature: ' . $signature,
            'User-Agent: ' . $this->userAgent,
        ];

        $ch = curl_init($url);
        curl_setopt_array($ch, [
            CURLOPT_POST => true,
            CURLOPT_POSTFIELDS => $body,
            CURLOPT_HTTPHEADER => $headers,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_TIMEOUT => $this->timeout,
            CURLOPT_CONNECTTIMEOUT => min(5, $this->timeout),
            CURLOPT_SSL_VERIFYPEER => true,
            CURLOPT_SSL_VERIFYHOST => 2,
            CURLOPT_HEADER => false,
        ]);

        $response = curl_exec($ch);
        $httpCode = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
        $errno = curl_errno($ch);
        $errmsg = curl_error($ch);
        curl_close($ch);

        if ($errno !== 0) {
            throw new KeyAuthError(
                'HTTP 请求失败: ' . $errmsg,
                1006,
                0
            );
        }

        $data = json_decode($response, true);
        if (!is_array($data)) {
            throw new KeyAuthError(
                '响应解析失败: 非 JSON 格式',
                1006,
                $httpCode
            );
        }

        $code = $data['code'] ?? 0;
        $message = $data['message'] ?? '';
        $success = $data['success'] ?? false;

        if (!$success || $httpCode >= 400) {
            throw new KeyAuthError(
                $message !== '' ? $message : '请求失败',
                (int) $code,
                $httpCode
            );
        }

        $responseData = $data['data'] ?? [];
        return is_array($responseData) ? $responseData : [];
    }

    /**
     * 计算签名
     *
     * @param string $method   HTTP 方法（POST）
     * @param string $path     完整路径（含前缀）
     * @param string $timestamp 时间戳
     * @param string $nonce    随机串
     * @param string $body     请求体
     * @return string 签名 hex
     */
    private function sign(
        string $method,
        string $path,
        string $timestamp,
        string $nonce,
        string $body
    ): string {
        $signString = implode("\n", [$method, $path, $timestamp, $nonce, $body]);
        return $this->hmacSha512256($this->signSecret, $signString);
    }

    /**
     * HMAC-SHA512/256 签名（与后端 crypto.HMACSHA256 对齐）
     * PHP 7.1+ 原生支持 sha512/256 算法，不支持时回退 sha256
     *
     * @param string $secret 密钥
     * @param string $msg    消息
     * @return string 64 位小写 hex
     */
    private function hmacSha512256(string $secret, string $msg): string
    {
        if (in_array('sha512/256', hash_hmac_algos(), true)) {
            return hash_hmac('sha512/256', $msg, $secret);
        }
        // 回退：标准 SHA-256（待核实：是否与后端 sha512.New512_256 完全等价）
        return hash_hmac('sha256', $msg, $secret);
    }

    /**
     * 生成 16 字节随机 nonce（32 位 hex）
     *
     * @return string
     * @throws \Exception
     */
    private function generateNonce(): string
    {
        return bin2hex(random_bytes(16));
    }

    // ============== Setter（可选） ==============

    /**
     * 设置 User-Agent
     *
     * @param string $ua
     * @return $this
     */
    public function setUserAgent(string $ua): self
    {
        $this->userAgent = $ua;
        return $this;
    }

    /**
     * 设置超时
     *
     * @param int $seconds
     * @return $this
     */
    public function setTimeout(int $seconds): self
    {
        $this->timeout = $seconds > 0 ? $seconds : 10;
        return $this;
    }
}
