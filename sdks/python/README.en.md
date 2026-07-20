# KeyAuth SaaS Python SDK

> [中文](README.md) | **English**

Client-side SDK for terminal software, wrapping 9 KeyAuth SaaS verification APIs.

## Installation

```bash
pip install requests>=2.20
# Pending release: pip install keyauth-py
```

Alternatively, `git clone` and import directly:

```python
import sys
sys.path.insert(0, "path/to/sdks/python")
from keyauth import KeyAuthClient, KeyAuthError
```

## Quick Start

```python
from keyauth import KeyAuthClient, KeyAuthError

client = KeyAuthClient(
    api_base="https://yourdomain.com",
    app_key="ak_xxx",        # Application AppKey (Developer Console → App Management → Details)
    sign_secret="sk_xxx",    # Application SignSecret (plaintext, after AES decryption)
    timeout=10.0,
)

hwid = "cpu-motherboard-mac-disk-hash"  # Your hardware fingerprint algorithm

try:
    # 1. Login (auto-binds device on first call)
    result = client.login(card_key="ABCD-1234-EFGH-5678", hwid=hwid,
                          device_name="My PC", device_type="windows")
    print(f"Login successful, remaining {result['card'].remaining_seconds} seconds")

    # 2. Heartbeat keep-alive (call periodically per heartbeat_interval)
    import time
    interval = result.get("heartbeat_interval", 60)
    while True:
        time.sleep(interval)
        hb = client.heartbeat(card_key="ABCD-1234-EFGH-5678", hwid=hwid)
        print(f"Heartbeat successful, next heartbeat: {hb['next_heartbeat']}")

except KeyAuthError as e:
    print(f"Verification failed [{e.code}]: {e.message}")
```

## 9 API Reference

| Method | Purpose | Route |
|---|---|---|
| `client.login` | Login (auto-binds device on first call) | POST /api/v1/client/login |
| `client.verify` | Verify card (no binding) | POST /api/v1/client/verify |
| `client.heartbeat` | Heartbeat keep-alive | POST /api/v1/client/heartbeat |
| `client.bind` | Manually bind device | POST /api/v1/client/bind |
| `client.unbind` | Unbind device (deducts time) | POST /api/v1/client/unbind |
| `client.get_var` | Get cloud variable | POST /api/v1/client/get_var |
| `client.notice` | Get app notice | POST /api/v1/client/notice |
| `client.version` | Check version update | POST /api/v1/client/version |
| `client.logout` | Logout | POST /api/v1/client/logout |

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

> **Compatibility note**: Backend uses the SHA-512/256 variant; Python `hashlib.new("sha512_256")` is natively supported on OpenSSL 1.1+; the SDK automatically falls back to `hashlib.sha256` when unavailable. Backend `pkg/crypto/crypto.go:165` is marked pending verification — to unify on plain SHA-256, file an issue with the backend.

## Error Handling

```python
from keyauth import KeyAuthError

try:
    result = client.verify(card_key="...", hwid="...")
except KeyAuthError as e:
    print(f"code={e.code}, message={e.message}, http_status={e.http_status}")
    # For error code reference, see docs/SPEC.md section 3.5
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
