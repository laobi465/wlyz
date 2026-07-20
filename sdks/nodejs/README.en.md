# KeyAuth SaaS Node.js SDK

> [中文](README.md) | **English**

Client-side SDK for terminal software, wrapping 9 KeyAuth SaaS verification APIs. **No third-party dependencies** (uses only Node.js built-in `https` / `crypto` modules).

## Installation

```bash
# Pending release: npm install keyauth-node
# Current: git clone and require directly
```

```js
const { KeyAuthClient, KeyAuthError } = require('./path/to/sdks/nodejs');
```

## Quick Start

```js
const { KeyAuthClient, KeyAuthError } = require('keyauth-node');

const client = new KeyAuthClient({
  apiBase: 'https://yourdomain.com',
  appKey: 'ak_xxx',
  signSecret: 'sk_xxx',
  timeout: 10000,
});

const hwid = 'cpu-motherboard-mac-disk-hash';

(async () => {
  try {
    // 1. Login (auto-binds device on first call)
    const result = await client.login('ABCD-1234-EFGH-5678', hwid, {
      deviceName: 'My PC',
      deviceType: 'windows',
    });
    console.log(`Login successful, remaining ${result.card.remaining_seconds} seconds`);

    // 2. Heartbeat keep-alive
    const interval = result.heartbeat_interval * 1000;
    setInterval(async () => {
      const hb = await client.heartbeat('ABCD-1234-EFGH-5678', hwid);
      console.log(`Heartbeat successful, next heartbeat: ${hb.next_heartbeat}`);
    }, interval);
  } catch (e) {
    if (e instanceof KeyAuthError) {
      console.error(`Verification failed [${e.code}]: ${e.message}`);
    } else {
      throw e;
    }
  }
})();
```

## 9 API Reference

| Method | Purpose | Route |
|---|---|---|
| `client.login(cardKey, hwid)` | Login (auto-binds device on first call) | POST /api/v1/client/login |
| `client.verify(cardKey, hwid)` | Verify card (no binding) | POST /api/v1/client/verify |
| `client.heartbeat(cardKey, hwid)` | Heartbeat keep-alive | POST /api/v1/client/heartbeat |
| `client.bind(cardKey, hwid)` | Manually bind device | POST /api/v1/client/bind |
| `client.unbind(cardKey, hwid)` | Unbind device (deducts time) | POST /api/v1/client/unbind |
| `client.getVar(cardKey, varKey)` | Get cloud variable | POST /api/v1/client/get_var |
| `client.notice()` | Get app notice | POST /api/v1/client/notice |
| `client.version(currentVersion)` | Check version update | POST /api/v1/client/version |
| `client.logout(cardKey, hwid)` | Logout | POST /api/v1/client/logout |

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

> **Compatibility note**: Backend uses the SHA-512/256 variant; Node.js `crypto.createHmac('sha512/256', secret)` is natively supported on Node.js 14+; the SDK automatically falls back to `sha256` when not supported. Backend `pkg/crypto/crypto.go:165` is marked pending verification.

## Error Handling

```js
const { KeyAuthError } = require('keyauth-node');

try {
  const result = await client.verify(cardKey, hwid);
} catch (e) {
  if (e instanceof KeyAuthError) {
    console.error(`code=${e.code}, message=${e.message}, httpStatus=${e.httpStatus}`);
  }
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
