# KeyAuth Go SDK

面向终端软件的客户端 SDK，封装 KeyAuth SaaS 9 个验证 API。

## 特性

- **零第三方依赖**：仅使用 Go 标准库（net/http / crypto/sha512 / encoding/json / time）
- **签名原生对齐**：`crypto/sha512.New512_256` 与后端 `crypto.HMACSHA256` 完全一致，无回退
- **强类型返回**：每个 API 返回专用 struct，避免 map 取值
- **错误不静默**：所有错误返回 `*KeyAuthError{Code, Message, HTTPStatus}`

## 安装

```bash
go get github.com/your-org/keyauth-saas/sdks/go/keyauth
```

## 快速开始

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

    login, err := client.Login("ABCD-1234-EFGH-5678", "hwid-cpu-mac-disk", "我的电脑", "pc")
    if err != nil {
        log.Fatalf("登录失败: %v", err)
    }
    fmt.Printf("Token: %s\n", login.Token)
}
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

后端使用 `sha512.New512_256`（SHA-512/256 变体），Go SDK 同样使用 `crypto/sha512.New512_256`，与后端字节级对齐。

## 跨语言签名对齐测试

`sdks/tests/sign.go` 是签名对齐脚本，被 `apps/server/pkg/crypto/sign_alignment_test.go` 调用：

```bash
go run sdks/tests/sign.go "your-secret" "POST\n/api/v1/client/login\n1721374800\na1b2c3d4e5f6\n{}"
```

## 错误处理

```go
login, err := client.Login(...)
if err != nil {
    var kae *keyauth.KeyAuthError
    if errors.As(err, &kae) {
        fmt.Printf("业务错误 code=%d msg=%s\n", kae.Code, kae.Message)
    }
}
```

## 铁律遵守

- **铁律 04（无硬编码）**：API 地址 / AppKey / SignSecret 全部由调用方传入
- **铁律 05（配置走后端）**：SDK 不内置任何配置，由 sys_config 控制后端行为
- **铁律 06（反幻觉）**：错误码与消息透传后端，不篡改；签名使用与后端一致的 SHA-512/256 算法

## 版本

v0.4.0 — 2026-07-20
