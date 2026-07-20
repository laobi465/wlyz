# KeyAuth SaaS PHP SDK

> [中文](README.md) | **English**

Client-side SDK for terminal software, wrapping 9 KeyAuth SaaS verification APIs.

## Installation

### Option 1: Composer (recommended)

```bash
composer require keyauth/keyauth-php
```

Or add to `composer.json`:

```json
{
    "require": {
        "keyauth/keyauth-php": "^0.3.6"
    }
}
```

### Option 2: Manual include

```bash
git clone https://github.com/your-org/keyauth-saas.git
cp -r keyauth-saas/sdks/php /path/to/your-project/keyauth
```

```php
<?php
require_once '/path/to/keyauth/src/KeyAuthError.php';
require_once '/path/to/keyauth/src/KeyAuthClient.php';
```

## Quick Start

```php
<?php
require_once 'vendor/autoload.php';

use KeyAuth\KeyAuthClient;
use KeyAuth\KeyAuthError;

// 1. Initialize the client (params from developer console)
$client = new KeyAuthClient(
    'https://keyauth.example.com',  // Platform API base URL
    'ak_xxxxxxxxxxxxxxxx',          // Application AppKey
    'ss_xxxxxxxxxxxxxxxx',          // SignSecret
    10                               // HTTP timeout in seconds (optional)
);

try {
    // 2. Login (auto-binds device on first call)
    $result = $client->login(
        'XXXX-XXXX-XXXX-XXXX',
        'hwid_abc123',
        'My Computer',
        'windows'
    );
    echo "Login successful, expires at: " . date('Y-m-d H:i:s', $result['expires_at']) . "\n";

    // 3. Heartbeat keep-alive (call every heartbeat_interval seconds)
    $heartbeat = $client->heartbeat('XXXX-XXXX-XXXX-XXXX', 'hwid_abc123');
    echo "Next heartbeat time: " . date('Y-m-d H:i:s', $heartbeat['next_heartbeat']) . "\n";

    // 4. Get cloud variable
    $var = $client->getVar('XXXX-XXXX-XXXX-XXXX', 'vip_level');
    echo "VIP level: " . $var['value'] . "\n";

} catch (KeyAuthError $e) {
    echo "Verification failed: [{$e->getErrorCode()}/{$e->getHttpStatus()}] " . $e->getMessage() . "\n";
    // Error code handling example
    switch ($e->getErrorCode()) {
        case 2001:
            echo "→ Card not found or invalid\n";
            break;
        case 2002:
            echo "→ Card status abnormal (expired/banned/disabled)\n";
            break;
        case 2003:
            echo "→ Device binding limit reached\n";
            break;
        case 2004:
            echo "→ Device not bound\n";
            break;
        case 2005:
            echo "→ Offline grace period exceeded\n";
            break;
    }
}
```

## 9 API Reference

| Method | Purpose | Route |
|---|---|---|
| `$client->login()` | Login (auto-binds device on first call) | POST /api/v1/client/login |
| `$client->verify()` | Verify card (no binding) | POST /api/v1/client/verify |
| `$client->heartbeat()` | Heartbeat keep-alive | POST /api/v1/client/heartbeat |
| `$client->bind()` | Manually bind device | POST /api/v1/client/bind |
| `$client->unbind()` | Unbind device (deducts time) | POST /api/v1/client/unbind |
| `$client->getVar()` | Get cloud variable | POST /api/v1/client/get_var |
| `$client->notice()` | Get app notice | POST /api/v1/client/notice |
| `$client->version()` | Check version update | POST /api/v1/client/version |
| `$client->logout()` | Logout | POST /api/v1/client/logout |

> Method names follow each SDK's naming convention (e.g. Python `client.login`, Go `client.Login`, C# `client.LoginAsync`, 易语言 `登录`).

## Signature Algorithm

Aligned with backend `internal/middleware/signature.go`:

```
payload = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
signature = HMAC-SHA512/256(sign_secret, payload) → 64-char lowercase hex
```

Request headers:

```
X-App-Key:    ak_xxx
X-Timestamp:  1767225600
X-Nonce:      32-char UUID/hex
X-Signature:  64-char hex signature
```

> **Compatibility note**: Backend uses the SHA-512/256 variant (Go `sha512.New512_256`), which is **different** from standard SHA-256. PHP 7.1+ natively supports `sha512/256` and the SDK prefers it; if the runtime lacks support it automatically falls back to `sha256` (pending verification: whether this is fully equivalent to backend `sha512.New512_256`).

## Error Handling

```php
try {
    $client->verify($cardKey, $hwid);
} catch (KeyAuthError $e) {
    $errorCode   = $e->getErrorCode();    // Business error code (e.g. 2001)
    $httpStatus  = $e->getHttpStatus();   // HTTP status code (e.g. 401)
    $message     = $e->getMessage();      // Error message
    $code        = $e->getCode();         // Same as getErrorCode()
}
```

### Error Codes

| Code | Meaning |
|---|---|
| 2001 | Card not found or invalid |
| 2002 | Card expired / banned / disabled / quota exhausted |
| 2003 | Device limit exceeded |
| 2004 | Device not bound |
| 2005 | Offline grace period exceeded |
| 2006 | Device already bound to this card |
| 2007 | Device not bound |
| 2008 | Cloud variable not found |
| 1001 | Invalid params / timestamp out of range / nonce replay |
| 1002 | Signature verification failed / signature params missing |
| 1006 | Network error / non-JSON response |
| 3001 | App not found or disabled |

## Compliance

- **Rule 04 (No hardcoding)**: API base / AppKey / SignSecret are all passed by caller, never hardcoded
- **Rule 06 (Anti-hallucination)**: All errors throw `KeyAuthError(code, message)`; signature algorithm strictly aligns with backend

## License

Proprietary — Distributed with the main KeyAuth SaaS project.
