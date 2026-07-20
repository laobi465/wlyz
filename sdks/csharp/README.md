# KeyAuth C# SDK

面向终端软件的客户端 SDK，封装 KeyAuth SaaS 9 个验证 API。

## 特性

- **.NET 6+ 原生**：使用 `HttpClient` + `JsonDocument`，无第三方 JSON 库
- **签名对齐**：优先反射探测 BouncyCastle 提供者，自动启用 `SHA-512/256`；不可用回退 `HMACSHA256`
- **异步 API**：所有方法返回 `Task<JsonElement>`
- **强类型异常**：`KeyAuthException(code, message, httpStatus)`

## 安装

### 源码集成

将 `sdks/csharp/src/KeyAuth/KeyAuthClient.cs` 直接复制到项目中。

### 项目引用

```bash
cd sdks/csharp/src/KeyAuth
dotnet pack -c Release
# 产物：bin/Release/KeyAuth.Sdk.0.4.0.nupkg
dotnet add package KeyAuth.Sdk --version 0.4.0
```

### 启用 BouncyCastle（推荐生产环境）

```bash
# 项目文件中添加条件引用
dotnet build /p:UseBouncyCastle=true
# 或直接安装
dotnet add package BouncyCastle.Cryptography
```

## 快速开始

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
        "我的电脑",
        "pc"
    );
    Console.WriteLine($"Token: {login.GetProperty("token").GetString()}");
}
catch (KeyAuthException e)
{
    Console.Error.WriteLine($"错误 code={e.Code} msg={e.Message}");
}
```

## API 列表

| 方法 | 路径 | 说明 |
|---|---|---|
| `LoginAsync` | POST /api/v1/client/login | 登录（首次自动绑定设备） |
| `VerifyAsync` | POST /api/v1/client/verify | 验证卡密有效性 |
| `HeartbeatAsync` | POST /api/v1/client/heartbeat | 心跳保活 |
| `BindAsync` | POST /api/v1/client/bind | 手动绑定设备 |
| `UnbindAsync` | POST /api/v1/client/unbind | 解绑设备 |
| `GetVarAsync` | POST /api/v1/client/get_var | 获取云变量 |
| `NoticeAsync` | POST /api/v1/client/notice | 获取应用公告 |
| `VersionAsync` | POST /api/v1/client/version | 检查版本更新 |
| `LogoutAsync` | POST /api/v1/client/logout | 退出登录 |

## 签名算法

```
原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
签名 = HMAC-SHA512/256(secret, 原文) → 64 位小写 hex
```

.NET 原生不直接提供 `HMACSHA512/256`。SDK 通过反射探测 BouncyCastle 提供者：
- 已加载 BouncyCastle：使用 `HMac(Sha512_256Digest)` 与后端字节级对齐
- 未加载：回退 `HMACSHA256`（与 Python/PHP 回退策略一致，签名长度相同但算法不同——回退场景下会与后端不匹配）

**生产环境强烈建议安装 `BouncyCastle.Cryptography` NuGet 包**。

## 跨语言签名对齐测试

`sdks/tests/sign.cs` 是独立签名脚本：

```bash
cd sdks/tests
csc sign.cs /out:sign.exe
./sign.exe "your-secret" "POST\n/api/v1/client/login\n1721374800\na1b2c3d4e5f6\n{}"
```

## 铁律遵守

- **铁律 04（无硬编码）**：API 地址 / AppKey / SignSecret 全部由构造函数传入
- **铁律 05（配置走后端）**：SDK 不内置任何配置
- **铁律 06（反幻觉）**：错误码与消息透传后端，不篡改；签名算法明确标注回退行为

## 版本

v0.4.0 — 2026-07-20
