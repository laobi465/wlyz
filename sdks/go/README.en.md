# KeyAuth SaaS Go SDK

> [中文](README.md) | **English**

Client-side SDK for terminal software, wrapping 9 KeyAuth SaaS verification APIs.

## Installation

```bash
go get github.com/your-org/keyauth-saas/sdks/go/keyauth
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    keyauth "github.com/your-org/keyauth-saas/sdks/go/keyauth"
)

func main() {
    client := keyauth.NewClient(
        "https://your-domain.com",
        "ak_your_app_key",
        "sk_your_sign_secret",
    )

    login, err := client.Login("ABCD-1234-EFGH-5678", "hwid-cpu-mac-disk", "My PC", "pc")
    if err != nil {
        log.Fatalf("Login failed: %v", err)
    }
    fmt.Printf("Token: %s\n", login.Token)
}
```

## 9 API Reference

| Method | Purpose | Route |
|---|---|---|
| `client.Login` | Login (auto-binds device on first call) | POST /api/v1/client/login |
| `client.Verify` | Verify card (no binding) | POST /api/v1/client/verify |
| `client.Heartbeat` | Heartbeat keep-alive | POST /api/v1/client/heartbeat |
| `client.Bind` | Manually bind device | POST /api/v1/client/bind |
| `client.Unbind` | Unbind device (deducts time) | POST /api/v1/client/unbind |
| `client.GetVar` | Get cloud variable | POST /api/v1/client/get_var |
| `client.Notice` | Get app notice | POST /api/v1/client/notice |
| `client.Version` | Check version update | POST /api/v1/client/version |
| `client.Logout` | Logout | POST /api/v1/client/logout |

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

> **Compatibility note**: Backend uses `sha512.New512_256` (SHA-512/256 variant). The Go SDK uses `crypto/sha512.New512_256` — byte-level alignment with backend, no fallback.

## Error Handling

```go
login, err := client.Login(...)
if err != nil {
    var kae *keyauth.KeyAuthError
    if errors.As(err, &kae) {
        fmt.Printf("Business error code=%d msg=%s\n", kae.Code, kae.Message)
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
