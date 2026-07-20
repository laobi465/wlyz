# KeyAuth SaaS C# SDK

> [中文](README.md) | **English**

Client-side SDK for terminal software, wrapping 9 KeyAuth SaaS verification APIs.

## Installation

### Source integration

Copy `sdks/csharp/src/KeyAuth/KeyAuthClient.cs` directly into your project.

### Project reference

```bash
cd sdks/csharp/src/KeyAuth
dotnet pack -c Release
# Output: bin/Release/KeyAuth.Sdk.0.4.0.nupkg
dotnet add package KeyAuth.Sdk --version 0.4.0
```

### Enable BouncyCastle (recommended for production)

```bash
# Add a conditional reference in the project file
dotnet build /p:UseBouncyCastle=true
# Or install directly
dotnet add package BouncyCastle.Cryptography
```

## Quick Start

```csharp
using KeyAuth.Sdk;

var client = new KeyAuthClient(
    "https://your-domain.com",
    "ak_your_app_key",
    "sk_your_sign_secret"
);

try
{
    var login = await client.LoginAsync(
        "ABCD-1234-EFGH-5678",
        "hwid-cpu-mac-disk",
        "My PC",
        "pc"
    );
    Console.WriteLine($"Token: {login.GetProperty("token").GetString()}");
}
catch (KeyAuthException e)
{
    Console.Error.WriteLine($"Error code={e.Code} msg={e.Message}");
}
```

## 9 API Reference

| Method | Purpose | Route |
|---|---|---|
| `client.LoginAsync` | Login (auto-binds device on first call) | POST /api/v1/client/login |
| `client.VerifyAsync` | Verify card (no binding) | POST /api/v1/client/verify |
| `client.HeartbeatAsync` | Heartbeat keep-alive | POST /api/v1/client/heartbeat |
| `client.BindAsync` | Manually bind device | POST /api/v1/client/bind |
| `client.UnbindAsync` | Unbind device (deducts time) | POST /api/v1/client/unbind |
| `client.GetVarAsync` | Get cloud variable | POST /api/v1/client/get_var |
| `client.NoticeAsync` | Get app notice | POST /api/v1/client/notice |
| `client.VersionAsync` | Check version update | POST /api/v1/client/version |
| `client.LogoutAsync` | Logout | POST /api/v1/client/logout |

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

> **Compatibility note**: .NET does not natively provide `HMACSHA512/256`. The SDK reflects on the BouncyCastle provider: when BouncyCastle is loaded, it uses `HMac(Sha512_256Digest)` for byte-level alignment with backend; when not loaded, it falls back to `HMACSHA256` (consistent with the Python/PHP fallback strategy — same signature length but different algorithm; the fallback scenario will NOT match the backend). **Production strongly recommends installing the `BouncyCastle.Cryptography` NuGet package.**

## Error Handling

```csharp
try
{
    var result = await client.VerifyAsync(cardKey, hwid);
}
catch (KeyAuthException e)
{
    Console.Error.WriteLine($"code={e.Code}, message={e.Message}, httpStatus={e.HttpStatus}");
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
- **Rule 06 (Anti-hallucination)**: All errors throw `KeyAuthException(code, message)`; signature algorithm strictly aligns with backend

## License

Proprietary — Distributed with the main KeyAuth SaaS project.
