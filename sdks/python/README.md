# KeyAuth SaaS Python SDK

> **中文** | [English](README.en.md)

面向终端软件的客户端 SDK，封装 KeyAuth SaaS 9 个验证 API。

## 安装

```bash
pip install requests>=2.20
# 待发布：pip install keyauth-py
```

当前可直接 `git clone` 后引入：

```python
import sys
sys.path.insert(0, "path/to/sdks/python")
from keyauth import KeyAuthClient, KeyAuthError
```

## 快速开始

```python
from keyauth import KeyAuthClient, KeyAuthError

client = KeyAuthClient(
    api_base="https://yourdomain.com",
    app_key="ak_xxx",        # 应用 AppKey（在开发者控制台 → 应用管理 → 详情中获取）
    sign_secret="sk_xxx",    # 应用 SignSecret（明文，AES 解密后）
    timeout=10.0,
)

hwid = "cpu-motherboard-mac-disk-hash"  # 你的硬件指纹算法

try:
    # 1. 登录（首次自动绑定设备）
    result = client.login(card_key="ABCD-1234-EFGH-5678", hwid=hwid,
                          device_name="我的电脑", device_type="windows")
    print(f"登录成功，剩余 {result['card'].remaining_seconds} 秒")

    # 2. 心跳保活（按 heartbeat_interval 周期调用）
    import time
    interval = result.get("heartbeat_interval", 60)
    while True:
        time.sleep(interval)
        hb = client.heartbeat(card_key="ABCD-1234-EFGH-5678", hwid=hwid)
        print(f"心跳成功，下次心跳: {hb['next_heartbeat']}")

except KeyAuthError as e:
    print(f"验证失败 [{e.code}]: {e.message}")
```

## 9 个 API 速查

| 方法 | 用途 | 路由 |
|---|---|---|
| `client.login(card_key, hwid)` | 登录（首次自动绑定） | POST /api/v1/client/login |
| `client.verify(card_key, hwid)` | 验证卡密（不绑定） | POST /api/v1/client/verify |
| `client.heartbeat(card_key, hwid)` | 心跳保活 | POST /api/v1/client/heartbeat |
| `client.bind(card_key, hwid)` | 手动绑定设备（多机场景） | POST /api/v1/client/bind |
| `client.unbind(card_key, hwid)` | 解绑设备（扣时） | POST /api/v1/client/unbind |
| `client.get_var(card_key, var_key)` | 获取云变量 | POST /api/v1/client/get_var |
| `client.notice()` | 获取应用公告 | POST /api/v1/client/notice |
| `client.version(current_version)` | 检查版本更新 | POST /api/v1/client/version |
| `client.logout(card_key, hwid)` | 退出登录 | POST /api/v1/client/logout |

## 签名算法

与后端 `internal/middleware/signature.go` 一致：

```
原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
签名 = HMAC-SHA512/256(sign_secret, 原文) → 64 位小写 hex
```

请求头：

```
X-App-Key:    ak_xxx
X-Timestamp:  1767225600
X-Nonce:      32 位 UUID
X-Signature:  64 位 hex 签名
```

> **兼容性说明**：后端使用 SHA-512/256 变体，Python `hashlib.new("sha512_256")` 在 OpenSSL 1.1+ 支持的环境下原生兼容；不支持时 SDK 自动回退到 `hashlib.sha256`。后端 `pkg/crypto/crypto.go:165` 已标注待核实，如需统一改用标准 SHA-256，可向后端提交 issue。

## 错误处理

```python
from keyauth import KeyAuthError

try:
    result = client.verify(card_key="...", hwid="...")
except KeyAuthError as e:
    print(f"code={e.code}, message={e.message}, http_status={e.http_status}")
    # 错误码参考 docs/SPEC.md 3.5 节
```

| 错误码 | 含义 |
|---|---|
| 2001 | 卡密不存在或已失效 |
| 2002 | 卡密已过期 / 已封禁 / 已禁用 / 次数用尽 |
| 2003 | 设备数超限 |
| 2004 | 设备未绑定 |
| 2005 | 超过离线宽限期 |
| 2006 | 设备已绑定该卡密 |
| 2007 | 设备未绑定 |
| 2008 | 云变量不存在 |
| 1001 | 参数错误 / 时间戳超出范围 / Nonce 重复 |
| 1002 | 签名校验失败 / 签名参数缺失 |
| 1006 | 网络错误 / 响应非 JSON |
| 3001 | 应用不存在或已禁用 |

## 铁律遵守

- **04 禁硬编码**：API 地址 / AppKey / SignSecret 全部由调用方传入，SDK 内不硬编码
- **06 防幻觉**：所有接口错误抛出 `KeyAuthError(code, message)`，不静默吞异常；签名算法严格对齐后端

## 许可证

Proprietary —— 随主项目 KeyAuth SaaS 一同分发
