# KeyAuth SaaS Java SDK

> [中文](README.md) | **English**

Client-side SDK for terminal software, wrapping 9 KeyAuth SaaS verification APIs.

## Installation

### Maven

```xml
<dependency>
    <groupId>com.keyauth</groupId>
    <artifactId>keyauth-sdk</artifactId>
    <version>0.4.0</version>
</dependency>
```

### Manual build

```bash
cd sdks/java
mvn clean package
# Output: target/keyauth-sdk-0.4.0.jar
```

## Quick Start

```java
import com.keyauth.sdk.KeyAuthClient;
import com.keyauth.sdk.KeyAuthException;
import java.util.Map;

public class Main {
    public static void main(String[] args) {
        KeyAuthClient client = new KeyAuthClient(
            "https://your-domain.com",
            "ak_your_app_key",
            "sk_your_sign_secret"
        );

        try {
            Map<String, Object> login = client.login(
                "ABCD-1234-EFGH-5678",
                "hwid-cpu-mac-disk",
                "My PC",
                "pc"
            );
            System.out.println("Token: " + login.get("token"));
        } catch (KeyAuthException e) {
            System.err.printf("Error code=%d msg=%s%n", e.getCode(), e.getMessage());
        }
    }
}
```

## 9 API Reference

| Method | Purpose | Route |
|---|---|---|
| `client.login` | Login (auto-binds device on first call) | POST /api/v1/client/login |
| `client.verify` | Verify card (no binding) | POST /api/v1/client/verify |
| `client.heartbeat` | Heartbeat keep-alive | POST /api/v1/client/heartbeat |
| `client.bind` | Manually bind device | POST /api/v1/client/bind |
| `client.unbind` | Unbind device (deducts time) | POST /api/v1/client/unbind |
| `client.getVar` | Get cloud variable | POST /api/v1/client/get_var |
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

> **Compatibility note**: JDK 17+ natively supports `Mac.getInstance("HmacSHA512/256")`; JDK 11-16 automatically falls back to `HmacSHA256` (consistent with the Python/PHP SDK fallback strategy — backend `sha512.New512_256` and `sha256` both produce 64-hex output but use different algorithms; in the fallback scenario the signature will NOT match the backend). For production, use JDK 17+ or install the BouncyCastle provider for JDK 11+ alignment.

## Error Handling

```java
try {
    Map<String, Object> result = client.verify(cardKey, hwid);
} catch (KeyAuthException e) {
    System.err.printf("Error code=%d msg=%s httpStatus=%d%n",
        e.getCode(), e.getMessage(), e.getHttpStatus());
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
