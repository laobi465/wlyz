# KeyAuth 易语言 SDK

> **中文** | [English](README.en.md)

面向终端软件的客户端 SDK，封装 KeyAuth SaaS 9 个验证 API。

## 特性

- **纯中文 API**：所有方法名 / 参数名 / 注释均为中文，符合易语言开发者习惯
- **精易模块依赖**：基于精易模块 v9.0+ 的 `HMAC_SHA256` / `json_解析` / `网页_访问`
- **Windows 平台**：易语言仅支持 Windows（7+ 推荐）

## 依赖

- 易语言 5.9+
- 精易模块 v9.0+（[下载地址](https://www.3600gz.cn/)）

## 安装

1. 打开易语言 IDE
2. 将 `sdks/epl/keyauth_sdk.e.txt` 内容复制到新建的「易模块」中
3. 编译为 `keyauth_sdk.fne`（精易模块格式）
4. 在主项目中「引用模块」→ 选择 `keyauth_sdk.fne`
5. 调用 `KeyAuth_SDK.初始化(...)` 启动

## 快速开始

```
' 初始化（铁律 04：所有配置由调用方传入）
KeyAuth_SDK.初始化 (“https://your-domain.com”, “ak_your_app_key”, “sk_your_sign_secret”, 10)

' 1. 登录
登录响应 = KeyAuth_SDK.登录 (“ABCD-1234-EFGH-5678”, “cpu-mac-disk-hash”, “我的电脑”, “pc”)
调试输出 (登录响应)

' 2. 心跳
心跳响应 = KeyAuth_SDK.心跳 (“ABCD-1234-EFGH-5678”, “cpu-mac-disk-hash”)

' 3. 取云变量
变量响应 = KeyAuth_SDK.取变量 (“ABCD-1234-EFGH-5678”, “pro_feature”)

' 4. 退出
KeyAuth_SDK.退出 (“ABCD-1234-EFGH-5678”, “cpu-mac-disk-hash”)
```

## API 列表

| 方法 | 路径 | 说明 |
|---|---|---|
| `登录` | POST /api/v1/client/login | 登录（首次自动绑定设备） |
| `验证` | POST /api/v1/client/verify | 验证卡密有效性 |
| `心跳` | POST /api/v1/client/heartbeat | 心跳保活 |
| `绑定设备` | POST /api/v1/client/bind | 手动绑定设备 |
| `解绑设备` | POST /api/v1/client/unbind | 解绑设备 |
| `取变量` | POST /api/v1/client/get_var | 获取云变量 |
| `取公告` | POST /api/v1/client/notice | 获取应用公告 |
| `取版本` | POST /api/v1/client/version | 检查版本更新 |
| `退出` | POST /api/v1/client/logout | 退出登录 |
| `计算签名` | （静态方法） | HMAC-SHA256 → 64 位小写 hex |

## 签名算法

```
原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
签名 = HMAC-SHA256(secret, 原文) → 64 位小写 hex
```

**重要限制**：易语言生态暂无稳定的 SHA-512/256 实现，本 SDK 使用 HMAC-SHA256。后端使用 `sha512.New512_256`（SHA-512/256 变体），输出长度都是 64 hex 但算法不同——本 SDK 的签名**仅在「后端 crypto.go:165 待核实兼容性」回退场景下与后端匹配**。

**生产环境建议**：
- 若应用对签名算法严格匹配有要求，请使用 **Go / C++ SDK**（原生 SHA-512/256）
- 若可接受 SHA-256 兼容模式，本易语言 SDK 可正常工作（需后端 crypto.go 同时支持回退）

## 跨语言签名对齐测试

`sdks/tests/sign.e.txt` 是签名对齐脚本（精易模块依赖）：

1. 在易语言 IDE 中打开 `sign.e.txt`
2. 引用精易模块
3. 运行，调试窗口会输出两个测试用例的签名

注：易语言 SDK 不参与 `apps/server/pkg/crypto/sign_alignment_test.go` 自动化测试（Windows-only，无法在 Linux CI 中执行）。

## 铁律遵守

- **铁律 04（无硬编码）**：API 地址 / AppKey / SignSecret 全部由 `初始化` 子程序传入
- **铁律 05（配置走后端）**：SDK 不内置任何配置
- **铁律 06（反幻觉）**：错误码与消息透传后端，不篡改；签名算法限制明确标注

## 版本

v0.4.0 — 2026-07-20
