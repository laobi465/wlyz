# KeyAuth PHP SDK

> KeyAuth SaaS 官方 PHP 客户端 SDK —— 多租户卡密验证客户端

[![PHP](https://img.shields.io/badge/PHP-7.2+-777BB4.svg)](https://php.net)
[![Version](https://img.shields.io/badge/version-0.3.6-blue.svg)](../../docs/CHANGELOG.md)

## 特性

- ✅ 9 个验证 API 全封装（login / verify / heartbeat / bind / unbind / get_var / notice / version / logout）
- ✅ HMAC-SHA512/256 签名算法（与后端 `crypto.HMACSHA256` 对齐，PHP 7.1+ 原生支持）
- ✅ 无第三方依赖（仅依赖 PHP cURL / json / hash 扩展，PHP 标配）
- ✅ 完整异常体系（`KeyAuthError` 含业务错误码 + HTTP 状态码）
- ✅ Composer 友好（PSR-4 自动加载）
- ✅ 全类型安全（`declare(strict_types=1)`）

## 环境要求

- PHP >= 7.2（推荐 7.4+ 或 8.x）
- 扩展：`ext-curl` / `ext-json` / `ext-hash`（PHP 标配，通常无需额外安装）

## 安装

### 方式一：Composer（推荐）

```bash
composer require keyauth/keyauth-php
```

或在 `composer.json` 中添加：

```json
{
    "require": {
        "keyauth/keyauth-php": "^0.3.6"
    }
}
```

### 方式二：手动引入

```bash
git clone https://github.com/your-org/keyauth-saas.git
cp -r keyauth-saas/sdks/php /path/to/your-project/keyauth
```

```php
<?php
require_once '/path/to/keyauth/src/KeyAuthError.php';
require_once '/path/to/keyauth/src/KeyAuthClient.php';
```

## 快速开始

```php
<?php
require_once 'vendor/autoload.php';

use KeyAuth\KeyAuthClient;
use KeyAuth\KeyAuthError;

// 1. 初始化客户端（参数从开发者后台获取）
$client = new KeyAuthClient(
    'https://keyauth.example.com',  // 平台 API 地址
    'ak_xxxxxxxxxxxxxxxx',          // 应用 AppKey
    'ss_xxxxxxxxxxxxxxxx',          // 签名密钥
    10                               // HTTP 超时秒数（可选）
);

try {
    // 2. 登录（首次自动绑定设备）
    $result = $client->login(
        'XXXX-XXXX-XXXX-XXXX',
        'hwid_abc123',
        'My Computer',
        'windows'
    );
    echo "登录成功，过期时间: " . date('Y-m-d H:i:s', $result['expires_at']) . "\n";

    // 3. 心跳保活（每隔 heartbeat_interval 秒调用一次）
    $heartbeat = $client->heartbeat('XXXX-XXXX-XXXX-XXXX', 'hwid_abc123');
    echo "下次心跳时间: " . date('Y-m-d H:i:s', $heartbeat['next_heartbeat']) . "\n";

    // 4. 获取云变量
    $var = $client->getVar('XXXX-XXXX-XXXX-XXXX', 'vip_level');
    echo "VIP 等级: " . $var['value'] . "\n";

} catch (KeyAuthError $e) {
    echo "验证失败: [{$e->getErrorCode()}/{$e->getHttpStatus()}] " . $e->getMessage() . "\n";
    // 错误码处理示例
    switch ($e->getErrorCode()) {
        case 2001:
            echo "→ 卡密不存在或已失效\n";
            break;
        case 2002:
            echo "→ 卡密状态异常（过期/封禁/禁用）\n";
            break;
        case 2003:
            echo "→ 设备绑定数已达上限\n";
            break;
        case 2004:
            echo "→ 设备未绑定\n";
            break;
        case 2005:
            echo "→ 超过离线宽限期\n";
            break;
    }
}
```

## 9 个 API 速查表

| 方法 | 路径 | 必填参数 | 说明 |
|---|---|---|---|
| `login()` | `/api/v1/client/login` | `card_key` / `hwid` | 登录并自动绑定设备 |
| `verify()` | `/api/v1/client/verify` | `card_key` / `hwid` | 验证卡密有效性（不增使用次数） |
| `heartbeat()` | `/api/v1/client/heartbeat` | `card_key` / `hwid` | 心跳保活 |
| `bind()` | `/api/v1/client/bind` | `card_key` / `hwid` | 手动绑定设备 |
| `unbind()` | `/api/v1/client/unbind` | `card_key` / `hwid` | 解绑设备（会扣时） |
| `getVar()` | `/api/v1/client/get_var` | `card_key` / `var_key` | 获取云变量 |
| `notice()` | `/api/v1/client/notice` | — | 获取应用公告 |
| `version()` | `/api/v1/client/version` | — | 检查版本更新 |
| `logout()` | `/api/v1/client/logout` | `card_key` / `hwid` | 登出（仅记录日志） |

## 签名算法

SDK 自动完成签名，无需手动计算。如需自定义实现，请严格遵循：

### 1. 签名原文构造

```
METHOD\nPATH\nTIMESTAMP\nNONCE\nBODY
```

5 个字段以 `\n` 拼接：

| 字段 | 说明 | 示例 |
|---|---|---|
| METHOD | HTTP 方法（全大写） | `POST` |
| PATH | URL 路径（含 query） | `/api/v1/client/login` |
| TIMESTAMP | 当前 Unix 时间戳（秒） | `1721457600` |
| NONCE | 32 位随机十六进制字符串 | `a1b2c3...` |
| BODY | 请求体 JSON 字符串 | `{"card_key":"XXX","hwid":"YYY"}` |

### 2. HMAC-SHA512/256 计算

```php
$signature = hash_hmac('sha512/256', $signString, $signSecret);
```

输出 64 位小写十六进制字符串。

> **注意**：后端 `crypto.HMACSHA256` 使用的是 `sha512.New512_256`（SHA-512/256 变体），与标准 SHA-256 算法**不同**。
> PHP 7.1+ 原生支持 `sha512/256` 算法，SDK 会优先使用；如运行时不支持则自动回退到 `sha256`（待核实：与后端 sha512.New512_256 是否完全等价）。

### 3. 请求头

```
Content-Type: application/json
X-App-Key: ak_xxxxxxxxxxxxxxxx
X-Timestamp: 1721457600
X-Nonce: a1b2c3d4e5f6...
X-Signature: <64 位 hex 签名>
```

## 错误处理

SDK 通过 `KeyAuthError` 异常统一传递错误信息：

```php
try {
    $client->verify($cardKey, $hwid);
} catch (KeyAuthError $e) {
    $errorCode   = $e->getErrorCode();    // 业务错误码（如 2001）
    $httpStatus  = $e->getHttpStatus();   // HTTP 状态码（如 401）
    $message     = $e->getMessage();      // 错误消息
    $code        = $e->getCode();         // 同 getErrorCode()
}
```

### 错误码表

| 错误码 | HTTP | 含义 | 触发场景 |
|---|---|---|---|
| 1001 | 400/401 | 参数错误 / 时间戳超出范围 | 请求体格式错误 / 客户端时间偏差 > 5 分钟 |
| 1002 | 401 | 签名参数缺失 / 签名校验失败 | 请求头缺失 / 签名密钥错误 |
| 1006 | 500 | 服务器内部错误 | 加密管理器未初始化 / 解密失败 |
| 2001 | 401 | 卡密不存在或已失效 | 卡密错误 / 卡密不属于该应用 |
| 2002 | 403 | 卡密状态异常 | 已过期 / 已封禁 / 已禁用 / 使用次数用尽 |
| 2003 | 403 | 设备绑定数已达上限 | 超过应用 `max_devices` 限制 |
| 2004 | 403 | 设备未绑定 | 调用 verify / heartbeat 时设备未绑定 |
| 2005 | 403 | 超过离线宽限期 | 心跳超时后调用 verify |
| 3001 | 401 | 应用不存在或已禁用 | AppKey 错误 / 应用被禁用 |
| 5001 | 500 | 查询卡密失败 | 数据库错误 |
| 5002 | 500 | 登录事务失败 | 事务回滚 |

## 配置参考

### 获取 AppKey 和 SignSecret

1. 登录 KeyAuth SaaS 平台开发者后台
2. 进入「应用管理」→ 选择应用 → 「密钥管理」
3. 复制 `AppKey`（`ak_` 开头）和 `SignSecret`（`ss_` 开头）

> ⚠️ **SignSecret 是机密信息**，请勿硬编码到客户端代码或公开仓库中。建议从环境变量读取：

```php
$client = new KeyAuthClient(
    getenv('KEYAUTH_API_BASE'),
    getenv('KEYAUTH_APP_KEY'),
    getenv('KEYAUTH_SIGN_SECRET')
);
```

### 获取 HWID（设备指纹）

PHP 端常见做法（Linux 服务器场景）：

```php
function generateHwid(): string {
    $cpuInfo = file_get_contents('/proc/cpuinfo');
    $macAddress = exec('cat /sys/class/net/eth0/address');
    $diskSN = exec('lsblk -d -n -o serial /dev/sda');
    return hash('sha512', $cpuInfo . '|' . $macAddress . '|' . $diskSN);
}
```

## 完整示例

参见 [`examples/demo.php`](examples/demo.php)（待补充）。

## 版本兼容性

| SDK 版本 | 后端版本 | 兼容性 |
|---|---|---|
| 0.3.6 | v0.3.6+ | ✅ 完全兼容 |
| < 0.3.6 | — | ❌ 不维护 |

## License

Proprietary - 见项目根目录 [LICENSE](../../LICENSE)
