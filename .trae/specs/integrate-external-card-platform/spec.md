# 对接外部发卡网平台 Spec

## Why
本项目（KeyAuth SaaS）作为卡密供应方，需要让开发者和代理能够将自己在本项目生成的卡密一键上架到另一个「企业多商户发卡网平台」对应的商品下，打通卡密生成 → 跨平台销售的链路，避免手工导出卡密再粘贴上架的重复劳动。

## What Changes
- 新增「外部平台绑定」能力：开发者（tenant）和代理（agent）可绑定对方平台凭证（API 地址 + API Token + 商户 ID），AES-256-GCM 加密存储 Token
- 新增「商品映射」能力：将本项目 `app_card_type` 与外部平台商品建立绑定关系（一对多），支持自动推送开关
- 新增「一键生成卡密并上架」能力：在选定映射下输入数量，本项目事务化生成卡密 → 调用外部平台「上架卡密」API → 写推送日志
- 新增「商品列表拉取」能力：通过绑定凭证调用外部平台「查询商户商品列表」API，便于在创建映射时下拉选择
- 新增「推送日志 + 失败重试」能力：每次推送落盘记录，失败可手动重试（重试只针对未上架成功的卡密）
- 新增「平台适配器抽象」：`PlatformAdapter` 接口，第一版实现通用 HTTP JSON 适配器，预留多平台扩展点（不同发卡网 API 协议不同）
- 新增 admin 端总览：admin 可查看全平台所有绑定/推送日志（只读 + 禁用），不参与业务操作
- 新增 8 项 sys_config（铁律 05）：超时、重试、批量大小、SSL 校验等
- **BREAKING**：无（纯新增能力，不改现有接口）

## Impact
- Affected specs:
  - `SPEC.md` 2.2 模块边界（新增 `external_platform` 包 + handler 文件）
  - `SPEC.md` 4.1 表清单（新增 3 张表）
  - `SPEC.md` 7.x 新增「7.8 外部发卡网平台对接」章节
  - `PROJECT.md` 3.1/3.2/3.3 三角色模块清单各新增 1 项
  - `PROMPT.md` v0.4.0 进度新增第十七项迁移
  - `CHANGELOG.md` 新增第十七项迁移
  - `TODO.md` 新增对接相关任务
- Affected code:
  - 新建 `migrations/020_v0.4.0_external_card_platform.up.sql` / `.down.sql`
  - 新建 `internal/externalplatform/` 包（adapter + manager）
  - 新建 `internal/handler/external_platform.go`（admin/tenant/agent 三端 handler）
  - 新建 `internal/handler/external_platform_test.go`
  - 修改 `internal/model/model.go`（新增 3 个 struct）
  - 修改 `internal/router/router.go`（注册新路由）
  - 修改 `internal/handler/deps.go`（Deps 注入 ExternalPlatformMgr）
  - 复用 `pkg/crypto` AES-256-GCM（Token 加密）
  - 复用 `internal/handler/card.go` 卡密生成逻辑（提取可复用函数或直接调用）

## ADDED Requirements

### Requirement: 外部平台绑定管理
系统 SHALL 提供开发者（tenant）和代理（agent）绑定外部发卡网平台凭证的能力，每个用户可绑定多个平台账号，凭证中 API Token 使用 AES-256-GCM 加密存储。

#### Scenario: 开发者成功创建绑定
- **WHEN** 开发者提交 `{platform_type, nickname, api_endpoint, api_token, merchant_id}` 创建绑定
- **THEN** 系统校验 api_endpoint 为 https URL + api_token 非空 + 同用户下 nickname 唯一
- **AND** 使用 AES-256-GCM 加密 api_token 写入 `external_card_platform_binding.api_token_enc`
- **AND** 返回绑定 ID + 脱敏的 api_token（仅前 4 后 4 字符）

#### Scenario: 代理创建绑定
- **WHEN** 代理提交绑定信息
- **THEN** 同上流程，user_type=agent，user_id 为当前代理 ID

#### Scenario: 同一昵称重复创建
- **WHEN** 同一用户下已存在相同 nickname 的绑定
- **THEN** 返回 1001 错误「绑定昵称已存在」

#### Scenario: API endpoint 非 https
- **WHEN** api_endpoint 不以 `https://` 开头
- **THEN** 返回 1001 错误「API 地址必须为 https」

### Requirement: 商品映射管理
系统 SHALL 提供开发者/代理将本项目的 `app_card_type` 与外部平台商品建立映射的能力，一个卡类可映射到多个外部商品，一个外部商品只能映射一次（同绑定下唯一）。

#### Scenario: 创建商品映射
- **WHEN** 用户提交 `{binding_id, card_type_id, external_product_id, external_product_name, auto_push_enabled}` 创建映射
- **THEN** 校验 binding 归属当前用户 + card_type 归属当前用户（开发者校验 tenant_id，代理校验可售卡类）
- **AND** 校验同 binding_id 下 external_product_id 唯一
- **AND** 写入 `external_card_product_mapping`

#### Scenario: 同 binding 下重复商品 ID
- **WHEN** 同一绑定下 external_product_id 已存在映射
- **THEN** 返回 1001 错误「该外部商品已建立映射」

#### Scenario: 跨用户访问映射
- **WHEN** 用户 A 尝试访问/操作用户 B 的映射
- **THEN** 返回 1004 错误「映射不存在」

### Requirement: 拉取外部平台商品列表
系统 SHALL 提供通过绑定凭证调用外部平台「查询商户商品列表」API 的能力，用于创建映射时下拉选择。

#### Scenario: 拉取成功
- **WHEN** 用户提交 binding_id 请求拉取商品列表
- **THEN** 系统解密 api_token + 调用外部平台 API（带超时 + 重试）
- **AND** 返回商品列表 `[{product_id, product_name, price, stock}]`

#### Scenario: 拉取失败
- **WHEN** 外部平台 API 返回非 2xx 或网络错误
- **THEN** 返回 5002 错误「拉取外部商品失败」+ 详细 error_message

### Requirement: 一键生成卡密并推送
系统 SHALL 提供开发者/代理在选定映射下输入数量，本项目事务化生成卡密并推送到外部平台对应商品的能力。

#### Scenario: 开发者一键推送成功
- **WHEN** 开发者提交 `{mapping_id, count}` 触发推送
- **THEN** 校验 mapping 归属 + card_type 归属 + 套餐配额（quota.CheckMaxCards）
- **AND** 事务内：生成 count 张卡密（复用现有卡密生成逻辑）+ 写 `app_card` 表 + 写 `external_card_push_log` pending 记录
- **AND** 事务外：调用外部平台「上架卡密」API（批量提交卡密明文 + external_product_id）
- **AND** 成功：更新 push_log status=success + external_order_id
- **AND** 失败：更新 push_log status=failed + error_message（卡密已生成不回滚，可重试推送）

#### Scenario: 代理一键推送
- **WHEN** 代理提交 `{mapping_id, count}` 触发推送
- **THEN** 同上流程，但额外执行代理余额扣款 + 佣金计算（复用 AgentGenerateCards 事务逻辑）
- **AND** 余额不足时返回 5004 错误「余额不足」

#### Scenario: 数量超限
- **WHEN** count > `external.platform.max_push_count`（sys_config 上限）
- **THEN** 返回 1001 错误「单次推送数量超限」

#### Scenario: 外部平台 API 失败
- **WHEN** 外部平台返回非 2xx
- **THEN** 卡密保留在 `app_card` 表（status=unused）+ push_log 标记 failed + 返回 5002 错误
- **AND** 用户可后续通过重试 API 重新推送这批卡密

### Requirement: 推送日志查询和重试
系统 SHALL 提供推送日志查询和失败重试能力。

#### Scenario: 查询推送日志
- **WHEN** 用户请求推送日志列表
- **THEN** 返回分页日志列表，支持 status / mapping_id / 时间区间筛选
- **AND** 每条日志包含 card_ids（卡密 ID 列表）+ external_order_id + error_message + retry_count

#### Scenario: 重试失败推送
- **WHEN** 用户对 status=failed 的推送日志触发重试
- **THEN** 系统重新调用外部平台 API 推送相同卡密
- **AND** 成功：status=success + retry_count++
- **AND** 失败：status=failed + retry_count++ + 更新 error_message

#### Scenario: 重试已成功的推送
- **WHEN** 用户对 status=success 的推送日志触发重试
- **THEN** 返回 1001 错误「仅失败状态可重试」

### Requirement: admin 端总览
系统 SHALL 提供 admin 端只读总览能力，查看全平台所有绑定和推送日志。

#### Scenario: admin 查看绑定列表
- **WHEN** admin 请求 /admin/external_platform/bindings
- **THEN** 返回全平台所有绑定（不脱敏 api_token_enc，但脱敏 api_token）
- **AND** 支持按 user_type / platform_type / status 筛选

#### Scenario: admin 禁用绑定
- **WHEN** admin 禁用某个绑定
- **THEN** 该绑定 status=disabled + 后续推送/拉取操作返回 1003 错误「绑定已禁用」

### Requirement: 平台适配器抽象
系统 SHALL 提供平台适配器抽象，第一版实现通用 HTTP JSON 适配器，预留多平台扩展点。

#### Scenario: 通用 HTTP JSON 适配器
- **WHEN** platform_type=generic_http
- **THEN** 使用 `GenericHTTPAdapter` 调用外部 API
- **AND** 支持配置：拉取商品路径 + 上架卡密路径 + 鉴权方式（header/query）+ 字段映射

#### Scenario: 扩展新平台
- **WHEN** 未来需要对接特定发卡网平台（如卡盟/卡库）
- **THEN** 实现 `PlatformAdapter` 接口新增适配器，不影响现有逻辑

## MODIFIED Requirements

### Requirement: 数据库表清单
在原表清单基础上新增 3 张表：
- `external_card_platform_binding`（id/user_type/user_id/platform_type/nickname/api_endpoint/api_token_enc/merchant_id/status/last_synced_at/created_at/updated_at）
- `external_card_product_mapping`（id/binding_id/user_type/user_id/card_type_id/external_product_id/external_product_name/auto_push_enabled/push_count/last_push_at/created_at/updated_at）
- `external_card_push_log`（id/mapping_id/user_type/user_id/card_type_id/external_product_id/card_ids_json/external_order_id/status/error_message/retry_count/max_retry/created_at）

### Requirement: sys_config 配置项
新增 8 项 `external.platform.*` sys_config：
- `external.platform.timeout_seconds`=30 HTTP 调用超时
- `external.platform.retry.times`=3 失败重试次数
- `external.platform.retry.interval_seconds`=60 重试间隔
- `external.platform.verify_ssl`=1 SSL 证书校验
- `external.platform.max_push_count`=100 单次推送数量上限
- `external.platform.default_push_batch_size`=50 默认批量大小
- `external.platform.push_concurrency`=5 推送并发数
- `external.platform.token_encryption_key` AES-256-GCM 密钥（空=使用全局 AES_KEY）

## REMOVED Requirements
无（纯新增能力）
