# KeyAuth Java SDK

> **中文** | [English](README.en.md)

面向终端软件的客户端 SDK，封装 KeyAuth SaaS 9 个验证 API。

## 特性

- **JDK 11+ 原生**：使用 `java.net.http.HttpClient`，无 Apache HttpClient 等额外 HTTP 依赖
- **签名对齐**：JDK 17+ 使用 `HmacSHA512/256`，JDK 11-16 回退 `HmacSHA256`（与后端 `sha512.New512_256` 对齐策略一致）
- **强类型异常**：所有错误抛出 `KeyAuthException(code, message, httpStatus)`
- **JSON 处理**：基于 Jackson Databind

## 安装

### Maven

```xml
<dependency>
    <groupId>com.keyauth</groupId>
    <artifactId>keyauth-sdk</artifactId>
    <version>0.4.0</version>
</dependency>
```

### 手动编译

```bash
cd sdks/java
mvn clean package
# 产物：target/keyauth-sdk-0.4.0.jar
```

## 快速开始

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
                "我的电脑",
                "pc"
            );
            System.out.println("Token: " + login.get("token"));
        } catch (KeyAuthException e) {
            System.err.printf("错误 code=%d msg=%s%n", e.getCode(), e.getMessage());
        }
    }
}
```

## API 列表

| 方法 | 路径 | 说明 |
|---|---|---|
| `login` | POST /api/v1/client/login | 登录（首次自动绑定设备） |
| `verify` | POST /api/v1/client/verify | 验证卡密有效性 |
| `heartbeat` | POST /api/v1/client/heartbeat | 心跳保活 |
| `bind` | POST /api/v1/client/bind | 手动绑定设备 |
| `unbind` | POST /api/v1/client/unbind | 解绑设备 |
| `getVar` | POST /api/v1/client/get_var | 获取云变量 |
| `notice` | POST /api/v1/client/notice | 获取应用公告 |
| `version` | POST /api/v1/client/version | 检查版本更新 |
| `logout` | POST /api/v1/client/logout | 退出登录 |

## 签名算法

```
原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
签名 = HMAC-SHA512/256(secret, 原文) → 64 位小写 hex
```

JDK 17+ 原生支持 `Mac.getInstance("HmacSHA512/256")`；JDK 11-16 自动回退 `HmacSHA256`（与 Python/PHP SDK 回退策略一致，后端 `sha512.New512_256` 与 `sha256` 输出长度都是 64 hex，但算法不同——回退场景下签名会与后端不匹配，建议生产环境使用 JDK 17+ 或部署 BouncyCastle 提供者）。

## 跨语言签名对齐测试

`sdks/tests/Sign.java` 是独立签名对齐脚本（不依赖 Jackson），被 `apps/server/pkg/crypto/sign_alignment_test.go` 调用：

```bash
cd sdks/tests
javac Sign.java
java Sign "your-secret" "POST\n/api/v1/client/login\n1721374800\na1b2c3d4e5f6\n{}"
```

## 铁律遵守

- **铁律 04（无硬编码）**：API 地址 / AppKey / SignSecret 全部由构造函数传入
- **铁律 05（配置走后端）**：SDK 不内置任何配置
- **铁律 06（反幻觉）**：错误码与消息透传后端，不篡改

## 版本

v0.4.0 — 2026-07-20
