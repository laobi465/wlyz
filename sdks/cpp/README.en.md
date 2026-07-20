# KeyAuth SaaS C++ SDK

> [中文](README.md) | **English**

Client-side SDK for terminal software, wrapping 9 KeyAuth SaaS verification APIs.

## Installation

### CMake integration

```bash
cd sdks/cpp
mkdir build && cd build
cmake .. -DCMAKE_BUILD_TYPE=Release
make -j$(nproc)
# Output: libkeyauth.a + keyauth_example
```

### System dependencies

```bash
# Ubuntu / Debian
sudo apt install -y libcurl4-openssl-dev libssl-dev nlohmann-json3-dev

# CentOS / RHEL
sudo yum install -y libcurl-devel openssl-devel

# macOS
brew install curl openssl nlohmann-json
```

## Quick Start

```cpp
#include "keyauth/keyauth.hpp"
#include <nlohmann/json.hpp>
#include <iostream>

int main() {
    keyauth::KeyAuthClient client(
        "https://your-domain.com",
        "ak_your_app_key",
        "sk_your_sign_secret"
    );

    try {
        std::string loginJson = client.Login(
            "ABCD-1234-EFGH-5678",
            "hwid-cpu-mac-disk",
            "My PC",
            "pc"
        );
        auto login = nlohmann::json::parse(loginJson);
        std::cout << "Token: " << login["token"] << std::endl;
    } catch (const keyauth::KeyAuthException& e) {
        std::cerr << "code=" << e.code() << " msg=" << e.message() << std::endl;
    }
}
```

Compile:

```bash
g++ -std=c++17 main.cpp -I sdks/cpp/include \
    -L sdks/cpp/build -lkeyauth \
    -lcurl -lcrypto -lssl
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

> **Compatibility note**: OpenSSL 1.1+ provides `EVP_sha512_256()`; the C++ SDK uses this function for byte-level alignment with backend. If the linked OpenSSL is < 1.1 (no SHA-512/256 support), the SDK automatically falls back to `EVP_sha256()` and emits a stderr warning.

## Error Handling

```cpp
try {
    std::string loginJson = client.Login(cardKey, hwid, deviceName, deviceType);
} catch (const keyauth::KeyAuthException& e) {
    std::cerr << "code=" << e.code() << " msg=" << e.message()
              << " http_status=" << e.http_status() << std::endl;
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
- **Rule 06 (Anti-hallucination)**: All errors throw `keyauth::KeyAuthException(code, message)`; signature algorithm strictly aligns with backend

## License

Proprietary — Distributed with the main KeyAuth SaaS project.
