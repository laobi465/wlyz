# 外部发卡网平台对接方案

> **文档版本**：v1.0（规划稿）
> **创建日期**：2026-07-20
> **维护者**：KeyAuth SaaS Team
> **状态**：⏸️ 暂缓实现（方案已归档，待启动时再走 spec 流程）

---

## 0. 现状说明（铁律 06 防幻觉）

### 0.1 本项目（KeyAuth SaaS）已有能力

| 能力 | 状态 | 备注 |
|---|---|---|
| OpenAPI 平台骨架 | ✅ 已实现（v0.4.0 第十五项迁移） | `internal/openapi` 包 + `APITokenAuth` 中间件 |
| 开发者 API Token 管理 | ✅ 已实现 | `/api/v1/tenant/openapi/tokens` CRUD |
| Token 鉴权：SHA-512 哈希存储 + scopes 权限 | ✅ 已实现 | `developer_api_token` 表 |
| 开放 API 入口 `/api/v1/openapi/whoami` | ✅ 已实现 | 用于 Token 自检 |
| Webhook 事件推送 | ✅ 已实现 | 独立能力，非本方案范畴 |

### 0.2 本项目待新增能力（本方案规划，尚未实现）

| 能力 | 状态 | 实现位置（规划） |
|---|---|---|
| `/api/v1/openapi/ping` 连通性测试 | ❌ 待实现 | `internal/handler/openapi.go` |
| `/api/v1/openapi/card_types/list` 拉取卡类 | ❌ 待实现 | 同上 |
| `/api/v1/openapi/cards/generate` 生成卡密 | ❌ 待实现 | 同上 |
| `/api/v1/openapi/cards/list` 查询卡密 | ❌ 待实现 | 同上 |
| `/api/v1/openapi/cards/ban` 封禁卡密 | ❌ 待实现 | 同上 |
| `developer_api_token` 支持 agent 角色 | ❌ 待实现 | migration 020 + model 扩展 |
| `external_api_call_log` 外部调用日志表 | ❌ 待实现 | migration 020 |
| 外部 API 限流 + 日志切面 | ❌ 待实现 | middleware + handler |

### 0.3 另一项目（多商户发卡网平台）状态

- 尚未启动开发
- 本文档作为另一项目的**对接需求文档**输出

---

## 1. 需求梳理

### 1.1 核心场景

开发者/代理在本项目生成 API Token → 配置到另一个多商户发卡网平台 → 另一平台调用本项目 API 一键生成卡密并自动上架到对应商品。

### 1.2 角色权限

- 开发者（tenant）：可生成自己应用的卡密
- 代理（agent）：可生成自己可售卡类的卡密（含余额扣款 + 佣金计算）

### 1.3 对接方向

单向：另一平台 → 本项目（拉取卡密）。本项目作为 API 供应方。

### 1.4 自定义参数

- 数量（count）
- 卡密类型（card_type_id）
- 应用 ID（app_id，开发者必填）
- 备注（remark，可选）

### 1.5 核心交付物

本文档（含 API 规范 + 鉴权协议 + 错误码 + SDK 示例 + 流程图）让另一项目的开发照着实现。

---

## 2. 项目定位与对标分析

### 2.1 本项目在生态中的定位

```
┌──────────────────────┐    调用 API 生成卡密    ┌──────────────────────┐
│  另一个多商户发卡网   │ ───────────────────────►│  本项目 KeyAuth SaaS  │
│  （销售端/前台）      │   API Token + HMAC 签名 │  （卡密供应方/后台）  │
│  - 多商户入驻         │ ◄─────────────────────── │  - 开发者/代理管理    │
│  - 商品上架展示       │    返回卡密明文+签名    │  - 应用/卡类/卡密生成 │
│  - 终端用户购买       │                         │  - 设备绑定/验证      │
│  - 订单/支付          │                         │  - 佣金/结算          │
└──────────────────────┘                         └──────────────────────┘
       ▲                                                    ▲
       │                                                    │
       └────────── 同一开发者开发两个项目 ────────────────┘
```

### 2.2 三大同赛道对标平台

| 对标平台 | 优点 | 缺点 | 差异化方案 |
|---|---|---|---|
| **卡盟平台** | 多年运营、商户体系成熟、卡密池丰富 | 接口封闭、佣金分成高、数据不在自己手里 | 完全自有双方数据，跨平台对账零成本 |
| **发卡网模板**（Laravel/ThinkPHP） | 部署简单、UI 美观、支付通道全 | 无自有卡密供应能力、依赖人工导入 | 本项目作为自动供应源，零人工干预 |
| **独角数卡**（dujiaoka 开源） | 完全开源、社区活跃、支持自动发卡 | 无多商户体系、无 API 对接能力 | 另一平台补足多商户+API 对接 |

### 2.3 差异化定位

「自研卡密验证 SaaS + 自研多商户发卡网」双平台闭环，相比卡盟分成模式，保留 100% 数据和利润；相比独角数卡，有完整的多商户和 API 供应能力。

---

## 3. 整体技术栈选型

### 3.1 本项目（已确定，无需选型）

- 后端：Go 1.22 + Gin + GORM
- 前端：Vue3 + TypeScript + Element Plus
- 数据库：MySQL 8.0 + Redis 7

### 3.2 另一项目推荐栈

由于两个项目都自己开发，建议**复用同栈**（Go + Vue3）以最大化复用经验和代码片段（如 JWT 中间件、响应封装、AES 加密、配置缓存等可直接搬运）。

| 方案 | 适用场景 | 优势 | 劣势 |
|---|---|---|---|
| **Go + Vue3**（推荐） | 已熟悉本项目栈 | 复用 KeyAuth 30%+ 代码 | 支付通道需自己接 |
| **PHP + Laravel** | 想用发卡网现成支付模板 | 生态成熟、支付通道多 | 维护两套栈成本高 |
| **Node.js + NestJS** | 想前后端同语言 | 开发效率高 | 与本项目栈分裂 |

### 3.3 配套基础设施

| 组件 | 选型 |
|---|---|
| 反代 | Nginx + Let's Encrypt SSL |
| 部署 | Docker Compose（与本项目同款） |
| 数据库 | MySQL 8.0（可与本项目同实例或独立实例） |
| 缓存 | Redis 7（独立实例，避免与本项目混淆） |
| 对象存储 | MinIO（卡密 CSV 导入导出） |
| 监控 | Prometheus + Grafana（可选，复用本项目栈） |

---

## 4. 完整模块架构拆分

### 4.1 另一项目（多商户发卡网平台）模块清单

```
多商户发卡网平台
├── 前台用户端（C 端）
│   ├── 首页（商品分类 + 搜索 + 热销推荐）
│   ├── 商品详情页（含库存实时查询）
│   ├── 购卡结算页（数量 + 优惠券 + 支付方式）
│   ├── 支付结果页（卡密明文展示 + 复制按钮）
│   ├── 我的订单（按商户分组）
│   ├── 商户入驻页（提交资料审核）
│   └── 帮助中心 + 公告
│
├── 商户端（B 端，对应本项目的开发者/代理）
│   ├── 工作台（销量/库存/收入概览）
│   ├── 商品管理（CRUD + 绑定 KeyAuth 卡类）
│   ├── 卡密供应配置（KeyAuth API Token + 卡类映射）
│   ├── 一键补货（输入数量 → 调 KeyAuth API 生成卡密 → 自动入库）
│   ├── 订单管理（按商品/状态筛选）
│   ├── 财务结算（收入提现 + 对账单）
│   ├── 数据统计（销量趋势 + 商品排行）
│   └── 商户设置（资料 + 密码 + 2FA + API Token 管理）
│
└── 平台超管端（A 端）
    ├── 平台看板（全站 GMV + 商户数 + 商品数）
    ├── 商户管理（入驻审核 + 封禁 + 抽成比例）
    ├── 商品审核（违规下架）
    ├── 订单全局视图
    ├── 财务管理（商户提现审核 + 平台抽成结算）
    ├── 系统配置（支付通道 + 抽成 + 通知）
    ├── 操作日志 + 安全中心
    └── KeyAuth 对接监控（API 调用统计 + 失败告警）
```

### 4.2 本项目需新增模块（最小改动）

| 模块 | 端 | 功能 |
|---|---|---|
| API Token 管理（agent 角色） | tenant/agent | 生成/吊销对接另一平台用的 Token（扩展 v0.4.0 developer_api_token，新增 agent 角色） |
| 外部调用日志 | admin | 记录另一平台所有 API 调用（便于对账和故障排查） |
| 限流配置 | admin | 每个 Token 的 QPS 限制（防滥用） |
| OpenAPI 业务端点 | openapi | 5 个新端点（ping / card_types / cards generate / cards list / cards ban） |

**核心设计**：本项目作为 API 供应方，**最小改动**即可支持对接。不需要主动推送逻辑，所有补货动作由另一平台主动发起。

---

## 5. 详细页面原型文案（聚焦对接核心页面）

### 5.1 本项目新增页面：API Token 管理（开发者/代理端）

**页面作用**：开发者/代理生成可配置到另一平台的 API Token

**布局**：
```
┌────────────────────────────────────────────┐
│ API Token 管理                       [生成] │
├────────────────────────────────────────────┤
│ ┌────────────────────────────────────────┐ │
│ │ Token: tk_abcde...fghij                │ │
│ │ 名称: 对接到发卡网平台 A                │ │
│ │ 权限: cards:generate cards:list        │ │
│ │ 创建: 2026-07-20 10:00  最后使用: 2分钟前│ │
│ │ QPS 限制: 10/s   状态: 启用             │ │
│ │                  [详情] [吊销]          │ │
│ └────────────────────────────────────────┘ │
│                                            │
│ ┌────────────────────────────────────────┐ │
│ │ Token: tk_xxxxx...yyyyy                │ │
│ │ ...                                    │ │
│ └────────────────────────────────────────┘ │
└────────────────────────────────────────────┘
```

**生成 Token 弹窗**：
- 名称（必填）
- 权限范围（多选）：`cards:generate` / `cards:list` / `cards:ban` / `card_types:list`
- QPS 限制（默认 10，最大 100）
- 过期时间（可选，默认 1 年）
- 点击「生成」→ 弹窗显示完整 Token 明文（仅一次，复制后关闭）

### 5.2 另一项目页面：KeyAuth 对接配置（商户端）

**页面作用**：商户把自己的 KeyAuth API Token 配置进来 + 建立商品与 KeyAuth 卡类的映射

**布局**：
```
┌────────────────────────────────────────────────────┐
│ KeyAuth 对接配置                                   │
├────────────────────────────────────────────────────┤
│ API 凭证                                           │
│ ┌────────────────────────────────────────────────┐ │
│ │ KeyAuth 平台地址: https://keyauth.example.com │ │
│ │ API Token: tk_******************************** │ │
│ │ 角色: 开发者 / 代理（自动识别）                 │ │
│ │ 连通性测试: ✓ 正常                              │ │
│ │                          [保存] [测试连接]      │ │
│ └────────────────────────────────────────────────┘ │
│                                                    │
│ 商品-卡类映射                                      │
│ ┌────────────────────────────────────────────────┐ │
│ │ 本站商品      ←→   KeyAuth 卡类                │ │
│ │ 月卡(¥30)     ←→   月度会员卡 (card_type_id=12)│ │
│ │ 季卡(¥80)     ←→   季度会员卡 (card_type_id=13)│ │
│ │ 永久卡(¥199)  ←→   永久会员卡 (card_type_id=14)│ │
│ │                          [新增映射] [批量补货]  │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

**批量补货弹窗**：
- 选择商品（联动显示对应 KeyAuth 卡类）
- 补货数量（1-1000，受 KeyAuth 端 `external.api.max_push_count` 配置约束）
- 单价（自动从 KeyAuth 卡类同步，可加价）
- 点击「确认补货」→ 调用 KeyAuth API 生成卡密 → 自动入库 → 弹窗显示入库结果

---

## 6. 数据库表结构设计

### 6.1 本项目新增/扩展表（供应方侧，最小改动）

复用 v0.4.0 已有的 `developer_api_token` 表，**仅需扩展支持 agent 角色**：

```sql
-- v0.4.0 已有表，无需新建
developer_api_token (
  id, tenant_id, name, token_hash, prefix, scopes, 
  expires_at, last_used_at, last_used_ip, status, revoked_at, ...
)

-- 待新增 migration 020：扩展支持 agent 角色
ALTER TABLE developer_api_token 
  ADD COLUMN owner_type VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT 'tenant/agent',
  ADD COLUMN owner_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'owner_type 对应 ID';

-- 修改 unique 索引：(owner_type, owner_id, name) 唯一
ALTER TABLE developer_api_token DROP INDEX uk_tenant_name;
ALTER TABLE developer_api_token ADD UNIQUE KEY uk_owner_name (owner_type, owner_id, name);
```

新增 1 张表记录外部调用日志（用于对账）：

```sql
CREATE TABLE external_api_call_log (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  token_id BIGINT UNSIGNED NOT NULL,
  owner_type VARCHAR(16) NOT NULL,
  owner_id BIGINT UNSIGNED NOT NULL,
  endpoint VARCHAR(64) NOT NULL COMMENT '调用的 API 路径',
  method VARCHAR(8) NOT NULL,
  request_ip VARCHAR(45) NOT NULL,
  request_body_hash VARCHAR(64) COMMENT 'SHA-256 哈希，便于去重',
  response_code INT NOT NULL COMMENT '业务 code',
  response_time_ms INT NOT NULL,
  card_ids_json TEXT COMMENT '生成的卡密 ID 列表',
  error_message TEXT,
  created_at DATETIME NOT NULL,
  INDEX idx_token_time (token_id, created_at),
  INDEX idx_owner_time (owner_type, owner_id, created_at)
) ENGINE=InnoDB COMMENT='外部 API 调用日志';
```

### 6.2 另一项目主要表（消费方侧，由另一项目开发）

```sql
-- 商户表
merchants (
  id, username, email, password_hash, status, 
  commission_rate, balance, created_at, ...
)

-- KeyAuth 对接凭证表
keyauth_bindings (
  id, merchant_id, keyauth_endpoint, api_token_enc, 
  owner_type, owner_id, status, last_test_at, last_test_result,
  created_at, updated_at
)

-- 商品表
products (
  id, merchant_id, name, price, stock, 
  keyauth_card_type_id, auto_replenish_threshold, 
  auto_replenish_count, status, ...
)

-- 卡密库存表
cards (
  id, product_id, merchant_id, card_key, 
  keyauth_card_id, source ENUM('manual','keyauth_api'),
  status, sold_at, order_id, ...
)

-- 订单表
orders (
  id, order_no, merchant_id, product_id, quantity, 
  total_amount, pay_status, card_ids_json, 
  created_at, paid_at, ...
)
```

---

## 7. UI 设计风格规范

### 7.1 另一项目设计调性

**风格**：科技商务极简（与本项目 KeyAuth 风格统一）

**配色方案**：
```css
--primary: #1677ff;       /* 主色：科技蓝 */
--success: #52c41a;       /* 成功：库存充足 */
--warning: #faad14;       /* 警示：库存预警 */
--danger:  #ff4d4f;       /* 危险：缺货/封禁 */
--bg:      #f5f7fa;       /* 背景 */
--card-bg: #ffffff;       /* 卡片背景 */
--text:    #1f2937;       /* 主文字 */
--text-secondary: #6b7280; /* 次要文字 */
--border:  #e5e7eb;       /* 边框 */
```

**字体**：
- 中文：`-apple-system, "PingFang SC", "Microsoft YaHei", sans-serif`
- 英文/数字：`"JetBrains Mono", "Fira Code", monospace`（Token、卡密等）

**排版**：
- 桌面端：宽度 1200px 居中，侧边栏 240px
- 移动端：单列布局，抽屉式导航

### 7.2 AI 绘图提示词（用于生成效果图）

```
A modern SaaS admin dashboard for a multi-merchant digital card selling platform.
Clean tech-business style, primary color #1677ff (tech blue).
Left sidebar 240px with merchant avatar, menu items: Dashboard, Products, 
KeyAuth Integration, Orders, Finance, Settings. 
Main content area shows KeyAuth Integration page with API token configuration card
and product-cardtype mapping table. White card backgrounds with subtle shadows,
rounded corners 8px. Typography: PingFang SC for Chinese, JetBrains Mono for codes.
Screenshot style, 1440x900, light mode, professional and trustworthy.
```

---

## 8. 开发工期拆分与版本迭代

### 8.1 一期 MVP（核心对接闭环）

**另一项目侧**（自开发）：
- 商户注册/登录 + 商品 CRUD + 卡密库存 + 订单基础流程
- KeyAuth 对接配置页 + 商品-卡类映射 + 一键补货
- 支付通道接入（先支持支付宝当面付 + 微信扫码）

**本项目侧**（最小改动）：
- 扩展 `developer_api_token` 支持 agent 角色（migration 020）
- 新增 5 个 OpenAPI 端点（ping / card_types / cards generate / cards list / cards ban）
- 新增 `external_api_call_log` 表 + 限流中间件

### 8.2 二期增值

- 自动补货（库存低于阈值自动调 KeyAuth API）
- 双向订单回写（另一平台销售后回写到 KeyAuth 记录卡密流向）
- 多 KeyAuth 实例支持（商户可对接多个 KeyAuth 平台）

### 8.3 三期商业化

- 商户分等级（基础版/专业版/企业版）
- 优惠券 + 满减营销
- 数据看板（销量趋势 + 商户排行 + 商品热度）
- 多语言 + 多币种

---

## 9. 风险预估与运维方案

### 9.1 安全风险

| 风险 | 等级 | 应对方案 |
|---|---|---|
| API Token 泄露 | 高 | Token SHA-512 哈希存储 + 可吊销 + QPS 限制 + IP 白名单（可选） |
| 卡密明文传输被抓包 | 高 | 强制 HTTPS + 响应 RSA-4096 签名（复用本项目已有能力） |
| 重复消费同一 Token 请求 | 中 | nonce 防重放（Redis 缓存 5 分钟） |
| 商户超额补货耗尽卡密 | 中 | 单次补货上限 + 每日补货总量限制（sys_config 配置） |
| 跨商户越权访问 | 高 | owner_type + owner_id 严格隔离 + 中间件校验 |

### 9.2 运维方案

- 双平台独立部署，互不依赖
- 本项目 `external_api_call_log` 按月分区，保留 6 个月
- 每日对账脚本：比对两平台订单数 + 卡密流向
- 故障降级：本项目 API 不可用时，另一平台展示「卡密补货暂停」提示，不影响已售卡密验证

---

## 10. 输出正式对接文档（核心交付物）

### 10.1 概述

#### 10.1.1 对接场景

```
另一平台（消费方）                        本项目 KeyAuth SaaS（供应方）
   商户点「一键补货」
        │
        │ 1. POST /api/v1/openapi/cards/generate
        │    (API Token + HMAC-SHA256 签名)
        ├─────────────────────────────────────────►
        │                                         │
        │                  2. 校验 Token + 签名 + 限流 + 配额
        │                  3. 事务内生成卡密 + 写日志
        │                                         │
        │  4. 返回卡密明文列表 + RSA-4096 签名     │
        │◄─────────────────────────────────────────
        │
   5. 卡密入库到本地 cards 表
   6. 弹窗显示补货结果
```

#### 10.1.2 鉴权方式

**API Token + HMAC-SHA256 签名**（与本项目客户端验证 API 同款协议，已稳定运行）

- **API Token**：开发者/代理在本项目后台生成，格式 `tk_xxxxxxxxxxxx`，SHA-512 哈希存储
- **签名算法**：HMAC-SHA256(API Token, 签名原文)
- **签名原文**：`METHOD\nPATH\nTIMESTAMP\nNONCE\nBODY`

#### 10.1.3 基础信息

| 项目 | 值 |
|---|---|
| Base URL | `https://keyauth.example.com`（替换为实际部署域名） |
| 协议 | HTTPS（强制） |
| 数据格式 | JSON（UTF-8） |
| 字符编码 | UTF-8 |
| 时区 | UTC+8（Asia/Shanghai） |
| 时间戳 | Unix 秒级（10 位） |
| 时间偏差 | 允许 ±300 秒（5 分钟） |

### 10.2 鉴权流程

#### 10.2.1 获取 API Token

开发者/代理登录本项目后台 → 「API Token 管理」页面 → 「生成」按钮 → 填写名称 + 权限范围 → 生成 Token（仅显示一次，请妥善保存）。

#### 10.2.2 请求签名

**签名原文格式**：

```
{HTTP_METHOD}\n{PATH}\n{TIMESTAMP}\n{NONCE}\n{BODY}
```

**字段说明**：

| 字段 | 说明 | 示例 |
|---|---|---|
| HTTP_METHOD | HTTP 方法（大写） | `POST` |
| PATH | 请求路径（不含 query string） | `/api/v1/openapi/cards/generate` |
| TIMESTAMP | 当前 Unix 时间戳（秒） | `1721374800` |
| NONCE | 随机字符串（8-32 位） | `a1b2c3d4e5f6` |
| BODY | 请求体原文（GET 请求为空字符串） | `{"card_type_id":12,"count":10}` |

**签名算法**：

```
signature = HMAC-SHA256(api_token, 签名原文)
         = 64 位小写十六进制字符串
```

**完整签名示例**：

请求：
```http
POST /api/v1/openapi/cards/generate HTTP/1.1
Host: keyauth.example.com
Content-Type: application/json
X-Api-Token: tk_abc123def456
X-Timestamp: 1721374800
X-Nonce: a1b2c3d4e5f6
X-Signature: 9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c5b4a39281706f5e4d3c2b1a0
```

签名原文：
```
POST
/api/v1/openapi/cards/generate
1721374800
a1b2c3d4e5f6
{"card_type_id":12,"count":10}
```

#### 10.2.3 错误响应

```json
{
  "code": 2002,
  "message": "Token 无效",
  "data": null,
  "request_id": "req-uuid-xxx",
  "timestamp": 1721374800
}
```

### 10.3 API 端点

#### 10.3.1 连通性测试

**GET** `/api/v1/openapi/ping`

**权限**：任意有效 Token

**响应示例**：

```json
{
  "code": 0,
  "data": {
    "pong": true,
    "server_time": 1721374800,
    "api_version": "v1"
  }
}
```

#### 10.3.2 生成卡密（核心接口）

**POST** `/api/v1/openapi/cards/generate`

**权限**：`cards:generate`

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| card_type_id | int | 是 | 卡类 ID（开发者/代理可访问的卡类） |
| count | int | 是 | 生成数量（1-100，受 `external.api.max_push_count` 配置约束） |
| app_id | int | 否 | 应用 ID（开发者必填，代理自动取关联开发者应用） |
| remark | string | 否 | 备注（最长 128 字符，写入卡密 remark 字段） |

**请求示例**：

```json
{
  "card_type_id": 12,
  "count": 10,
  "app_id": 1,
  "remark": "发卡网补货-订单 #20260720001"
}
```

**响应示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "cards": [
      {
        "card_id": 10001,
        "card_key": "K2X9-AB7C-MN4P-QR8S",
        "card_type": "monthly",
        "duration_seconds": 2592000,
        "expires_at": "2026-08-19 10:30:00"
      },
      {
        "card_id": 10002,
        "card_key": "L3Y0-BC8D-NO5Q-RS9T",
        "card_type": "monthly",
        "duration_seconds": 2592000,
        "expires_at": "2026-08-19 10:30:00"
      }
    ],
    "total": 10,
    "batch_id": "batch_20260720103000_abc123"
  },
  "signature": "RSA-4096 签名(base64)",
  "request_id": "req-uuid-xxx",
  "timestamp": 1721374800
}
```

**响应签名校验**：

响应 `data` 字段 JSON 序列化后用本项目公钥做 RSA-4096 验签，公钥可在本项目后台「系统配置」页面下载。

#### 10.3.3 查询卡类列表

**GET** `/api/v1/openapi/card_types/list`

**权限**：`card_types:list`

**查询参数**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| app_id | int | 是 | 应用 ID |
| page | int | 否 | 页码（默认 1） |
| page_size | int | 否 | 每页数量（默认 20，最大 100） |

**响应示例**：

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "card_type_id": 12,
        "name": "月度会员卡",
        "type": "monthly",
        "price": 30.00,
        "duration_seconds": 2592000,
        "max_devices": 1,
        "status": "active"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

#### 10.3.4 查询卡密列表

**GET** `/api/v1/openapi/cards/list`

**权限**：`cards:list`

**查询参数**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| card_type_id | int | 否 | 卡类 ID |
| status | string | 否 | 状态筛选（unused/active/expired/banned/disabled） |
| batch_id | string | 否 | 批次 ID（来自生成接口返回） |
| start_time | int | 否 | 创建时间起（Unix 时间戳） |
| end_time | int | 否 | 创建时间止 |
| page | int | 否 | 页码 |
| page_size | int | 否 | 每页数量 |

#### 10.3.5 封禁卡密

**POST** `/api/v1/openapi/cards/ban`

**权限**：`cards:ban`

**请求参数**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| card_id | int | 是 | 卡密 ID |
| reason | string | 是 | 封禁原因（最长 256 字符） |

### 10.4 错误码

| 错误码 | 含义 | 处理建议 |
|---|---|---|
| 0 | 成功 | - |
| 1001 | 参数错误 | 检查请求参数 |
| 1002 | 未授权 | 检查 Token 是否正确 |
| 1003 | 禁止访问 | 权限不足或 Token 已吊销 |
| 1004 | 资源不存在 | 卡类/卡密不存在 |
| 1005 | 请求过于频繁 | 触发 QPS 限制，等待后重试 |
| 1006 | 服务器内部错误 | 联系 KeyAuth 管理员 |
| 2002 | Token 无效 | 检查 Token 是否正确 |
| 2003 | Token 已过期 | 重新生成 Token |
| 3001 | 卡密不存在 | 检查 card_id |
| 3002 | 卡类不存在 | 检查 card_type_id |
| 3003 | 卡类已下架 | 联系开发者/代理 |
| 3004 | 数量超限 | 单次生成数量超限 |
| 5004 | 余额不足 | 代理余额不足，请充值 |
| 6001 | 签名校验失败 | 检查签名算法是否正确 |
| 6002 | 时间戳过期 | 检查服务器时间是否同步 |
| 6003 | Nonce 重复 | 重新生成 nonce |

### 10.5 SDK 示例

#### 10.5.1 curl

```bash
#!/bin/bash
API_TOKEN="tk_abc123def456"
TIMESTAMP=$(date +%s)
NONCE=$(head -c 16 /dev/urandom | xxd -p)
BODY='{"card_type_id":12,"count":10,"app_id":1}'
PATH_URL="/api/v1/openapi/cards/generate"

# 拼接签名原文
SIGN_STR="POST\n${PATH_URL}\n${TIMESTAMP}\n${NONCE}\n${BODY}"

# 计算 HMAC-SHA256 签名
SIGNATURE=$(echo -ne "${SIGN_STR}" | openssl dgst -sha256 -hmac "${API_TOKEN}" -hex | awk '{print $2}')

curl -X POST "https://keyauth.example.com${PATH_URL}" \
  -H "Content-Type: application/json" \
  -H "X-Api-Token: ${API_TOKEN}" \
  -H "X-Timestamp: ${TIMESTAMP}" \
  -H "X-Nonce: ${NONCE}" \
  -H "X-Signature: ${SIGNATURE}" \
  -d "${BODY}"
```

#### 10.5.2 PHP

```php
<?php
class KeyAuthClient {
    private $endpoint;
    private $apiToken;
    
    public function __construct($endpoint, $apiToken) {
        $this->endpoint = rtrim($endpoint, '/');
        $this->apiToken = $apiToken;
    }
    
    public function generateCards($cardTypeId, $count, $appId = null, $remark = '') {
        $path = '/api/v1/openapi/cards/generate';
        $body = json_encode([
            'card_type_id' => $cardTypeId,
            'count' => $count,
            'app_id' => $appId,
            'remark' => $remark,
        ]);
        return $this->request('POST', $path, $body);
    }
    
    private function request($method, $path, $body = '') {
        $timestamp = time();
        $nonce = bin2hex(random_bytes(8));
        
        $signStr = implode("\n", [$method, $path, $timestamp, $nonce, $body]);
        $signature = hash_hmac('sha256', $signStr, $this->apiToken);
        
        $ch = curl_init($this->endpoint . $path);
        curl_setopt_array($ch, [
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_CUSTOMREQUEST => $method,
            CURLOPT_POSTFIELDS => $body,
            CURLOPT_HTTPHEADER => [
                'Content-Type: application/json',
                'X-Api-Token: ' . $this->apiToken,
                'X-Timestamp: ' . $timestamp,
                'X-Nonce: ' . $nonce,
                'X-Signature: ' . $signature,
            ],
            CURLOPT_TIMEOUT => 30,
            CURLOPT_SSL_VERIFYPEER => true,
        ]);
        
        $response = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        $error = curl_error($ch);
        curl_close($ch);
        
        if ($error) {
            throw new RuntimeException("cURL error: $error");
        }
        if ($httpCode !== 200) {
            throw new RuntimeException("HTTP $httpCode: $response");
        }
        
        $data = json_decode($response, true);
        if ($data['code'] !== 0) {
            throw new RuntimeException("API error [{$data['code']}]: {$data['message']}");
        }
        
        return $data['data'];
    }
}

// 使用示例
$client = new KeyAuthClient('https://keyauth.example.com', 'tk_abc123def456');
$result = $client->generateCards(12, 10, 1, '发卡网补货');
print_r($result['cards']);
```

#### 10.5.3 Python

```python
import time
import hmac
import hashlib
import secrets
import json
import requests

class KeyAuthClient:
    def __init__(self, endpoint: str, api_token: str):
        self.endpoint = endpoint.rstrip('/')
        self.api_token = api_token
        self.session = requests.Session()
    
    def generate_cards(self, card_type_id: int, count: int, 
                       app_id: int = None, remark: str = '') -> dict:
        path = '/api/v1/openapi/cards/generate'
        body_dict = {
            'card_type_id': card_type_id,
            'count': count,
        }
        if app_id is not None:
            body_dict['app_id'] = app_id
        if remark:
            body_dict['remark'] = remark
        return self._request('POST', path, body_dict)
    
    def _request(self, method: str, path: str, body_dict: dict = None) -> dict:
        body = json.dumps(body_dict) if body_dict else ''
        timestamp = str(int(time.time()))
        nonce = secrets.token_hex(8)
        
        sign_str = '\n'.join([method, path, timestamp, nonce, body])
        signature = hmac.new(
            self.api_token.encode(),
            sign_str.encode(),
            hashlib.sha256
        ).hexdigest()
        
        headers = {
            'Content-Type': 'application/json',
            'X-Api-Token': self.api_token,
            'X-Timestamp': timestamp,
            'X-Nonce': nonce,
            'X-Signature': signature,
        }
        
        resp = self.session.request(
            method,
            self.endpoint + path,
            data=body,
            headers=headers,
            timeout=30,
        )
        resp.raise_for_status()
        data = resp.json()
        if data['code'] != 0:
            raise RuntimeError(f"API error [{data['code']}]: {data['message']}")
        return data['data']

# 使用示例
client = KeyAuthClient('https://keyauth.example.com', 'tk_abc123def456')
result = client.generate_cards(card_type_id=12, count=10, app_id=1, remark='发卡网补货')
for card in result['cards']:
    print(card['card_key'])
```

#### 10.5.4 Node.js

```javascript
const crypto = require('crypto');
const axios = require('axios');

class KeyAuthClient {
  constructor(endpoint, apiToken) {
    this.endpoint = endpoint.replace(/\/$/, '');
    this.apiToken = apiToken;
    this.client = axios.create({ timeout: 30000 });
  }

  async generateCards({ cardTypeId, count, appId = null, remark = '' }) {
    const path = '/api/v1/openapi/cards/generate';
    const body = JSON.stringify({
      card_type_id: cardTypeId,
      count,
      app_id: appId,
      remark,
    });
    return this._request('POST', path, body);
  }

  async _request(method, path, body = '') {
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const nonce = crypto.randomBytes(8).toString('hex');
    const signStr = [method, path, timestamp, nonce, body].join('\n');
    const signature = crypto.createHmac('sha256', this.apiToken)
      .update(signStr).digest('hex');

    const headers = {
      'Content-Type': 'application/json',
      'X-Api-Token': this.apiToken,
      'X-Timestamp': timestamp,
      'X-Nonce': nonce,
      'X-Signature': signature,
    };

    const resp = await this.client.request({
      method,
      url: this.endpoint + path,
      data: body,
      headers,
    });
    const data = resp.data;
    if (data.code !== 0) {
      throw new Error(`API error [${data.code}]: ${data.message}`);
    }
    return data.data;
  }
}

// 使用示例
(async () => {
  const client = new KeyAuthClient('https://keyauth.example.com', 'tk_abc123def456');
  const result = await client.generateCards({
    cardTypeId: 12,
    count: 10,
    appId: 1,
    remark: '发卡网补货',
  });
  console.log(result.cards.map(c => c.card_key));
})();
```

### 10.6 对接流程清单

另一项目开发时按以下顺序对接：

- [ ] Step 1：在本项目后台创建开发者/代理账号 + 生成 API Token（权限勾选 `cards:generate` + `card_types:list`）
- [ ] Step 2：另一项目实现 `KeyAuthClient` 类（参考 SDK 示例）
- [ ] Step 3：实现「连通性测试」接口（调用 `GET /api/v1/openapi/ping`，返回 200 即成功）
- [ ] Step 4：实现「拉取卡类列表」功能（调用 `card_types:list` 接口，存入本地映射表）
- [ ] Step 5：实现「一键补货」功能（调用 `cards/generate` 接口，卡密明文存入本地 `cards` 表）
- [ ] Step 6：实现「订单购买」流程（从本地 `cards` 表取未售卡密 + 标记 sold）
- [ ] Step 7：实现「库存预警 + 自动补货」（库存低于阈值时自动调用 `cards/generate`）
- [ ] Step 8：实现「对账脚本」（每日比对两平台订单数 + 卡密流向）
- [ ] Step 9：实现「Token 吊销」UI（吊销后另一平台补货接口返回 1003）
- [ ] Step 10：联调 + 上线

### 10.7 限流与配额

| 限制项 | 默认值 | 可配置 |
|---|---|---|
| 单 Token QPS | 10/s | 是（Token 创建时设置，最大 100） |
| 单 Token 日调用次数 | 10000 | 是（sys_config） |
| 单次生成卡密数量 | 1-100 | 是（`external.api.max_push_count`） |
| 单 Token 日生成卡密总数 | 5000 | 是（`external.api.daily_card_limit`） |
| Nonce 防重放窗口 | 300 秒 | 否 |

### 10.8 联调与上线

**测试环境**：

- 沙箱环境：`https://sandbox.keyauth.example.com`
- 沙箱 Token：`tk_sandbox_test_token_xxx`（不限流，仅用于联调）
- 测试卡类 ID：1（月卡，price=0）

**联系方式**：

- 技术支持：`keyauth@example.com`
- 紧急故障：本项目后台「系统监控」会自动告警
- 文档版本：v1.0（2026-07-20）

---

## 附录 A：与本项目现有 OpenAPI 平台的关系

本项目 v0.4.0 第十五项迁移已实现 OpenAPI 平台骨架（Token 管理 + webhook + whoami），本方案在此基础上的增量改动：

| 维度 | v0.4.0 现状 | 本方案增量 |
|---|---|---|
| Token 表 | `developer_api_token`（仅 tenant_id） | 新增 owner_type / owner_id 字段支持 agent |
| 鉴权中间件 | `APITokenAuth`（已实现） | 无需改动 |
| 业务端点 | 仅 `/openapi/whoami` | 新增 5 个端点（ping / card_types / cards generate / cards list / cards ban） |
| 限流 | 无 | 新增 Token 级 QPS 限流 |
| 调用日志 | 无 | 新增 `external_api_call_log` 表 |
| Webhook | 已实现 | 不变（与本方案无关） |

## 附录 B：启动实现时的 spec 入口

当本方案正式启动实现时，按以下步骤走 spec 流程：

1. 删除 `/workspace/.trae/specs/integrate-external-card-platform/`（上次取消的旧 spec，方向不同）
2. 创建新 change-id：`openapi-cards-supply-endpoints`
3. spec.md 引用本文档作为需求来源
4. tasks.md 拆分 migration 020 + 5 个 OpenAPI 端点 + 限流 + 日志 + 测试 + 文档同步
5. checklist.md 对应本文档第 0.2 节「待新增能力」清单

---

**文档版本**：v1.0
**最后更新**：2026-07-20
**维护者**：KeyAuth SaaS Team
