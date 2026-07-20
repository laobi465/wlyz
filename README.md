# KeyAuth SaaS

> 面向开发者的多租户卡密验证 SaaS 平台

[![Version](https://img.shields.io/badge/version-0.6.0-blue.svg)](docs/CHANGELOG.md)
[![License](https://img.shields.io/badge/license-Proprietary-red.svg)](#许可证)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4+-42b883.svg)](https://vuejs.org)
[![Deploy](https://img.shields.io/badge/deploy-one--click-success.svg)](#一键部署)

## 项目简介

KeyAuth SaaS 是一个多租户卡密验证 SaaS 平台，借鉴布丁卡密的安全设计理念，为软件开发者提供：

- **在线验证 + 一机一卡**：HMAC-SHA256 签名 + RSA-4096 响应签名 + 硬件指纹绑定
- **多层支付体系**：平台总支付（默认）+ 开发者自定义易支付（按套餐开通）+ 海外支付（USDT/PayPal/Stripe）
- **三级公告系统**：平台公告 / 开发者公告 / 应用公告 同时显示
- **代理分销体系**：开发者邀请码 + 注册费 + 多级佣金分成（按比例/按差价/跨级分润）
- **后台可视化配置**：所有可变参数走 `sys_config` 表，无需重启即时生效
- **响应式 H5 全栈**：管理后台 / 开发者控制台 / 代理中心 / 官网 / 终端用户 H5 全部响应式适配
- **高级分析体系**：用户行为画像 + 卡密使用画像 + 风险用户识别 + 24h 异常模式检测 + 自动封禁候选

## 一键部署

> 全新服务器？只需 SSH 连接后执行下面这一行命令，脚本会自动完成：检测系统 → 安装宝塔（若未装）→ 安装 Docker（若未装）→ 拉取源码 → 生成密钥 → 构建启动 → 输出部署信息到 `/root/keyauth_deploy_info.txt`。

### 远程一行命令（推荐，适合全新服务器，无需事先上传源码）

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh)"
```

### 先下载后执行（适合需审查脚本或网络不稳定）

```bash
curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh -o one_click_deploy.sh
sudo bash one_click_deploy.sh
```

### 已 clone 项目内执行

```bash
sudo bash scripts/one_click_deploy.sh
```

**脚本执行流程（约 8-15 分钟，一行命令完成全部）**：

| 步骤 | 操作 | 说明 |
|---|---|---|
| 1 | 检测操作系统 | 自动识别 CentOS/Ubuntu/Debian 系 |
| 2 | 安装基础工具 | curl / wget / openssl / git |
| 3 | 检测/安装宝塔面板 | 未装则拉取宝塔官方脚本自动安装 |
| 4 | 检测/安装 Docker | 优先用宝塔脚本，回退 Docker 官方脚本（阿里云镜像） |
| 5 | 拉取/更新项目源码 | git clone 到 `/www/wwwroot/keyauth` |
| 6 | 生成 RSA-4096 密钥对 | 响应签名用，现场生成不进仓库 |
| 7 | 生成 `.env` 配置 | 自动填充强随机密钥（MySQL/Redis/AES/JWT） |
| 8 | `docker compose up -d --build` | 构建 mysql/redis/server/admin |
| 9 | **生成部署信息 txt** | 所有信息写入 `/root/keyauth_deploy_info.txt`（chmod 600） |

**部署完成后自动生成的 txt 文件包含**：
- 宝塔面板入口（地址/账号/密码/端口）
- KeyAuth 访问地址（前端后台 + API）
- 所有密钥（MySQL/AES/JWT）
- 服务状态、下一步操作、运维命令、备份建议

**部署完成后必做**：
1. `cat /root/keyauth_deploy_info.txt` 查看完整部署信息
2. 执行 `bash scripts/reset_admin_password.sh` 重置超管密码
3. 宝塔面板「安全」关闭 MySQL/Redis 公网端口
4. 宝塔面板「网站」绑定域名 + 申请 SSL 证书
5. 登录后台「系统配置 > 支付」配置易支付参数

> 脚本严格遵守三铁律：禁硬编码（密钥现场生成）/ 配置走 sys_config / 反幻觉（脚本含详细日志和错误处理）。完整说明见 [scripts/one_click_deploy.sh](scripts/one_click_deploy.sh)。

## 当前版本（v0.6.0 进行中）

| 模块 | 状态 | 说明 |
|---|---|---|
| 项目骨架 + 26 张表 DDL + 种子数据 | ✅ 已完成 v0.2.0 | Go + Vue3 + Docker + 宝塔部署 |
| JWT 双 Token + TOTP 2FA + 登录锁定 | ✅ 已完成 v0.2.1 | 三角色统一鉴权，refresh 轮换 + 黑名单 |
| 应用管理（CRUD + 密钥生成/轮换） | ✅ 已完成 v0.2.2 | AppKey/AppSecret/SignSecret + AES 加密入库 |
| 卡密管理（5 类型 + 批量生成 + 状态机 + CSV 导入导出） | ✅ 已完成 v0.3.6 | duration/count/permanent/trial/feature + CSV 导出/导入（v0.3.6） |
| 客户端验证 API（9 个端点全部实现） | ✅ 已完成 v0.2.2 | login/verify/heartbeat/bind/unbind/get_var/notice/version/logout |
| 心跳保活（Redis Sorted Set） | ✅ 已完成 v0.2.2 | 百万级在线设备支持 |
| 设备强制下线（封禁卡密联动） | ✅ 已完成 v0.3.6 | TenantBanCard 联动 heartbeat.Remove 清 Redis 心跳 + DB 标记 banned |
| 平台总支付（彩虹易支付） | ✅ 已完成 v0.2.3 | 下单/异步回调/同步跳转/自动发卡/Redis 防重入/订单超时关闭/平台抽成结算 |
| 前端响应式 H5（三角色 + 官网 + H5） | ✅ 已完成 v0.2.4 | BasicLayout 通用布局 + 移动端抽屉 + 官网首页 + H5 购卡/查卡 + 2FA 登录 + auth refresh 自动续期 |
| 代理核心页面（购卡/订单/佣金/提现） | ✅ 已完成 v0.2.5 | Dashboard + Cards + Orders + Commission + Balance 全响应式 |
| 三角色 Profile + 双 Dashboard | ✅ 已完成 v0.2.6 | admin/tenant/agent 账号设置（基础资料/改密/2FA/登录设备/提现账户）+ admin/tenant 工作台（8 数据卡 + 趋势图 + 排行榜） |
| 前端三角色全页面响应式 H5 完整覆盖 | ✅ 已完成 v0.2.7 | 16 个 PlaceholderView 全部替换为真实页面 |
| 后端业务接口（admin/tenant/agent dashboard + profile + CRUD） | ✅ 已完成 v0.3.0 | 51 个 501 占位全部升级为真实实现（含云变量/版本/邀请码/三级公告/2FA 全流程/登录设备） |
| 字段补全 + 登录失败日志 + 登录设备表 | ✅ 已完成 v0.3.1 | migration 006 字段补齐 + log_login_failed 表 + refresh_token_device 表 |
| 代理充值/提现审核闭环 | ✅ 已完成 v0.3.2 | tenant_finance.go 6 个审核 handler + 前端双审核页面 |
| 日志系统（异步 Worker + CSV 导出） | ✅ 已完成 v0.3.3 | 验证/操作日志异步 channel worker + 三表独立查询 + UTF-8 BOM CSV 导出 + 前端 3 Tab 升级 |
| 结算与对账闭环（开发者余额体系） | ✅ 已完成 v0.3.4 | sys_tenant.balance/frozen_balance + tenant_balance_log + tenant_withdraw + 批量结算 + 对账报表 + 双审核页面 |
| P0 修复：RSA 脚本 / 迁移机制 / H5 公共 API / 套餐配额 | ✅ 已完成 v0.3.5 | 独立 RSA 生成脚本 + 轻量级 SQL 文件迁移 + H5 公共 API + quota 包统一封装 |
| 安装向导页面 `/install` | ✅ 已完成 v0.3.6 | 首次部署 4 步向导配置超管账号 + 平台基础参数（替代原 seed 占位 hash + 后置脚本） |
| 代理注册付费流程 | ✅ 已完成 v0.3.6 | 方案 B 先支付后建 Agent：AgentRegister 创建 REG 订单 + EpayNotify 前缀分发 + processAgentRegisterPaid 事务建 Agent + 邀请码状态机闭环（达 max_uses 置 exhausted）+ Register.vue 落地 3 处 TODO + 修复 install.go 配置键名 bug |
| 文档全量同步对齐实际状态 | ✅ 已完成 v0.3.6 | README/PROMPT/PROJECT/SPEC/TODO/CHANGELOG 六份文档联动更新 |
| 开发者自有易支付 + 双层支付模式切换 | ✅ 已完成 v0.3.6 | EpayTenantNotify 完整实现 + processTenantOwnPaidOrder 事务 + loadTenantPayConfig AES 解密 + CreatePayOrder 内 SysPackage.AllowCustomPay + TenantPayConfig.Enabled 双开关切换 + TOP/ORD/REG 前缀分发 |
| 客户端 SDK（Python / Node.js / PHP 三语言） | ✅ 已完成 v0.3.6 | `sdks/python/` + `sdks/nodejs/` + `sdks/php/` 各封装 9 个验证 API + HMAC-SHA512/256 签名 + KeyAuthError 异常体系 + 完整 README 文档 |
| 单元测试 + 签名对齐测试 | ✅ 已完成 v0.3.6 | 6 个测试包（crypto/snowflake/epay/quota/heartbeat/middleware）+ 跨语言签名对齐测试（`sdks/tests/` + `pkg/crypto/sign_alignment_test.go`）；`go test ./...` 全 PASS |
| UA 解析工具包 | ✅ 已完成 v0.4.0 | `pkg/ua` 自实现（零第三方依赖）：DeviceInfo 结构 + Parse + IsBot；handler 层 `parseDeviceName` / `detectDeviceType` / `ListLoginDevicesFull` 全部接入；ListLoginDevices 响应新增 os/os_version/browser/browser_version/is_bot 字段（不改 DB schema，向前兼容） |
| JWT jti 精准单点踢出 | ✅ 已完成 v0.4.0 | jti 嵌入 JWT RegisteredClaims.ID + `auth.BlacklistRefreshTokenByJTI` + `revokeSessionByJTI`；KickDevice / Logout / RefreshToken 轮换全部改造为 jti 维度（仅失效指定会话，不影响其他设备）；修改密码/关闭 2FA 仍走 user 维度黑名单强制全部重登；8 个测试包（新增 `internal/auth`，18 个测试 + 1 个 middleware JTI 注入测试）全 PASS；兼容旧 token（无 jti 回退 user 维度） |
| 2FA 备用码 DB 持久化 | ✅ 已完成 v0.4.0 | migration 008 三表（sys_admin/sys_tenant/agent）加 `backup_codes VARCHAR(512)` 字段；profile.go 新增 `loadUserBackupCodes`/`updateUserBackupCodes`/`consumeBackupCode`；Verify2FA 改为 DB 落库 + Disable2FA 清空字段；兼容 v0.3.x 老用户（DB 为空时回退 Redis 读取，首次消费后自动迁移到 DB + 清理 Redis）；13 个 handler 测试全 PASS |
| 登录失败日志结构化 | ✅ 已完成 v0.4.0 | 新建 `internal/logger` 包基于 Go 标准库 `log/slog`（零依赖，取代 zap/zerolog）；`AppConfig` 加 LogLevel/LogFormat/LogOutput；3 处 `_ = err` 静默丢弃替换为 `logger.Error` 结构化日志（含 err + 业务字段）；6 个 logger 测试全 PASS；10 个测试包全 PASS |
| 全语言 SDK 扩展 | ✅ 已完成 v0.4.0 | 新增 5 个语言 SDK（Go / Java / C# / C++ / 易语言），每个含 9 个客户端 API + 独立签名对齐脚本；`pkg/crypto/sign_alignment_test.go` 从 3 语言扩展到 7 语言（缺失运行时自动 skip + 易语言 Windows-only 永久 skip）+ 5 个新 SDK 目录结构元数据校验；11 个测试包全 PASS；签名算法：Go / C++ 原生 SHA-512/256 字节级对齐，Java / C# 反射探测 BouncyCastle 启用 SHA-512/256 否则回退 HmacSHA256，易语言统一 HMAC-SHA256（生态限制明确标注） |
| 多级代理体系 | ✅ 已完成 v0.4.0 | migration 009：`agent.parent_id` + `level` + `agent_invite_code.creator_type` + `creator_agent_id` + 4 项 sys_config（cross_level_2_rate=50% / cross_level_3_rate=20% / max_level=3 / agent_can_create=1）；新建 `internal/multilevel` 包：`DistributeCrossCommission` 沿 parent_id 链向上分润（level 2→父级 50%，level 3→父级 50% + 祖父级 20%）+ `CanCreateSubordinate` 层级资格校验 + `ComputeSubordinateLevel` 计算下级层级 + `BuildAgentTree` 递归树（含租户隔离）+ `ListSubordinates` 单层列表；AgentGenerateCards 接入跨级佣金分发；processAgentRegisterPaid 接入层级计算；三端代理树查询 API（admin/tenant/agent）；5 个代理邀请码管理路由；27 个 multilevel 测试全 PASS（11 个测试包全 PASS 无回归） |
| 灰度发布体系 | ✅ 已完成 v0.4.0 | migration 010：`app_version` 5 字段（`release_strategy` / `grayscale_rate` / `grayscale_platforms` / `grayscale_regions` / `grayscale_channels`）+ 复合索引 `idx_app_status_strategy` + 3 项 sys_config（`app.version.grayscale.enabled`/`default_rate`/`hash_salt`）；新建 `internal/grayscale` 包：`Match` 7 步过滤链（full 命中 + 全局开关回退 + 平台/渠道/地区白名单 + 比例判定）+ `HashBucket` SHA-256(salt:appID:clientID) 稳定桶 0-99 + `ParseList` 逗号分隔解析器 + `DefaultRate`/`IsEnabled` 配置读取；`ClientVersion` 升级遍历候选版本匹配首个命中（请求扩展 hwid/device_id/platform/channel/region，响应扩展 release_strategy/grayscale_hit/grayscale_bucket/grayscale_rate）；`TenantUpdateVersion` 编辑接口 + `AdminListVersions`/`AdminGetVersion` 跨租户查询；33 个 grayscale 测试全 PASS（12 个测试包全 PASS 无回归） |
| 在线更新体系 | ✅ 已完成 v0.4.0 | migration 011：`system_update_log` 表（trigger_source/trigger_by/commit_before/commit_after/status/steps_json/log_text/rolled_back_from）+ 3 索引 + 8 项 sys_config（webhook.secret/branch/auto_update + deploy.script_path + healthcheck.url/timeout + rollback.enabled + lock.timeout）；新建 `internal/update` 包：`VerifyWebhookSignature` HMAC-SHA256（`hmac.Equal` 防时序攻击）+ `ParsePushEvent` push event 解析 + `BranchMatches` refs/heads/ 规范化 + `Manager.AcquireLock`/`ReleaseLock` 双重锁（进程内 mutex + Redis SET NX EX）+ `Manager.ExecuteUpdate` 6 步流程（加锁→pending 日志→git fetch+reset→bash 部署脚本→健康检查→失败回滚）+ `Manager.Rollback` 回滚到 commit_before + `Manager.HealthCheck` 禁用重定向捕获原始 3xx；新建 `handler/update.go`：`GitHubWebhook` POST /public/update/webhook（无鉴权+HMAC 签名）+ `AdminUpdateStatus`/`AdminTriggerUpdate`/`AdminListUpdateHistory`/`AdminRollbackUpdate`/`AdminGetUpdateLog` 5 个 admin 接口；`scripts/deploy_update.sh` 默认脚本（go mod download + go build + DEPLOY_MODE 自适应 systemd/docker/pm2/none）；37 个 update 测试全 PASS（13 个测试包全 PASS 无回归） |
| 数据备份恢复体系 | ✅ 已完成 v0.4.0 | migration 012：`system_backup_log` 表（backup_type/file_path/file_size/checksum/status/tables_count/rows_count/restored_from）+ 3 索引 + 6 项 sys_config（backup.dir/retention_days/auto_enabled/encryption_key/compress/tables_filter）；新建 `internal/backup` 包：`Manager.CreateBackup` 全库 SQL INSERT 序列化 + 可选 gzip 压缩 + 可选 AES-256-GCM 加密 + SHA-256 checksum + 文件写入 + 审计日志；`Manager.RestoreBackup` SHA-256 校验 + AES 解密 + gunzip + 事务化 DELETE+INSERT 防 PK 冲突 + restored_from 关联原备份；`Manager.CleanupExpired` 按保留天数清理；`serializeValue` 处理 nil/string/[]byte/bool/time.Time/int/float；新建 `handler/backup.go`：7 个 admin 接口（status/create/list/get/download/restore/cleanup），下载前强制 checksum 校验，恢复前 status=success 校验；backup 测试全 PASS（14 个测试包全 PASS 无回归） |
| 监控告警体系 | ✅ 已完成 v0.4.0 | migration 013：`system_metric` 表（metric_name/metric_value/metric_unit/labels_json/collected_at）+ `system_alert` 表（alert_rule/severity/status/threshold/operator/fired_at/resolved_at/acked_by/notify_sent）+ 6 索引 + 9 项 sys_config（collect_interval/alert_enabled/notify.webhook_url/silence_minutes/threshold.cpu_usage/memory_usage/disk_usage/error_rate/retention_days）；新建 `internal/monitor` 包：`CompareWithOperator` 显式 switch 实现 `>`/`<`/`>=`/`<=`/`==`（禁止字符串拼接 eval）+ `Manager.CollectSystemMetrics` gopsutil 采集 CPU/内存/磁盘 + DB 查询在线设备/验证数/错误率 + `Manager.EvaluateAlerts` 阈值比较 + 静默期去重 + 自动恢复 + webhook 通知 + `Manager.ResolveStaleAlerts` 超 1h 自动恢复 + `Manager.CollectAndEvaluate` 一体化入口（互斥锁防并发）；新建 `handler/monitor.go`：7 个 admin 接口（status/collect/metrics/alerts/ack/resend/cleanup）；53 个 monitor 测试全 PASS（15 个测试包全 PASS 无回归） |
| 通知系统 | ✅ 已完成 v0.4.0 | migration 014：`notify_template`（code/channel/subject/body_html/tenant_id/status）+ `notify_log`（tenant_id/channel/recipient/template_code/status/provider_msg_id/error_message/retry_count/priority）+ 16 项 notify.* sys_config（sms.enabled/provider/access_key_id/access_secret_enc/sign_name + email.enabled/smtp_host/port/username/password_enc/from_address/from_name + inapp.enabled + retry.times/interval_seconds + rate_limit.per_minute）；新建 `internal/notify` 包：`Render` 用 `strings.NewReplacer` 安全替换（防 SSTI，不用 text/template）+ `Manager.Send` 同步发送（限流→查模板→渲染→调 provider→写日志）+ `Manager.Retry` 失败重试 + `Manager.GetStats` 统计（修复 GORM Where 累积 bug，用 baseWhere 闭包新会话）+ `SMSProvider`/`EmailProvider` 接口 + Aliyun SMS 骨架（AccessKey 未配置返回 ErrProviderNotConfig）+ SMTP Email 真实现（`net/smtp.SendMail` + AES 解密密码）；新建 `handler/notify.go`：9 个 admin 端点（status/templates CRUD/logs/list+get/retry/test）；36 个 notify 测试全 PASS（修复 GORM 列名 `provider_msg_id` ≠ JSON tag `provider_msgid` 的 snake_case 转换 bug， Updates map 必须用 GORM 列名）（15 个测试包全 PASS 无回归） |
| 终端用户体系 | ✅ 已完成 v0.4.0 | migration 015：`end_user`（tenant_id/username/email/phone/password_hash/status/last_login_at/last_login_ip）+ `end_user_card`（user_id/card_id/tenant_id）+ `end_user_token`（user_id/refresh_token_hash/access_jti/expires_at/user_agent/ip/revoked）+ ALTER `app_card.end_user_id` + 10 项 enduser.* sys_config（register_enabled/login_enabled/password_min_length/username_min_length/refresh_token_ttl_days/access_token_ttl_minutes/max_sessions_per_user/verify_code_ttl_minutes/bcrypt_cost/default_avatar）；新建 `internal/enduser` 包：`Manager.Register/Login/RefreshToken/Logout/RevokeSession/RevokeAllSessions/ListSessions/BindCard/UnbindCard/ListMyCards/GetCardDetail/GetProfile/UpdateProfile/ChangePassword/ResetPassword` 15 方法；access token 用简化 HMAC-SHA256(secret|user_id|app_id|exp).signature（不引 jwt 依赖）+ refresh token 用 UUID + SHA-512 哈希存储 + jti 单点踢出 + bcrypt cost=12；新建 `middleware/auth.go`：`H5EndUserAuth` 用 HMAC-SHA256 校验 + `hmac.Equal` 常量时间比较防时序攻击；新建 `handler/enduser.go`：19 个端点（5 公开 + 10 H5 + 4 admin），绑卡用 `deps.DB.Transaction()` 事务保护防 race condition；53 个 enduser 测试全 PASS（15 个测试包全 PASS 无回归） |
| API 开放平台 | ✅ 已完成 v0.4.0 | migration 016：`developer_api_token`（tenant_id/name/token_hash/prefix/scopes/expires_at/last_used_at/last_used_ip/status/revoked_at）+ `webhook_endpoint`（tenant_id/name/url/secret_enc/events/status/failure_count/last_response_code/last_response_at/last_error）+ `webhook_delivery`（tenant_id/endpoint_id/event_type/event_id/payload/status/response_code/response_body/attempt_count/max_retry/next_retry_at/delivered_at）+ 8 项 openapi.*/webhook.* sys_config；新建 `internal/openapi` 包：`TokenManager` SHA-512 哈希存储 + 8 个预定义 scope + TTL + 单租户数量上限；`WebhookManager` HMAC-SHA256 签名（`hmac.Equal` 防时序攻击）+ AES-256-GCM 加密 secret + 退避重试（2/4/6 分钟）+ 连续失败阈值自动 disable endpoint；5 个事件类型常量（order.paid/card.generated/agent.registered/agent.recharge.approved/agent.withdraw.paid）；新建 `middleware/auth.go`：`APITokenAuth` 第三方鉴权 + `RequireScope` 权限校验；新建 `handler/openapi.go`：15 个端点（1 admin + 13 tenant + 1 openapi/whoami）+ `DispatchWebhookEvent` 异步分发辅助；5 个业务点接入 Webhook（card.generated / order.paid / agent.registered / agent.recharge.approved / agent.withdraw.paid）；61 个 openapi 测试全 PASS（16 个测试包全 PASS 无回归） |
| 管理员更新弹窗通知 | ✅ 已完成 v0.4.0 | migration 017：2 项 `update.poll.*` sys_config（enabled=1 总开关 / interval_seconds=30 间隔）+ `PollIntervalMin=10` 强制下限；新建 `AdminUpdatePoll` GET /admin/update/poll 轻量轮询端点：仅返回 enabled + interval_seconds + current_commit + is_locked + 最近一次更新元信息共 8 字段（不含 log_text/steps_json 重字段），后端调整配置下一次轮询即时生效；前端 `apps/admin/src/api/update.ts` 补全 6 个 API；前端 `apps/admin/src/components/UpdateNotifier.vue` 组件挂载于 `AdminLayout.vue`：localStorage 持久化 `keyauth_admin_last_known_commit` 跨会话检测更新 + 自适应间隔（每次轮询用响应 interval_seconds 重置定时器）+ `ElMessageBox.confirm` 弹窗 + `notifiedCommit` ref 防本会话重复弹窗；13 个测试全 PASS（handler 包无回归） |
| 高级安全 | ✅ 已完成 v0.4.0 | migration 018：3 张新表 `risk_rule`/`risk_event`/`login_geo_alert` + ALTER `app_device` 增加 6 字段（hwid_components/user_agent/client_ip_ext/screen_resolution/timezone/language）+ 16 项 `cloudflare.*`/`risk.*` sys_config + 5 条 seed 内置规则（异地登录 60 分 alert / 新设备 40 分 alert / 异常 UA 30 分 alert / 异常时段 20 分 disabled / 高频请求 50 分 challenge）；新建 `internal/risk` 包（901 行）：5 条内置规则评估函数 + custom 自定义规则 + 评分累计阈值升级 alert→challenge→block + EvaluateLogin 接入登录流程 ShouldBlock 撤销会话 + RecordEvent 落盘 + 异地登录 IP 网段比较 IPv4 /24 IPv6 /64 无需 GeoIP + NetworkOf 工具函数 + 规则 CRUD（内置禁删/禁改类型）+ 事件/告警确认 + GetStats 看板；新建 `middleware/cloudflare.go`：CloudflareRealIP 中间件从 CF-Connecting-IP 头取真实 IP + 受信 CIDR 列表校验来源 + RealIP(c)/IPCountry(c) 工具函数；新建 `middleware/risk_engine.go`：匿名请求风控评估中间件；修改 `ratelimit.go`：RateLimitByIP/IPBlacklist 接入 RealIP(c)；修改 `handler/auth.go`：步骤 7.2 风控评估（ShouldBlock 撤销会话拒绝登录，HitRules 记录事件）；新建 `handler/risk.go`：admin 风控面板 11 端点（stats/rules CRUD/events/geo_alerts）；router 注册 11 条路由 + 全局中间件 CloudflareRealIP；~30 个 risk 测试 + 5 个 cloudflare 测试全 PASS（17 个测试包全 PASS 无回归） |
| 公告增强 + 数据统计 | ✅ 已完成 v0.4.0 | migration 019：ALTER `notice` ADD `content_format` 字段（text/html）+ 9 项 `notice.*`/`stats.*` sys_config（popup.enabled/max_unread/dismiss_ttl_hours + richtext.enabled/max_length + verify_trend.default_days/max_days + agent_ranking.default_limit/max_limit）；新建 `handler/notice_stats.go`（~620 行）：三端首次登录强制弹窗 API（admin/tenant/agent 三端 `GET /:role/notices/popup`，status=published + is_popup=true + 未在 notice_read 表 + 受 max_unread 上限约束）+ 标记已读 API（`POST /:role/notices/:id/read` FirstOrCreate 幂等）+ 验证趋势图 API（admin/tenant 两端 `GET /:role/stats/verify_trend?days=30`，按日聚合 log_verify 按 result/action 双维度 + days 参数受 sys_config 上下限约束）+ 代理业绩排行 API（admin/tenant 两端 `GET /:role/stats/agent_ranking?sort_by=total_amount&limit=10`，联表 agent+sys_tenant+app_order 支持 total_amount/commission/net_amount/order_count 四种排序 + limit 受 sys_config 上下限约束）；admin/tenant Create/Update/List 接口扩展支持 content_format/is_popup/show_badge 字段 + 富文本校验（richtext.enabled 开关 + max_length 长度限制）；router 注册 10 条新路由；18 个 notice_stats 测试全 PASS（17 个测试包全 PASS 无回归） |

详细变更见 [CHANGELOG.md](docs/CHANGELOG.md)，任务进度见 [TODO.md](docs/TODO.md)。

## 技术栈

| 层级 | 技术选型 |
|---|---|
| 后端 | Go 1.22 + Gin + GORM |
| 前端 | Vue 3.4 + TypeScript + Element Plus + Vite + Pinia |
| 数据库 | MySQL 8.0 + Redis 7 |
| 部署 | Docker + Docker Compose + 宝塔面板 |
| 加密 | AES-256-GCM + RSA-4096 + bcrypt(cost=12) + HMAC-SHA256 |

## 目录结构

```
keyauth-saas/
├── apps/
│   ├── server/                    # Go 后端
│   │   ├── cmd/main.go            # 入口
│   │   ├── internal/
│   │   │   ├── config/            # 配置加载 + sys_config 缓存
│   │   │   ├── handler/           # HTTP 处理器
│   │   │   ├── middleware/         # 中间件（auth/signature/tenant/ratelimit）
│   │   │   ├── model/             # GORM 模型
│   │   │   └── router/            # 路由注册
│   │   ├── migrations/            # 数据库迁移 SQL
│   │   │   ├── 001_init_schema.up.sql   # 26 张表 DDL
│   │   │   ├── 001_init_schema.down.sql
│   │   │   ├── 002_seed_data.up.sql     # 默认配置/套餐/超管
│   │   │   └── 002_seed_data.down.sql
│   │   └── pkg/
│   │       ├── crypto/            # AES/RSA/HMAC/bcrypt/卡密生成
│   │       └── snowflake/         # 订单号生成
│   └── admin/                     # Vue3 前端
│       ├── src/
│       │   ├── api/               # HTTP 封装
│       │   ├── components/        # 公告横幅等通用组件
│       │   ├── layouts/           # 三套布局：AdminLayout/TenantLayout/AgentLayout
│       │   ├── router/            # 路由 + 角色守卫
│       │   ├── stores/            # Pinia stores（auth/sysConfig）
│       │   ├── styles/            # 全局 SCSS 变量
│       │   └── views/             # 登录页 + 404 + 代理注册页
│       └── vite.config.ts
├── configs/                       # 运行时配置
│   └── config.yaml.example
├── deploy/
│   └── nginx/                     # admin.conf / gateway.conf
├── docs/                          # 项目文档
│   ├── CHANGELOG.md
│   ├── PROJECT.md
│   ├── SPEC.md
│   └── TODO.md
├── keys/                          # RSA 密钥对挂载点
├── scripts/
│   ├── one_click_deploy.sh        # SSH 一键自动化部署（推荐）
│   ├── baota_deploy.sh            # 宝塔面板手动部署
│   └── reset_admin_password.sh    # 重置超管密码
├── Dockerfile                     # 后端镜像
├── Dockerfile.admin               # 前端镜像
├── docker-compose.yml             # 完整编排
├── .env.example                   # 环境变量样例
└── README.md
```

## 快速开始

### 方式 A：一键部署（强烈推荐）

见上方「一键部署」章节，远程一行命令搞定全部：

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh)"
```

### 方式 B：手动部署（已装好宝塔 + Docker）

适合已有环境、需精细控制每一步的用户。

### 1. 准备环境

- Linux 服务器（推荐 Ubuntu 22.04 / CentOS 7+）
- 已安装宝塔面板（推荐）或 Docker + Docker Compose
- 域名 + SSL 证书（生产环境）

### 2. 拉取代码

```bash
git clone https://github.com/laobi465/wlyz.git keyauth-saas
cd keyauth-saas
```

### 3. 配置环境变量

```bash
cp .env.example .env
vim .env
```

**必须修改的字段**（铁律 04：禁止硬编码密钥）：

```bash
MYSQL_ROOT_PASSWORD=<强密码>
MYSQL_PASSWORD=<强密码>
REDIS_PASSWORD=<强密码>

# AES-256 密钥：32 字节 base64
openssl rand -base64 32
AES_KEY=<上面的输出>

# JWT 密钥
openssl rand -hex 32
JWT_SECRET=<上面的输出>
```

### 4. 生成 RSA 密钥对

```bash
mkdir -p keys
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out keys/rsa_private.pem
openssl rsa -in keys/rsa_private.pem -pubout -out keys/rsa_public.pem
chmod 600 keys/rsa_private.pem
```

### 5. 复制后端配置

```bash
cp configs/config.yaml.example configs/config.yaml
# 按需修改（一般默认即可，敏感字段从环境变量传入）
```

### 6. 启动服务

**方式一：宝塔面板一键部署**（推荐）

```bash
bash scripts/baota_deploy.sh
```

**方式二：Docker Compose 直接启动**

```bash
docker compose up -d
```

**方式三：生产环境（含 nginx 网关 + HTTPS）**

```bash
docker compose --profile production up -d
```

### 7. 首次部署后必做

```bash
# 重置超管密码（种子数据中的哈希为占位）
bash scripts/reset_admin_password.sh

# 在宝塔面板「安全」中关闭 MySQL/Redis 的公网端口
# 登录后台 http://your-ip:8081 配置平台易支付参数
```

## 默认账号

| 角色 | 用户名 | 密码 | 说明 |
|---|---|---|---|
| 平台超管 | admin | （占位） | 部署后必须执行 `reset_admin_password.sh` 重置 |

## 核心文档

- [CHANGELOG.md](docs/CHANGELOG.md) —— 版本变更记录
- [PROJECT.md](docs/PROJECT.md) —— 项目架构与模块清单
- [SPEC.md](docs/SPEC.md) —— 代码 / API / 安全 / 部署规范
- [TODO.md](docs/TODO.md) —— 任务清单与里程碑

## 安全说明

- 所有敏感字段（AES_KEY / JWT_SECRET / RSA 私钥 / DB 密码）通过环境变量传入，不进代码
- 客户端 API 强制 HMAC-SHA256 签名 + nonce 防重放
- 服务端响应可选 RSA-4096 签名（fail-closed）
- 密码 bcrypt cost=12，敏感字段 AES-256-GCM 加密
- IP 自动封禁机制（失败次数达阈值自动拉黑）
- 心跳通过 Redis Sorted Set 维护，支持百万级在线设备

## 后续版本规划

详见 [TODO.md](docs/TODO.md)：

- **v0.3.6（已完成 2026-07-20）**：剩余 P1 收尾（卡密 CSV 导入导出 + 设备强制下线 + 安装向导 + 代理注册付费流程 + 开发者自有易支付 + 双层支付模式切换 + 客户端 SDK 三语言 + 单元测试 + 跨语言签名对齐测试 + 文档同步）
- **v0.4.0（进行中 2026-07-20）**：UA 解析迁移 + JWT jti 精准单点踢出 + 2FA backup_codes DB 持久化 + 登录失败日志结构化 slog + 全语言 SDK 扩展 + 多级代理体系 + 灰度发布体系 + 在线更新体系 + 数据备份恢复 + 监控告警 + 通知系统 + 终端用户体系 + API 开放平台 + 管理员更新弹窗通知 + 高级安全 + 公告增强 + 数据统计 已完成（16 项迁移全绿；17 个测试包全 PASS；新增 multilevel + grayscale + update + backup + monitor + notify + enduser + openapi + risk + cloudflare + notice_stats 共 400+ 个新测试）；后续：前端 H5 接入等

## 开发约束（铁律）

本项目严格遵守 `web-project-flow` skill（已全局安装）的三份铁律：

1. **禁硬编码**：密钥 / token / 域名 / 价格 / 接口地址 全部抽离到环境变量或 sys_config
2. **配置后台化**：所有可调参数走 `sys_config` 表 + Redis 缓存 + 后台可视化编辑
3. **防幻觉**：不确定处标注「待核实」，代码不确定处标注「需验证」

可通过 `/bhelp` 查看 skill 全部 11 份提示词索引；写业务代码前用 `/bhardcode /bconfig /bhaluc` 一次性加载三铁律；变更/加功能后用 `/bdocs` 触发四份核心文档联动更新。

## 许可证

Proprietary —— 未经授权禁止商用
