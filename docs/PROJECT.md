# 项目文档 (PROJECT)

## 1. 项目概述

### 1.1 项目名称
**KeyAuth SaaS** —— 面向开发者的多租户卡密验证平台

### 1.2 项目目标
为开发者提供自托管 SaaS 卡密验证基础设施，一次部署即可服务多个软件应用，支持多租户隔离、一机一卡密、在线验证、心跳保活、代理分销、自动发卡等完整能力。

### 1.3 项目背景
参考卡密通、T3 网络验证、布丁卡密三家之长，结合 License-SaaS 开源项目的技术栈选型，打造完全开源自托管、数据自主可控的卡密验证 SaaS 平台。

### 1.4 适用范围
- 独立开发者为自己开发的软件加授权
- 软件工作室商业化运营软件
- 中小型 SaaS 团队多应用统一授权管理
- 卡密分销代理商体系运营

### 1.5 对标产品

| 产品 | 借鉴点 |
|---|---|
| [卡密通 keyt.cn](https://www.keyt.cn/) | 极简三步接入、应用隔离、心跳保活、数据看板 |
| [T3 网络验证 t3yanzheng.com](https://t3yanzheng.com/) | 双向加密传输、独立接口链接、多级代理体系 |
| [布丁卡密 wmxwz.cn](https://wmxwz.cn/) | 硬件指纹双模式、七层纵深防御、SHA-512 校验位防伪 |
| [License-SaaS 开源项目](https://bbs.ijingyi.com/forum.php?mod=viewthread&tid=14876646) | Go + Vue3 技术栈、单端口部署、HMAC-SHA256 签名协议、云变量机制 |

### 1.6 差异化优势
- 完全开源自托管，数据自主可控
- 多应用 + 多租户架构
- 一机一卡密强绑定（CPU+主板+MAC+磁盘多重哈希）
- 双层支付模式（平台总支付 / 开发者自有易支付）
- 多级代理分销体系（v0.4.0 三级代理 + 跨级佣金自动分润）
- 灰度发布体系（v0.4.0 三策略 + Hash 桶稳定匹配 + 平台/渠道/地区白名单）
- 在线更新体系（v0.4.0 GitHub Webhook 自动部署 + 双重锁防并发 + 失败自动回滚 + 完整审计日志）
- 数据备份恢复体系（v0.4.0 全库 SQL 备份 + SHA-256 校验 + AES-256-GCM 加密 + gzip 压缩 + 异步恢复 + 过期清理）
- 监控告警体系（v0.4.0 CPU/内存/磁盘/错误率采集 + 阈值告警 + webhook 通知 + 静默期去重 + 自动恢复 + 告警确认）
- 心跳保活 + 离线宽限期
- 云变量远程下发
- 多语言 SDK（Python / Node.js / Java / C# / Go / PHP / C++ / 易语言）
- Docker 一键部署（同时支持宝塔 Docker + 原生 Docker Compose）

---

## 2. 架构总览

### 2.1 系统架构图

```
                    ┌─────────────────────────────────┐
                    │   Cloudflare CDN + WAF + SSL    │
                    └────────────┬────────────────────┘
                                 │
                    ┌────────────▼────────────────────┐
                    │   Nginx (反向代理 + 限流)        │
                    └────────────┬────────────────────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                        │
┌───────▼────────┐    ┌──────────▼──────────┐   ┌─────────▼─────────┐
│  Vue3 管理后台  │    │  Go Gin API 服务    │   │  H5 移动端         │
│  (超管/开发者/  │    │  (RESTful + HMAC)   │   │  (终端用户)        │
│   代理自适应)   │    │                     │   │                    │
└────────────────┘    └──────────┬──────────┘   └────────────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
       ┌──────▼──────┐    ┌──────▼──────┐   ┌──────▼──────┐
       │  MySQL 8.0  │    │  Redis 7    │   │  MinIO/OSS  │
       │  (主从可选) │    │  (缓存/限流)│   │  (对象存储) │
       └─────────────┘    └─────────────┘   └─────────────┘
```

### 2.2 技术栈

| 层级 | 选型 |
|---|---|
| 前端 | Vue3 + TypeScript + Element Plus + Vite + Pinia |
| 后端 | Go 1.22 + Gin + GORM |
| 数据库 | MySQL 8.0 + Redis 7 |
| 部署 | Docker Compose + 宝塔面板 Docker |
| 反代 | Nginx + Let's Encrypt |
| 监控 | Prometheus + Grafana（可选） |
| 日志 | Loki + Promtail（可选） |

### 2.3 核心模块依赖关系

```
认证模块 ──► 租户模块 ──► 应用模块 ──► 卡密模块
                │              │            │
                ▼              ▼            ▼
            套餐模块      设备模块      订单模块
                │              │            │
                ▼              ▼            ▼
            支付模块      验证日志      代理模块
                │                          │
                ▼                          ▼
            通知模块                  佣金结算
```

### 2.4 数据流（核心验证流程）

```
客户端启动
  │
  ▼
SDK 调用 /api/v1/login（卡密 + 机器码 + 签名）
  │
  ▼
Gin 接收请求 → 中间件验签 → 限流检查 → 租户路由
  │
  ▼
查 Redis 缓存 → 命中返回 → 未命中查 MySQL
  │
  ▼
校验卡密状态 → 校验设备绑定 → 写入/更新设备表
  │
  ▼
写入验证日志 → 返回带 RSA 签名的响应
  │
  ▼
SDK 校验签名 → 通过则解锁功能
  │
  ▼
后台定时心跳 /api/v1/heartbeat（保活）
```

---

## 3. 功能清单

### 3.1 平台超管后台（17 个模块）

| 编号 | 模块 | 已实现 | 说明 |
|---|---|---|---|
| S-01 | 平台看板 | ✅ | 8 数据卡 + 待办列表 + 收入趋势 + 最近开发者/订单表（v0.2.6） |
| S-02 | 租户管理 | ✅ | 开发者 CRUD + 状态/套餐分配（v0.3.0） |
| S-03 | 套餐管理 | ✅ | 套餐 CRUD + 应用数/卡密数/代理数上限 + 抽成比例（v0.3.0） |
| S-04 | 应用审核 | ☐ | 应用上架审核、违规下架（v0.4.x） |
| S-05 | 代理全局视图 | ✅ | 跨租户代理列表 + 状态/封禁（v0.3.0） |
| S-06 | 平台总支付配置 | ✅ | sys_config 易支付网关/商户号/密钥 + 抽成比例 + TestPayConfig（v0.2.3） |
| S-07 | 系统配置 | ✅ | sys_config 全局参数 CRUD + Redis 缓存（v0.2.0） |
| S-08 | 通知模板 | ☐ | 短信/邮件/站内信模板（v0.4.x） |
| S-09 | 安全防护 | ✅ | 全局 IP 黑名单 + 登录失败日志 + 安全中心统计（v0.3.1） |
| S-10 | 操作日志 | ✅ | 三表独立查询（操作/验证/登录失败）+ CSV 导出（v0.3.3） |
| S-11 | 系统监控 | ✅ | CPU/内存/磁盘/在线设备/QPS/错误率采集 + 阈值告警 + webhook 通知 + 静默期 + 自动恢复 + 告警确认/重发 + 24h 聚合（v0.4.0） |
| S-12 | 数据备份 | ✅ | 全库 SQL 备份 + SHA-256 校验 + AES-256-GCM 加密 + gzip 压缩 + 异步恢复 + 下载校验 + 过期清理（v0.4.0） |
| S-13 | 更新管理 | ✅ | GitHub Webhook 自动部署 + 手动触发 + 失败自动回滚 + 完整审计日志 + 双重锁防并发（v0.4.0） |
| S-14 | 管理员管理 | ✅ | 超管账号 + 2FA TOTP + 登录设备管理（v0.3.0）+ 安装向导首次配置（v0.3.6 `/install`） |
| S-15 | 平台总公告管理 | ✅ | 公告 CRUD + 横幅开关 + 公开 API（v0.3.0） |
| S-16 | 开发者公告管理 | ✅ | 公告 CRUD（v0.3.0） |
| S-17 | 代理注册管理 | ◐ | 注册订单流程已实现（v0.3.6：AgentRegister + processAgentRegisterPaid + 邀请码状态机闭环），收入统计/退款待 v0.4.x |

### 3.2 开发者控制台（19 个模块）

| 编号 | 模块 | 已实现 | 说明 |
|---|---|---|---|
| D-01 | 工作台 | ✅ | 8 数据卡 + 8 快捷入口 + 收入趋势 + 应用 TOP5 + 最近订单（v0.2.6） |
| D-02 | 应用管理 | ✅ | 应用 CRUD + 密钥生成/轮换（保留旧 SignSecret 7 天）（v0.2.2） |
| D-03 | 卡密管理 | ✅ | 批量生成 + 状态机 + 封禁/解封/删除（v0.2.2）+ CSV 导入导出（v0.3.6） |
| D-04 | 卡类套餐 | ✅ | 时长卡/次数卡/永久卡/试用卡/功能解锁卡 5 类型 CRUD（v0.2.2） |
| D-05 | 设备管理 | ✅ | 列表 + 强制下线（v0.3.0）+ 封禁卡密联动下线（v0.3.6） |
| D-06 | 用户管理 | ☐ | 终端用户列表、封禁（v0.4.x，终端用户体系未建） |
| D-07 | 订单管理 | ✅ | 订单列表 + 状态筛选（v0.3.0） |
| D-08 | 代理管理 | ✅ | 代理 CRUD + 邀请码生成/禁用 + 套餐配额校验（v0.3.0 + v0.3.5） |
| D-09 | 云变量 | ✅ | 变量 CRUD + 客户端 get_var 接口（v0.3.0） |
| D-10 | 公告管理 | ✅ | 应用公告 CRUD（v0.3.0） |
| D-11 | 版本管理 | ✅ | 版本号/最低版本/下载地址/强制更新 CRUD + 客户端 version 接口（v0.3.0） |
| D-12 | 验证日志 | ✅ | log_verify 按月分区 + 异步 worker（v0.3.3） |
| D-13 | 操作日志 | ✅ | log_operation + 切面 RecordOperation（v0.3.3） |
| D-14 | 财务统计 | ✅ | 结算记录 + 余额流水 + 提现申请 + 提现审核（v0.3.2 + v0.3.4） |
| D-15 | 安全设置 | ☐ | IP 黑名单、频率限制（v0.4.x，目前仅超管侧） |
| D-16 | SDK 下载 | ✅ | 八语言 SDK 已发布（Python / Node.js / PHP / Go / Java / C# / C++ / 易语言，`sdks/` 目录）+ 单元测试 + 跨语言签名对齐测试（`sdks/tests/` + `pkg/crypto/sign_alignment_test.go` 7 语言自动化 + 1 语言 Windows-only skip） |
| D-17 | 开发者设置 | ✅ | 资料 + 公司信息 + 密码 + 2FA + 登录设备（v0.3.0） |
| D-18 | 支付配置 | ✅ | 双层模式切换（平台总支付 / 自有易支付）（v0.3.6：CreatePayOrder 双层切换 + TOP/ORD 前缀分发 + EpayTenantNotify 完整实现） |
| D-19 | 代理充值审核 | ✅ | 充值申请列表 + 批准/驳回 + 实际到账金额调整（v0.3.2） |

### 3.3 代理商控制台（10 个模块）

| 编号 | 模块 | 已实现 | 说明 |
|---|---|---|---|
| P-01 | 工作台 | ✅ | 4 数据卡 + 4 快捷入口 + 最近订单（v0.2.5） |
| P-02 | 卡密库存 | ✅ | 可售卡类 + 余额扣款生成卡密 + 佣金计算（v0.2.5 + v0.3.0 事务化） |
| P-03 | 卡密管理 | ✅ | 自己生成的卡密列表（v0.2.5） |
| P-04 | 销售订单 | ✅ | 自售订单 + 状态筛选 + 分页（v0.2.5） |
| P-05 | 佣金结算 | ✅ | 佣金明细 + 提现申请 + 双模式（percentage/diff）（v0.2.5） |
| P-06 | 独立门户 | ☐ | 代理专属购卡页 + 子域名绑定（v0.4.x） |
| P-07 | 代理设置 | ✅ | 资料 + 提现账户 + 密码 + 2FA + 登录设备（v0.2.6 + v0.3.1） |
| P-08 | 公告中心 | ✅ | 平台 + 开发者通知列表 + 已读标记（v0.2.7） |
| P-09 | 余额充值 | ✅ | 充值申请 + 支付方式 + 凭证上传 + 待审核闭环（v0.3.1 + v0.3.2） |
| P-10 | 实时购卡 | ☐ | 扫码购卡（备用方式）（v0.4.x） |

### 3.4 终端用户 H5（14 个页面）

| 编号 | 页面 | 已实现 | 说明 |
|---|---|---|---|
| U-01 | 首页 | ✅ | H5 购卡首页（v0.2.4 + v0.3.5 接入公共 API） |
| U-02 | 应用详情页 | ✅ | PublicAppInfo 渲染（v0.3.5） |
| U-03 | 购卡结算页 | ✅ | PublicCardTypes + CreatePayOrder（v0.2.3 + v0.3.5） |
| U-04 | 支付结果页 | ✅ | GetPayOrder + 卡密明文展示（v0.2.4 + v0.3.5） |
| U-05 | 我的卡密 | ☐ | 终端用户体系未建（v0.4.x） |
| U-06 | 卡密详情 | ✅ | 卡密查询 + 详情（v0.2.4） |
| U-07 | 查卡页 | ✅ | 按 card_key 查询（v0.2.4） |
| U-08 | 在线激活页 | ☐ | 终端用户体系未建（v0.4.x） |
| U-09 | 用户登录/注册 | ☐ | 终端用户体系未建（v0.4.x） |
| U-10 | 用户中心 | ☐ | 终端用户体系未建（v0.4.x） |
| U-11 | 订单列表 | ☐ | 终端用户体系未建（v0.4.x） |
| U-12 | 公告详情 | ☐ | v0.4.x |
| U-13 | 帮助中心 | ☐ | v0.4.x |
| U-14 | 联系客服 | ☐ | v0.4.x |

### 3.5 客户端 SDK

| 语言 | 包名 | 已实现 | 说明 |
|---|---|---|---|
| Python | `keyauth-py` | ✅（v0.3.6） | `sdks/python/` 9 API + HMAC-SHA512/256 + KeyAuthError + CardInfo/DeviceInfo 数据类 |
| Node.js | `keyauth-node` | ✅（v0.3.6） | `sdks/nodejs/` 9 异步 API + crypto.createHmac('sha512/256') + index.d.ts 类型定义，无第三方依赖 |
| PHP | `keyauth-php` | ✅（v0.3.6） | `sdks/php/` 9 API + hash_hmac('sha512/256') + cURL，无第三方依赖，PSR-4 自动加载 |
| Go | `keyauth-go` | ✅（v0.4.0） | `sdks/go/` 9 API + `crypto/sha512.New512_256` 原生对齐 + 强类型 struct 返回 + 零第三方依赖 |
| Java | `keyauth-java` | ✅（v0.4.0） | `sdks/java/` 9 API + JDK 11+ HttpClient + `HmacSHA512/256`（JDK 17+，回退 HmacSHA256）+ Jackson + Maven 工程 |
| C# | `keyauth-csharp` | ✅（v0.4.0） | `sdks/csharp/` 9 异步 API + .NET 6+ HttpClient + 反射探测 BouncyCastle 启用 SHA-512/256 + System.Text.Json |
| C++ | `keyauth-cpp` | ✅（v0.4.0） | `sdks/cpp/` 9 API + libcurl + OpenSSL 1.1+ `EVP_sha512_256` 原生对齐 + nlohmann/json + CMake 工程 |
| 易语言 | `keyauth-epl` | ✅（v0.4.0） | `sdks/epl/` 9 API 纯中文 + 精易模块 v9.0+ 依赖 + HMAC-SHA256（易语言生态无 SHA-512/256，仅在后端回退场景匹配） |

> 八语言 SDK 均封装 9 个验证 API（login/verify/heartbeat/bind/unbind/get_var/notice/version/logout），签名算法与后端 `crypto.HMACSHA256`（`sha512.New512_256` 变体）对齐：Go / C++ 原生支持字节级对齐；Python / Node.js / PHP 优先 sha512/256 不支持时回退 sha256；Java / C# 反射探测 BouncyCastle 提供者启用 SHA-512/256 否则回退 HmacSHA256；易语言生态无 SHA-512/256 实现统一使用 HMAC-SHA256（与后端算法不同，仅在后端 crypto.go:165 待核实兼容性回退场景下匹配）。

---

## 4. 数据库设计

### 4.1 表清单（共 30 张表，对应 30 个 GORM struct）

| 分类 | 表名 | 说明 | 引入版本 |
|---|---|---|---|
| 平台 | `sys_admin` | 超管账号 | v0.2.0 |
| 平台 | `sys_config` | 系统配置（铁律 05） | v0.2.0 |
| 平台 | `sys_tenant` | 租户（开发者），含 balance/frozen_balance（v0.3.4） | v0.2.0 |
| 平台 | `sys_package` | 平台套餐定义 | v0.2.0 |
| 平台 | `tenant_pay_config` | 租户自有易支付配置 | v0.2.0 |
| 应用 | `app` | 开发者应用 | v0.2.0 |
| 应用 | `app_card_type` | 卡类套餐 | v0.2.0 |
| 应用 | `app_card` | 卡密 | v0.2.0 |
| 应用 | `app_device` | 设备绑定 | v0.2.0 |
| 应用 | `app_order` | 订单 | v0.2.0 |
| 应用 | `app_cloud_var` | 云变量 | v0.2.0 |
| 应用 | `app_version` | 应用版本 | v0.2.0 |
| 代理 | `agent` | 代理商账号 | v0.2.0 |
| 代理 | `agent_invite_code` | 代理邀请码 | v0.2.0 |
| 代理 | `agent_balance_log` | 代理余额流水 | v0.2.0 |
| 代理 | `agent_withdraw` | 代理提现记录 | v0.2.0 |
| 代理 | `agent_commission` | 佣金结算记录 | v0.2.0 |
| 代理 | `agent_registration_order` | 代理注册订单 | v0.2.0 |
| 公告 | `notice` | 统一公告表 | v0.2.0 |
| 公告 | `notice_target` | 公告精准投递 | v0.2.0 |
| 公告 | `notice_read` | 公告已读记录 | v0.2.0 |
| 安全 | `sec_ip_blacklist` | IP 黑名单 | v0.2.0 |
| 日志 | `log_verify` | 验证日志（按月分区） | v0.2.0 |
| 日志 | `log_operation` | 后台操作日志 | v0.2.0 |
| 平台 | `platform_settlement` | 平台抽成结算记录 | v0.2.3 |
| 安全 | `log_login_failed` | 登录失败日志（异步 worker 写入） | v0.3.1 |
| 安全 | `refresh_token_device` | 登录设备记录（用于 ListLoginDevices + KickDevice） | v0.3.1 |
| 平台 | `tenant_balance_log` | 开发者余额流水（settle/withdraw/refund/adjust） | v0.3.4 |
| 平台 | `tenant_withdraw` | 开发者提现申请 | v0.3.4 |
| 系统 | `schema_migrations` | 轻量级迁移机制版本跟踪（dirty 状态） | v0.3.5 |
| 安全 | `sys_admin.backup_codes` / `sys_tenant.backup_codes` / `agent.backup_codes` | v0.4.0 2FA 备用码 DB 持久化（AES 加密的逗号分隔字符串，migration 008） | v0.4.0 |
| 代理 | `agent.parent_id` / `agent.level` | v0.4.0 多级代理体系（parent_id 链 + level 1/2/3 层级，migration 009） | v0.4.0 |
| 代理 | `agent_invite_code.creator_type` / `agent_invite_code.creator_agent_id` | v0.4.0 邀请码创建者类型（tenant=开发者→一级 / agent=代理→creator.level+1，migration 009） | v0.4.0 |
| 应用版本 | `app_version.release_strategy` / `grayscale_rate` / `grayscale_platforms` / `grayscale_regions` / `grayscale_channels` | v0.4.0 灰度发布体系（full / grayscale / canary 三策略 + 平台/渠道/地区白名单 + Hash 桶比例匹配，migration 010） | v0.4.0 |
| 在线更新 | `system_update_log.trigger_source` / `trigger_by` / `commit_before` / `commit_after` / `status` / `steps_json` / `log_text` / `rolled_back_from` | v0.4.0 在线更新体系（GitHub Webhook + 自动部署 + 回滚 + 审计日志，migration 011） | v0.4.0 |
| 数据备份 | `system_backup_log.backup_type` / `file_path` / `file_size` / `checksum` / `status` / `tables_count` / `rows_count` / `restored_from` | v0.4.0 数据备份恢复（全库 SQL 备份 + SHA-256 + AES-256-GCM + gzip + 过期清理，migration 012） | v0.4.0 |
| 监控告警 | `system_metric.metric_name` / `metric_value` / `metric_unit` / `labels_json` / `collected_at` + `system_alert.alert_rule` / `severity` / `status` / `threshold` / `operator` / `fired_at` / `resolved_at` / `acked_by` / `notify_sent` | v0.4.0 监控告警（CPU/内存/磁盘/错误率采集 + 阈值告警 + webhook 通知 + 静默期 + 自动恢复，migration 013） | v0.4.0 |

> migration 文件：`apps/server/migrations/` 共 13 套（001 ~ 013），由 `internal/migration/migrator.go` 在 `InitContainer` 阶段自动执行。

### 4.2 Redis 缓存键设计

| Key 模式 | TTL | 用途 |
|---|---|---|
| `config:{key}` | 1h | sys_config 缓存 |
| `pay:config:tenant:{tenant_id}` | 10min | 租户支付配置缓存 |
| `card:verify:{card_key_hash}` | 60s | 卡密验证结果缓存 |
| `device:online:{device_id}` | 180s | 设备在线状态 |
| `heartbeat:{card_id}` | 180s | 心跳计数（Sorted Set） |
| `rate:verify:{ip}` | 60s | IP 限流计数 |
| `rate:login:{card_key}` | 60s | 卡密登录防爆破 |
| `lock:card:{card_id}` | 5s | 卡密操作分布式锁 |
| `nonce:{nonce}` | 5min | Nonce 防重放 |
| `pay:notify:lock:{order_no}` | 10s | 回调防重入锁 |
| `2fa:setup:{role}:{user_id}` | 10min | 2FA setup 中转（待 verify 前临时存 totp_secret） |
| `2fa:backup:{role}:{user_id}` | persistent | v0.3.x 2FA 备用码持久化；v0.4.0 改为 DB backup_codes 字段，此 key 仅作兼容回退读取（consumeBackupCode 消费后自动清理） |
| `login:fail:{username}` | 15min | 登录失败次数（达阈值锁定账号） |
| `auth:refresh:blacklist:jti:{jti}` | refresh_ttl | v0.4.0：jti 维度黑名单（精准单点踢出，KickDevice/Logout/RefreshToken 轮换） |
| `auth:refresh:blacklist:{role}:{user_id}` | refresh_ttl | user 维度黑名单（修改密码/关闭 2FA 强制所有设备重登） |

---

## 5. 使用指南

### 5.1 安装

#### 方式 1：Docker Compose 一键部署
```bash
git clone https://github.com/your-org/keyauth-saas.git
cd keyauth-saas
cp .env.example .env
# 编辑 .env 修改数据库密码、域名等
docker-compose up -d
```

#### 方式 2：宝塔面板 Docker 部署
1. 宝塔面板 → Docker → 应用商店 → 自定义 → 上传 compose 文件
2. 配置环境变量
3. 启动容器
4. 宝塔配置反向代理 + SSL 证书

### 5.2 配置

环境变量（写入 `.env`）：
```
# 数据库
MYSQL_HOST=mysql
MYSQL_PORT=3306
MYSQL_DATABASE=keyauth
MYSQL_USER=keyauth
MYSQL_PASSWORD=your-password
MYSQL_ROOT_PASSWORD=your-root-password

# Redis
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=

# 应用
APP_PORT=8080
APP_MODE=release
APP_JWT_SECRET=your-jwt-secret
APP_AES_KEY=your-aes-32-byte-key
APP_RSA_PRIVATE_KEY_PATH=/app/keys/private.pem
APP_RSA_PUBLIC_KEY_PATH=/app/keys/public.pem

# 域名
DOMAIN=yourdomain.com
```

### 5.3 运行

```bash
# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f api

# 停止
docker-compose down

# 升级
git pull
docker-compose up -d --build
```

### 5.4 初始化

1. 访问 `https://yourdomain.com/install`
2. 设置超管账号密码
3. 配置平台总支付（易支付网关/商户号/密钥）
4. 创建套餐（免费版/专业版/企业版）
5. 配置代理注册费用

### 5.5 示例（开发者接入流程）

```go
// Go SDK 示例
package main

import (
    "github.com/your-org/keyauth-go"
)

func main() {
    client := keyauth.New("your-app-key", "your-sign-secret", "https://yourdomain.com")
    
    // 1. 用户登录（首次自动绑定设备）
    result, err := client.Login("K2X9-AB7C-MN4P-QR8S", getHWID())
    if err != nil {
        panic(err)
    }
    
    // 2. 校验响应签名
    if !client.VerifyResponse(result) {
        panic("响应签名校验失败")
    }
    
    // 3. 启动心跳保活
    client.StartHeartbeat(result.Token, 60, func() {
        // 心跳失败回调
        os.Exit(1)
    })
    
    // 4. 解锁功能
    if result.Features["pro"] {
        unlockProFeatures()
    }
}
```

---

## 6. 目录结构说明

```
keyauth-saas/
├── apps/
│   ├── server/                    # Go 后端
│   │   ├── cmd/
│   │   │   └── main.go           # 程序入口（含 StartVerifyLogWorker + StartOperationLogWorker）
│   │   ├── internal/
│   │   │   ├── auth/             # JWT/TOTP/login_lock（v0.2.1）+ jti 单点踢出（v0.4.0：BlacklistRefreshTokenByJTI + IsRefreshTokenBlacklisted 双维度）
│   │   │   ├── config/           # 配置加载 + sys_config 缓存（cache.go）+ v0.4.0 AppConfig 加 LogLevel/LogFormat/LogOutput
│   │   │   ├── handler/          # HTTP 处理器（18 个文件，148 条路由，v0.3.6 新增 3 条代理注册公开路由）
│   │   │   │   ├── admin.go / admin_business.go / admin_finance.go  # 超管 3 文件
│   │   │   │   ├── tenant_business.go / tenant_finance.go / tenant_settle.go  # 开发者 3 文件
│   │   │   │   ├── agent_business.go  # 代理 1 文件
│   │   │   │   ├── app.go / card.go / client.go  # 应用/卡密/客户端验证
│   │   │   │   ├── auth.go / session.go / profile.go / public.go  # 鉴权/会话/账号设置/公开 API（auth.go 含 v0.3.6 AgentRegister + v0.4.0 jti 单点踢出；session.go 含 v0.4.0 revokeSessionByJTI + 结构化日志；profile.go 含 v0.4.0 2FA backup_codes DB 持久化）
│   │   │   │   ├── install.go  # 安装向导（v0.3.6，首次部署配置）
│   │   │   │   ├── pay.go  # 平台总支付 + 开发者自有易支付占位（v0.3.6 EpayNotify 前缀分发 + processAgentRegisterPaid）
│   │   │   │   ├── log_worker.go  # 异步日志 worker（验证 4096 + 操作 2048，v0.4.0 替换 _ = err 为 logger.Error 结构化日志）
│   │   │   │   └── deps.go  # 依赖注入容器
│   │   │   ├── grayscale/        # v0.4.0 灰度发布核心（Match 7 步过滤链 + HashBucket SHA-256 稳定桶 + ParseList + DefaultRate + IsEnabled）
│   │   │   ├── heartbeat/        # 心跳保活（Redis Sorted Set，6 个方法）
│   │   │   ├── logger/           # v0.4.0 结构化日志封装（基于 Go 标准库 log/slog，零依赖；Init/Debug/Info/Warn/Error + 4 个 Ctx 版本）
│   │   │   ├── middleware/       # 中间件（auth/tenant/signature/ratelimit/response/time）
│   │   │   ├── migration/        # 轻量级 SQL 文件迁移（v0.3.5）
│   │   │   ├── model/            # 30 个 GORM struct（v0.4.0 三表加 BackupCodes 字段；v0.4.0 Agent 加 ParentID/Level + AgentInviteCode 加 CreatorType/CreatorAgentID；v0.4.0 AppVersion 加 ReleaseStrategy/GrayscaleRate/GrayscalePlatforms/GrayscaleRegions/GrayscaleChannels）
│   │   │   ├── multilevel/       # v0.4.0 多级代理核心（DistributeCrossCommission 跨级佣金 + CanCreateSubordinate + ComputeSubordinateLevel + BuildAgentTree + ListSubordinates）
│   │   │   ├── quota/            # 套餐配额检查（CheckMaxApps/MaxCards/MaxAgents/MaxDevices，v0.3.5）
│   │   │   ├── router/           # 路由注册
│   │   │   └── update/           # v0.4.0 在线更新核心（Manager.ExecuteUpdate 6 步流程 + VerifyWebhookSignature + ParsePushEvent + BranchMatches + AcquireLock/ReleaseLock + HealthCheck + Rollback）
│   │   ├── migrations/           # 11 套 SQL 迁移（001 ~ 011；008 = v0.4.0 2FA backup_codes 字段；009 = v0.4.0 多级代理 parent_id/level + creator_type/creator_agent_id + 4 项 sys_config；010 = v0.4.0 灰度发布 app_version 5 字段 + 3 项 sys_config；011 = v0.4.0 在线更新 system_update_log 表 + 8 项 sys_config）
│   │   ├── pkg/
│   │   │   ├── crypto/           # AES-256-GCM + RSA-4096 + HMAC-SHA256 + bcrypt + 卡密生成
│   │   │   ├── epay/             # 彩虹易支付工具包
│   │   │   ├── snowflake/        # 雪花算法订单号
│   │   │   └── ua/               # User-Agent 解析（OS/Browser/版本号/设备类型/爬虫，v0.4.0）
│   │   ├── go.mod
│   │   └── go.sum
│   │
│   └── admin/                     # Vue3 前端（超管 + 开发者 + 代理 + 官网 + H5 五合一）
│       ├── src/
│       │   ├── api/               # 9 个 API 模块（admin/agent/apps/auth/cards/http/pay/profile/sysConfig/tenant/tenantFinance）
│       │   ├── components/        # 公告横幅（Platform/Developer/Agent）+ EmptyState + PageHeader + ResponsiveTable
│       │   ├── layouts/           # AdminLayout / TenantLayout / AgentLayout / BasicLayout / H5Layout
│       │   ├── router/            # 路由 + 角色守卫
│       │   ├── stores/            # Pinia: auth / sysConfig
│       │   ├── styles/            # SCSS 变量
│       │   ├── views/
│       │   │   ├── admin/         # 12 个超管页面
│       │   │   ├── agent/         # 8 个代理页面
│       │   │   ├── tenant/        # 16 个开发者页面
│       │   │   ├── h5/            # 4 个 H5 页面
│       │   │   ├── landing/       # 官网首页
│       │   │   ├── login/         # 登录页
│       │   │   ├── register/      # 开发者注册
│       │   │   └── error/         # 404
│       │   ├── App.vue
│       │   └── main.ts
│       ├── .env.development / .env.production
│       ├── vite.config.ts
│       └── package.json
│
├── configs/                       # 运行时配置
│   └── config.yaml.example
│
├── deploy/                        # 部署相关
│   └── nginx/
│       ├── admin.conf             # admin 反代
│       └── gateway.conf           # 网关总入口（限流）
│
├── docs/                          # 四份核心文档
│   ├── CHANGELOG.md
│   ├── PROJECT.md                 # 本文件
│   ├── SPEC.md
│   └── TODO.md
│
├── keys/                          # RSA 密钥对挂载点
│   └── .gitkeep
│
├── scripts/                       # 脚本
│   ├── baota_deploy.sh            # 宝塔一键部署
│   ├── gen_rsa_key.sh             # RSA-4096 密钥对生成（v0.3.5，独立脚本）
│   ├── reset_admin_password.sh    # 重置超管密码
│   ├── auto_push.sh               # 自动提交推送
│   └── deploy_update.sh           # v0.4.0 在线更新部署脚本（go mod download + go build + DEPLOY_MODE 自适应重启 systemd/docker/pm2/none）
│
├── Dockerfile                     # 后端镜像（多阶段构建）
├── Dockerfile.admin               # 前端镜像
├── docker-compose.yml             # 完整编排（mysql/redis/api/admin/nginx）
├── .env.example
├── .gitignore
├── PROMPT.md                      # AI 接手指引
└── README.md
```

---

## 7. 贡献指南

### 7.1 开发环境搭建
```bash
# 1. 克隆仓库
git clone https://github.com/your-org/keyauth-saas.git
cd keyauth-saas

# 2. 启动依赖服务
docker-compose up -d mysql redis

# 3. 后端开发
cd apps/server
go mod tidy
go run cmd/main.go

# 4. 前端开发
cd apps/admin
pnpm install
pnpm dev
```

### 7.2 提交规范
详见 [SPEC.md](./SPEC.md) 第 4 节「提交规范」。

### 7.3 分支策略
- `main`：生产分支，受保护，只接受 PR
- `develop`：开发主分支
- `feature/*`：功能分支
- `fix/*`：修复分支
- `release/*`：发布分支

### 7.4 编码铁律（HARD）
所有代码必须遵守 `web-project-flow` skill（已全局安装）的三份铁律：
- references/04 禁硬编码假数据
- references/05 配置后台化 sys_config
- references/06 防 AI 幻觉

可通过 `/bhardcode /bconfig /bhaluc` 一次性加载；用 `/bdocs` 触发四份文档联动更新。违反铁律的代码必须重写。

---

## 8. 联系与支持

- 项目仓库：https://github.com/your-org/keyauth-saas
- 问题反馈：https://github.com/your-org/keyauth-saas/issues
- 文档中心：https://docs.yourdomain.com

---

**文档版本**：0.4.0
**最后更新**：2026-07-20（v0.4.0 第八项迁移：在线更新体系 migration 011 system_update_log 表 + 8 项 sys_config + update 包 Manager.ExecuteUpdate/Rollback/HealthCheck + VerifyWebhookSignature + GitHubWebhook/AdminUpdateStatus/AdminTriggerUpdate/AdminListUpdateHistory/AdminRollbackUpdate/AdminGetUpdateLog handler + scripts/deploy_update.sh + 37 个测试全 PASS）
**维护者**：KeyAuth SaaS Team
