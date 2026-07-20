# KeyAuth SaaS Node.js SDK

面向终端软件的客户端 SDK，封装 KeyAuth SaaS 9 个验证 API。**无第三方依赖**（仅用 Node.js 内置 `https` / `crypto` 模块）。

## 安装

```bash
# 待发布：npm install keyauth-node
# 当前：直接 git clone 后 require
```

```js
const { KeyAuthClient, KeyAuthError } = require('./path/to/sdks/nodejs');
```

## 快速开始

```js
const { KeyAuthClient, KeyAuthError } = require('keyauth-node');

const client = new KeyAuthClient({
  apiBase: 'https://yourdomain.com',
  appKey: 'ak_xxx',
  signSecret: 'sk_xxx',
  timeout: 10000,
});

const hwid = 'cpu-motherboard-mac-disk-hash';

(async () => {
  try {
    // 1. 登录（首次自动绑定设备）
    const result = await client.login('ABCD-1234-EFGH-5678', hwid, {
      deviceName: '我的电脑',
      deviceType: 'windows',
    });
    console.log(`登录成功，剩余 ${result.card.remaining_seconds} 秒`);

    // 2. 心跳保活
    const interval = result.heartbeat_interval * 1000;
    setInterval(async () => {
      const hb = await client.heartbeat('ABCD-1234-EFGH-5678', hwid);
      console.log(`心跳成功，下次心跳: ${hb.next_heartbeat}`);
    }, interval);
  } catch (e) {
    if (e instanceof KeyAuthError) {
      console.error(`验证失败 [${e.code}]: ${e.message}`);
    } else {
      throw e;
    }
  }
})();
```

## 9 个 API 速查

| 方法 | 用途 | 路由 |
|---|---|---|
| `client.login(cardKey, hwid)` | 登录（首次自动绑定） | POST /api/v1/client/login |
| `client.verify(cardKey, hwid)` | 验证卡密（不绑定） | POST /api/v1/client/verify |
| `client.heartbeat(cardKey, hwid)` | 心跳保活 | POST /api/v1/client/heartbeat |
| `client.bind(cardKey, hwid)` | 手动绑定设备（多机场景） | POST /api/v1/client/bind |
| `client.unbind(cardKey, hwid)` | 解绑设备（扣时） | POST /api/v1/client/unbind |
| `client.getVar(cardKey, varKey)` | 获取云变量 | POST /api/v1/client/get_var |
| `client.notice()` | 获取应用公告 | POST /api/v1/client/notice |
| `client.version(currentVersion)` | 检查版本更新 | POST /api/v1/client/version |
| `client.logout(cardKey, hwid)` | 退出登录 | POST /api/v1/client/logout |

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
X-Nonce:      32 位随机 hex
X-Signature:  64 位 hex 签名
```

> **兼容性说明**：后端使用 SHA-512/256 变体，Node.js `crypto.createHmac('sha512/256', secret)` 在 Node.js 14+ 原生支持；不支持时 SDK 自动回退到 `sha256`。后端 `pkg/crypto/crypto.go:165` 已标注待核实。

## 错误处理

```js
const { KeyAuthError } = require('keyauth-node');

try {
  const result = await client.verify(cardKey, hwid);
} catch (e) {
  if (e instanceof KeyAuthError) {
    console.error(`code=${e.code}, message=${e.message}, httpStatus=${e.httpStatus}`);
  }
}
```

错误码与 Python SDK 一致，详见 `docs/SPEC.md` 3.5 节。

## TypeScript 支持

`index.d.ts` 已包含完整类型定义，TS 项目可直接 import：

```ts
import { KeyAuthClient, KeyAuthError, LoginResult } from 'keyauth-node';
```

## 铁律遵守

- **04 禁硬编码**：API 地址 / AppKey / SignSecret 全部由调用方传入，SDK 内不硬编码
- **06 防幻觉**：所有接口错误抛出 `KeyAuthError(code, message)`，不静默吞异常

## 许可证

Proprietary —— 随主项目 KeyAuth SaaS 一同分发
