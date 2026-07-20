# KeyAuth SaaS 易语言 SDK

> [中文](README.md) | **English**

Client-side SDK for terminal software, wrapping 9 KeyAuth SaaS verification APIs.

## Installation

1. Open the 易语言 (EPL) IDE
2. Copy the contents of `sdks/epl/keyauth_sdk.e.txt` into a new "易模块" (EPL module)
3. Compile to `keyauth_sdk.fne` (精易模块 / JingYi module format)
4. In your main project, "引用模块" (Reference Module) → select `keyauth_sdk.fne`
5. Call `KeyAuth_SDK.初始化(...)` to start

## Quick Start

```text
' Initialize (Rule 04: all config is passed by caller)
KeyAuth_SDK.初始化 ("https://your-domain.com", "ak_your_app_key", "sk_your_sign_secret", 10)

' 1. Login
登录响应 = KeyAuth_SDK.登录 ("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash", "My PC", "pc")
调试输出 (登录响应)

' 2. Heartbeat
心跳响应 = KeyAuth_SDK.心跳 ("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash")

' 3. Get cloud variable
变量响应 = KeyAuth_SDK.取变量 ("ABCD-1234-EFGH-5678", "pro_feature")

' 4. Logout
KeyAuth_SDK.退出 ("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash")
```

## 9 API Reference

| Method | Purpose | Route |
|---|---|---|
| `登录` | Login (auto-binds device on first call) | POST /api/v1/client/login |
| `验证` | Verify card (no binding) | POST /api/v1/client/verify |
| `心跳` | Heartbeat keep-alive | POST /api/v1/client/heartbeat |
| `绑定设备` | Manually bind device | POST /api/v1/client/bind |
| `解绑设备` | Unbind device (deducts time) | POST /api/v1/client/unbind |
| `取变量` | Get cloud variable | POST /api/v1/client/get_var |
| `取公告` | Get app notice | POST /api/v1/client/notice |
| `取版本` | Check version update | POST /api/v1/client/version |
| `退出` | Logout | POST /api/v1/client/logout |

> Method names follow each SDK's naming convention (e.g. Python `client.login`, Go `client.Login`, C# `client.LoginAsync`, 易语言 `登录`).

## Signature Algorithm

Aligned with backend `internal/middleware/signature.go`:

```
payload = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
signature = HMAC-SHA256(sign_secret, payload) → 64-char lowercase hex
```

Request headers:

```
X-App-Key:    ak_xxx
X-Timestamp:  1767225600
X-Nonce:      32-char UUID/hex
X-Signature:  64-char hex signature
```

> **Compatibility note**: The 易语言 ecosystem lacks a stable SHA-512/256 implementation, so this SDK uses HMAC-SHA256. **Important**: the signature only matches the backend in the "fallback compatibility" scenario documented at `crypto.go:165`. For strict signature matching, use the Go or C++ SDK instead (both implement native SHA-512/256).

## Error Handling

The 易语言 SDK surfaces backend error codes and messages via the response object; codes are passed through unmodified.

```text
' Inspect 返回响应 for code/message fields returned by the backend
调试输出 (返回响应)
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
