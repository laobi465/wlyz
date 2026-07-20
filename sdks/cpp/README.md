# KeyAuth C++ SDK

面向终端软件的客户端 SDK，封装 KeyAuth SaaS 9 个验证 API。

## 特性

- **C++17 标准**：跨平台（Linux / macOS / Windows）
- **OpenSSL 1.1+ 原生对齐**：`EVP_sha512_256()` 与后端 `sha512.New512_256` 字节级一致
- **libcurl HTTP**：成熟稳定的 HTTP 客户端
- **nlohmann/json**：JSON 序列化/反序列化事实标准
- **强类型异常**：`keyauth::KeyAuthException(code, message, http_status)`

## 依赖

- C++17 编译器（g++ 7+ / clang++ 6+ / MSVC 2019+）
- libcurl（HTTP 客户端）
- OpenSSL 1.1+（推荐 3.0+，提供 `EVP_sha512_256`）
- nlohmann/json（CMake 自动 FetchContent）

## 安装

### CMake 集成

```bash
cd sdks/cpp
mkdir build && cd build
cmake .. -DCMAKE_BUILD_TYPE=Release
make -j$(nproc)
# 产物：libkeyauth.a + keyauth_example
```

### 系统依赖安装

```bash
# Ubuntu / Debian
sudo apt install -y libcurl4-openssl-dev libssl-dev nlohmann-json3-dev

# CentOS / RHEL
sudo yum install -y libcurl-devel openssl-devel

# macOS
brew install curl openssl nlohmann-json
```

## 快速开始

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
            "我的电脑",
            "pc"
        );
        auto login = nlohmann::json::parse(loginJson);
        std::cout << "Token: " << login["token"] << std::endl;
    } catch (const keyauth::KeyAuthException& e) {
        std::cerr << "code=" << e.code() << " msg=" << e.message() << std::endl;
    }
}
```

编译：

```bash
g++ -std=c++17 main.cpp -I sdks/cpp/include \
    -L sdks/cpp/build -lkeyauth \
    -lcurl -lcrypto -lssl
```

## API 列表

| 方法 | 路径 | 说明 |
|---|---|---|
| `Login` | POST /api/v1/client/login | 登录（首次自动绑定设备） |
| `Verify` | POST /api/v1/client/verify | 验证卡密有效性 |
| `Heartbeat` | POST /api/v1/client/heartbeat | 心跳保活 |
| `Bind` | POST /api/v1/client/bind | 手动绑定设备 |
| `Unbind` | POST /api/v1/client/unbind | 解绑设备 |
| `GetVar` | POST /api/v1/client/get_var | 获取云变量 |
| `Notice` | POST /api/v1/client/notice | 获取应用公告 |
| `Version` | POST /api/v1/client/version | 检查版本更新 |
| `Logout` | POST /api/v1/client/logout | 退出登录 |

## 签名算法

```
原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
签名 = HMAC-SHA512/256(secret, 原文) → 64 位小写 hex
```

OpenSSL 1.1+ 提供 `EVP_sha512_256()`，C++ SDK 使用此函数与后端字节级对齐；若链接的 OpenSSL < 1.1（无 SHA-512/256），自动回退 `EVP_sha256()` 并 stderr 输出警告。

## 跨语言签名对齐测试

`sdks/tests/sign.cpp` 是独立签名对齐脚本（仅依赖 OpenSSL，无 libcurl/json）：

```bash
cd sdks/tests
g++ -std=c++17 -O2 sign.cpp -o sign -lcrypto
./sign "your-secret" "POST\n/api/v1/client/login\n1721374800\na1b2c3d4e5f6\n{}"
```

## 铁律遵守

- **铁律 04（无硬编码）**：API 地址 / AppKey / SignSecret 全部由构造函数传入
- **铁律 05（配置走后端）**：SDK 不内置任何配置
- **铁律 06（反幻觉）**：错误码与消息透传后端，不篡改；回退行为明确标注 stderr 警告

## 版本

v0.4.0 — 2026-07-20
