# PROMPT.md —— AI 接手指引

> 当新的 AI / 开发者接手本项目时，按本文件指引快速进入工作状态。
>
> 配套 Skill：`web-project-flow`（已全局安装）。输入 `/bhelp` 查看全部 11 份提示词索引；写代码前用 `/bhardcode /bconfig /bhaluc` 加载三铁律；变更后用 `/bdocs` 触发四份文档联动更新；接手项目用 `/bonboard`。

## 一、项目背景速读（2 分钟）

本项目是 **KeyAuth SaaS**：面向开发者的多租户卡密验证 SaaS 平台。

**核心定位**：开发者注册账号 → 创建应用 → 生成卡密 → 客户端 SDK 接入验证。代理通过开发者邀请码注册并分销卡密。平台提供总支付（默认）与开发者自定义易支付（按套餐）双轨支付。

**v0.3.6 已发布**：6 个测试包（crypto/snowflake/epay/quota/heartbeat/middleware）+ 跨语言签名对齐测试全部通过。运行 `cd apps/server && go test ./...` 验证。

**v0.4.0 进行中**：UA 解析迁移 + JWT jti 精准单点踢出 + 2FA backup_codes DB 持久化 + 登录失败日志结构化 slog + 全语言 SDK 扩展 已完成（5 项迁移全绿；`pkg/ua` + `internal/auth` + `internal/logger` + `internal/handler/profile_2fa_test.go` + 5 个新 SDK 共 50+ 个新测试，11 个测试包全 PASS）。

## 二、必读文档（按顺序）

1. **README.md** —— 项目概览 + 快速部署
2. **docs/PROJECT.md** —— 架构总览 + 46 个后端模块清单 + 26 张表关系
3. **docs/SPEC.md** —— 9 大规范（代码 / API / 安全 / 部署 / 文档）
4. **docs/TODO.md** —— 当前待办与里程碑
5. **docs/CHANGELOG.md** —— 已完成版本记录

## 三、铁律强制约束

本项目的所有代码生成必须遵守三份铁律（位于 `web-project-flow` skill 的 references/04-06）：

| 铁律 | 文件 | 核心要求 |
|---|---|---|
| ① 禁硬编码 | 04-no-hardcode-fake-data.md | 密钥/token/域名/价格/IP 禁止写入业务代码 |
| ② 配置后台化 | 05-config-to-backend.md | 所有可变参数走 `sys_config` 表 + 缓存 + 后台 UI |
| ③ 防幻觉 | 06-anti-hallucination.md | 不确定处标「待核实」/「需验证」，不编造 API |

**违反铁律的代码视为不合格，必须重写。**

## 四、目录速查

```
apps/server/                # Go 后端
  internal/handler/         # HTTP 处理器（admin.go / client.go）
  internal/middleware/      # 中间件（auth / signature / tenant / ratelimit）
  internal/model/model.go   # GORM 模型（26 张表）
  internal/config/cache.go  # sys_config 缓存（铁律 05 核心）
  pkg/crypto/crypto.go      # AES / RSA / HMAC / bcrypt / 卡密生成
  pkg/ua/ua.go              # User-Agent 解析（OS/Browser/版本号/设备类型/爬虫）
  migrations/               # SQL 迁移（001 schema + 002 seed）

apps/admin/                 # Vue3 前端
  src/layouts/              # 三套布局：Admin / Tenant / Agent
  src/components/           # PlatformNoticeBanner / DeveloperNoticeBanner / AgentNotifyBanner
  src/router/index.ts       # 路由 + 角色守卫
  src/stores/               # Pinia: auth / sysConfig

deploy/nginx/               # nginx 配置（admin 反代 + gateway 总入口）
scripts/                    # 部署脚本（baota_deploy.sh / reset_admin_password.sh）
docs/                       # 四份核心文档
```

## 五、当前进度

**v0.3.6（进行中，2026-07-20）**：

已新增完成（v0.3.6）：
- ✅ 卡密 CSV 导入导出（card.go `TenantExportCardsCSV` / `TenantImportCardsCSV` + Cards.vue 导出/导入对话框 + 路由 + API）
- ✅ 设备强制下线（card.go `TenantBanCard` 联动 `heartbeat.Remove` 清 Redis 心跳 + DB 标记 banned）
- ✅ 安装向导（install.go `InstallStatus` / `Install` + Install.vue 4 步向导 + 路由，替代原 seed 占位 hash + 后置脚本）
- ✅ 代理注册付费流程（方案 B 先支付后建 Agent：auth.go `AgentRegister` / `AgentRegisterConfig` / `AgentRegisterOrderStatus` + pay.go `dispatchPaidOrder` 前缀分发 + `processAgentRegisterPaid` 事务建 Agent + 邀请码状态机闭环 + Register.vue 落地 3 处 TODO + 修复 install.go 配置键名 bug `agent.register_fee` → `agent.register.fee`）
- ✅ 开发者自有易支付回调（pay.go `EpayTenantNotify` 完整实现 + `processTenantOwnPaidOrder` 事务 + `loadTenantPayConfig` AES 解密 + Redis 防重入按 tenant_id 命名空间隔离）
- ✅ 双层支付模式切换（`CreatePayOrder` 内 `SysPackage.AllowCustomPay` + `TenantPayConfig.Enabled` 双开关，TOP/ORD/REG 前缀分发，响应新增 `pay_mode` 字段）
- ✅ 修复 `002_seed_data.up.sql` 配置键名 bug：`pay.platform.notify_path` 与 router 不一致 → `/api/v1/pay/notify/epay`，新增 `pay.tenant.notify_path` / `pay.platform.order_name_prefix` / `pay.platform.return_front_url` 三个配置项
- ✅ 客户端 SDK 三语言（`sdks/python/` keyauth-py + `sdks/nodejs/` keyauth-node + `sdks/php/` keyauth-php，各封装 9 个验证 API + HMAC-SHA512/256 签名算法 + KeyAuthError 异常体系 + 完整 README 文档）
- ✅ 单元测试 + 客户端 SDK 签名对齐测试（5 个测试包：`pkg/crypto` + `pkg/snowflake` + `pkg/epay` + `internal/quota` + `internal/heartbeat`，全部 PASS；跨语言签名对齐测试 `pkg/crypto/sign_alignment_test.go` + `sdks/tests/sign.{py,js,php}`，Python + PHP 全 PASS，Node.js 沙箱 OpenSSL 限制 t.Skipf；`go vet ./...` + `go build ./...` 通过）
- ✅ 中间件层单元测试（`internal/middleware/middleware_test.go` 21 个测试全 PASS：JWTAuth 7 + TenantScope 3 + SignatureAuth 7（含 Nonce 防重放/时间戳容差/AES 解密 sign_secret 端到端闭环）+ RateLimitByIP 4（含 Redis 故障 fail-open）+ IPBlacklist 2 + RecordCardFailure 3 + Response 2 + GenerateToken RoundTrip 1；用 `httptest.NewRecorder` + `gin.TestMode` + `mockConfigReader` + miniredis + SQLite 内存库）
- ✅ 文档全量同步对齐 v0.3.6 实际状态（README/PROMPT/PROJECT/SPEC/TODO/CHANGELOG 六份联动更新）

v0.4.0 已新增完成（进行中）：
- ✅ UA 解析库迁移（`pkg/ua` 自实现零第三方依赖 + 20 个测试全 PASS；handler 层 `parseDeviceName` / `detectDeviceType` / `ListLoginDevicesFull` 全部接入；ListLoginDevices 响应新增 `os`/`os_version`/`browser`/`browser_version`/`is_bot` 字段，不改 DB schema 向前兼容；移除 profile.go「待核实 v0.4.x：引入更完整的 UA 解析库」标注）
- ✅ JWT jti 精准单点踢出（jti 嵌入 JWT `RegisteredClaims.ID` + `auth.BlacklistRefreshTokenByJTI` + `revokeSessionByJTI`；`KickDevice` / `Logout` / `RefreshToken` 轮换全部改造为 jti 维度，仅失效指定会话不影响其他设备；`ChangePassword` / `Disable2FA` 仍走 user 维度 `BlacklistRefreshToken` 强制全部重登；`internal/auth/jwt_test.go` 18 个测试 + `internal/middleware/middleware_test.go` 新增 `TestJWTAuth_JTI注入上下文`，8 个测试包全 PASS；兼容旧 token：无 jti 时 `IsRefreshTokenBlacklisted` 回退 user 维度）
- ✅ 2FA backup_codes DB 持久化（migration 008 三表 sys_admin/sys_tenant/agent 加 `backup_codes VARCHAR(512)` 字段 + model struct 同步；profile.go 新增 `loadUserBackupCodes`/`updateUserBackupCodes`/`consumeBackupCode` 三函数；Verify2FA 改为 DB 落库 + Disable2FA 清空字段；兼容 v0.3.x 老用户：DB 字段为空时 `loadUserBackupCodes` 自动回退 Redis 读取，首次 `consumeBackupCode` 消费成功后回写 DB + 清理 Redis 老数据；`internal/handler/profile_2fa_test.go` 13 个测试全 PASS）
- ✅ 登录失败日志结构化（新建 `internal/logger` 包基于 Go 1.21+ 标准库 `log/slog`，零第三方依赖，取代 zap/zerolog；`Options{Level, Format, Output}` + `Init/Debug/Info/Warn/Error` + 4 个 Ctx 版本；`AppConfig` 加 LogLevel/LogFormat/LogOutput；`cmd/main.go` 启动时调用 `logger.Init`；`session.go` + `log_worker.go` 3 处 `_ = err` 静默丢弃替换为 `logger.Error("xxx write failed", "err", err, ...业务字段...)` 结构化日志，移除 3 处「待核实 v0.4.x：引入结构化日志记录此错误」标注；`internal/logger/logger_test.go` 6 个测试全 PASS；10 个测试包全绿）
- ✅ 全语言 SDK 扩展（v0.4.0 第五项迁移）：新增 5 个 SDK（`sdks/go/` keyauth-go 用 `crypto/sha512.New512_256` 原生字节级对齐 + 强类型 struct 返回 + 零第三方依赖；`sdks/java/` keyauth-java 用 JDK 11+ HttpClient + `HmacSHA512/256`（JDK 17+，回退 HmacSHA256）+ Jackson + Maven 工程；`sdks/csharp/` keyauth-csharp 用 .NET 6+ HttpClient + 反射探测 BouncyCastle 启用 SHA-512/256 否则回退 HMACSHA256 + System.Text.Json；`sdks/cpp/` keyauth-cpp 用 C++17 + libcurl + OpenSSL 1.1+ `EVP_sha512_256` 原生对齐（OpenSSL < 1.1 回退 `EVP_sha256`）+ nlohmann/json + CMake FetchContent；`sdks/epl/` keyauth-epl 易语言纯中文 API + 精易模块 v9.0+ 依赖 + HMAC-SHA256（易语言生态无 SHA-512/256，仅在后端回退场景匹配））；`pkg/crypto/sign_alignment_test.go` 从 3 语言扩展到 7 语言自动化（Python/Node/PHP/Go 解释器模式 + C++ g++ 编译 + Java JDK 11+ 单文件源码模式 + C# dotnet 临时项目 + 易语言 Windows-only 永久 `t.Skip`）+ 新增 `TestSignAlignment_NewLanguages` 5 个 SDK 目录结构元数据校验（CI 友好，无运行时依赖）；JDK 17+ 才断言签名匹配，否则 `t.Logf` 暴露 mismatch 而非 `t.Skip` 掩盖；11 个测试包全绿，`go vet ./...` + `go build ./...` 通过）

v0.3.5 已完成（基线）：
- ✅ 后端 Go 项目结构（main / config / model / middleware / handler / router / quota / migration / heartbeat）
- ✅ 26 张基础表 + 4 张扩展表（log_login_failed / refresh_token_device / tenant_balance_log / tenant_withdraw）共 30 个 GORM struct，7 套 migration（001~007）
- ✅ 后端业务 API 全量实现：17 个 handler 文件，143 条路由，三角色 dashboard/profile/CRUD/2FA/登录设备全部真实实现，无 501 占位
- ✅ 三角色前端：45 个 .vue 页面，BasicLayout + AdminLayout/TenantLayout/AgentLayout + 官网 + H5，全部响应式
- ✅ 平台总支付闭环（彩虹易支付：下单/回调/同步跳转/自动发卡/防重入/超时关闭/抽成结算）
- ✅ 代理体系闭环（邀请码 + 充值审核 + 余额扣款生成卡密 + 佣金计算 + 提现审核 + 三级通知）
- ✅ 开发者结算与对账闭环（balance/frozen_balance + 批量结算 + 对账报表 + 双审核页面）
- ✅ 日志系统（验证/操作异步 worker + 三表独立查询 + UTF-8 BOM CSV 导出）
- ✅ 套餐配额统一封装（quota 包：CheckMaxApps/MaxCards/MaxAgents/MaxDevices）
- ✅ 轻量级 SQL 文件迁移机制（schema_migrations 表 + dirty 状态 + 单迁移事务）
- ✅ H5 公共 API（PublicAppInfo + PublicCardTypes，购卡闭环）
- ✅ Docker Compose + 宝塔部署 + RSA-4096 密钥生成独立脚本

**v0.3.6 已完成（2026-07-20）**：剩余 P1 收尾 + 单元测试 + 客户端 SDK 签名对齐测试，全部完成。

**v0.4.0 进行中（2026-07-20）**：UA 解析迁移 + JWT jti 精准单点踢出 + 2FA backup_codes DB 持久化 + 登录失败日志结构化 slog + 全语言 SDK 扩展 5 项迁移全绿（11 个测试包全 PASS），后续推进多级代理 / 在线更新 / 数据备份恢复 / 监控告警 / 通知系统 / 终端用户体系 / API 开放平台。

**v0.4.0（三期商业化，待开始）**：
- 多级代理 + 在线更新 + 数据备份恢复 + 监控告警 + 通知系统 + 终端用户体系 + API 开放平台

详细任务见 `docs/TODO.md`。

## 六、开发工作流

1. **接到新需求时**：先阅读 `docs/TODO.md`，确认优先级与依赖
2. **写代码前**：阅读对应铁律 + SPEC 规范，确认涉及哪些表与配置
3. **写代码时**：所有可调参数通过 `cfgCache.GetString("key", "默认值")` 读取
4. **写完后**：
   - 同步更新 `docs/CHANGELOG.md` 与 `docs/TODO.md`
   - 涉及新表/字段：补 migration SQL
   - 涉及新配置项：在 `002_seed_data.up.sql` 中追加默认值
   - 涉及新接口：在 SPEC.md 中补 API 文档
5. **提交前**：检查铁律合规性（grep 是否有硬编码 / 是否有待核实标注）

## 七、关键配置键速查

| 业务场景 | 配置键 | 默认值 |
|---|---|---|
| 心跳间隔 | `verify.heartbeat.interval` | 60 |
| IP 全局限流 | `security.rate.limit_global` | 100 |
| IP 自动封禁阈值 | `security.ban.threshold` | 50 |
| 平台易支付网关 | `pay.platform.gateway_url` | （待填写） |
| 平台抽成比例 | `pay.platform.commission_default` | 5.00 |
| 代理注册费 | `agent.register.fee` | 99.00 |
| 最低提现金额 | `agent.min_withdraw_amount` | 100.00 |
| 平台公告横幅开关 | `notice.platform.banner_enabled` | 1 |

完整列表见 `apps/server/migrations/002_seed_data.up.sql`。

## 八、调试技巧

**本地启动后端**：
```bash
cd apps/server
cp ../../configs/config.yaml.example ../../configs/config.yaml
# 修改 configs/config.yaml 中 mysql/redis 指向本地
go run ./cmd --config=../../configs/config.yaml
```

**本地启动前端**：
```bash
cd apps/admin
npm install
npm run dev   # 默认 http://localhost:5173
```

**查看 SQL 日志**：MySQL 配置中开启了慢查询日志（>2s），通过 `docker logs keyauth-mysql` 查看。

**重置超管密码**：
```bash
bash scripts/reset_admin_password.sh NewPass@2026
```

## 九、可信度声明

本项目中以下内容已标注「待核实」（铁律 06）：
- `pkg/crypto/crypto.go` 中 HMAC-SHA256 使用 `sha512.New512_256` 变体，需验证与客户端 SDK 一致性
- `pkg/snowflake/snowflake.go` 中 `twepoch = 1767225600000`（2026-01-01 UTC）
- `migrations/002_seed_data.up.sql` 中默认超管密码哈希为占位，部署后通过 `/install` 向导重置（v0.3.6 替代原"占位 hash + 后置脚本"方案）
- `scripts/reset_admin_password.sh` 中后端 `--reset-admin-password` subcommand 需在 main.go 实现

> v0.3.6 已修复：原 `card.go:422` 设备强制下线 TODO 已实现（联动 heartbeat.Remove）；卡密 CSV 导入导出已实现；原 `auth.go:443` AgentRegister 501 占位已实现（方案 B 先支付后建 Agent）；Register.vue 三处 TODO 已落地（读配置+调起支付+查询订单状态）；install.go 配置键名 bug 已修复（`agent.register_fee` → `agent.register.fee` 与 seed 002 对齐）；原 `pay.go:528` `EpayTenantNotify` 占位 `c.String(200, "fail")` 已实现完整回调流程（含 `processTenantOwnPaidOrder` 事务 + `loadTenantPayConfig` AES 解密）；双层支付模式切换已生效（`CreatePayOrder` 内 `SysPackage.AllowCustomPay` + `TenantPayConfig.Enabled` 双开关，TOP/ORD 前缀分发）；`002_seed_data.up.sql` 中 `pay.platform.notify_path` 与 router 不一致 bug 已修复；客户端 SDK 三语言已发布（`sdks/python/` + `sdks/nodejs/` + `sdks/php/`，9 个验证 API + HMAC-SHA512/256 签名算法 + KeyAuthError 异常体系，PHP `php -l` 校验通过）。

> 客户端 SDK 签名算法「待核实」项更新（v0.3.6 已通过单元测试部分验证）：三语言 SDK 优先用 `sha512/256` 算法（与后端 `crypto.HMACSHA256` 的 `sha512.New512_256` 变体对齐），运行时不支持时回退标准 `sha256`。**已验证**：Python + PHP 在 sha512/256 可用环境下与后端签名完全一致（`sdks/tests/sign.py` + `sign.php` + `pkg/crypto/sign_alignment_test.go`，3 组固定输入全 PASS）；**待核实**：sha256 回退分支与 sha512.New512_256 不等价（已确认两者输出不同，回退会导致签名校验失败），Node.js 沙箱环境 OpenSSL 不支持 sha512/256，已 `t.Skipf` 标注「环境限制」，待生产环境 Node.js 验证。

> v0.4.0 已落地：UA 解析库迁移完成（`pkg/ua` 自实现零第三方依赖 + 20 个测试全 PASS；handler 层 `parseDeviceName` / `detectDeviceType` / `ListLoginDevicesFull` 全部接入；ListLoginDevices 响应新增 `os` / `os_version` / `browser` / `browser_version` / `is_bot` 字段，不改 DB schema 向前兼容）；profile.go 中原「待核实 v0.4.x：引入更完整的 UA 解析库」标注已移除。

> v0.4.0 已落地：JWT jti 精准单点踢出完成（`internal/auth` 包新增 `BlacklistRefreshTokenByJTI` + `IsRefreshTokenBlacklisted` 改造为双维度优先 jti；`internal/middleware/auth.go` `JWTAuth` 注入 `c.Set("jti", claims.ID)` + `GenerateToken` 保留 `claims.ID` 修复 jti 丢失 bug；`internal/handler/auth.go` Login/RegisterTenant/RefreshToken/Logout 全部生成并传递 jti；`internal/handler/session.go` `recordLoginSession` 增加 jti 参数 + 新增 `revokeSessionByJTI`；`internal/handler/profile.go` KickDevice 注释更新为「v0.4.0 已支持精准单点踢出」，ChangePassword/Disable2FA 故意保留 user 维度；18 个 auth 测试 + 1 个 middleware JTI 注入测试全 PASS，8 个测试包全绿；兼容旧 token：无 jti 回退 user 维度）；session.go 中原「待核实 v0.4.x：将 jti 嵌入 JWT 后改为只黑名单指定 jti」标注已移除。

> v0.4.0 已落地：2FA backup_codes DB 持久化完成（`migrations/008_v0.4.0_2fa_backup_codes.up.sql` 三表 sys_admin/sys_tenant/agent 加 `backup_codes VARCHAR(512) NOT NULL DEFAULT ''` 字段 + `internal/model/model.go` 三表 struct 同步加 `BackupCodes` 字段；`internal/handler/profile.go` 新增 `loadUserBackupCodes`/`updateUserBackupCodes`/`consumeBackupCode` 三函数，Verify2FA 第 4 步从 Redis 持久化改为 DB 字段写入 + 清理 Redis 老数据，Disable2FA 第 5 步清空 DB `backup_codes` 字段 + 清理 Redis；兼容 v0.3.x 老用户：`loadUserBackupCodes` DB 为空时回退 Redis 读取，`consumeBackupCode` 消费成功后回写 DB + 清理 Redis 老数据；`internal/handler/profile_2fa_test.go` 13 个测试全 PASS，覆盖 DB 读取 / Redis 回退 / 消费 / 边界 / role 分支 / 兼容路径全场景）；profile.go 中原「待核实 v0.4.x：备用码理想方案为 bcrypt 哈希入库，当前简化用 AES 加密存 Redis，后续 v0.4.x 加 backup_codes 字段后迁移」标注已移除；session.go 中原「待核实 v0.4.x：引入结构化日志记录此错误」标注已移除（log_worker.go 同步移除）。

> v0.4.0 已落地：登录失败日志结构化完成（新建 `internal/logger` 包基于 Go 1.21+ 标准库 `log/slog`，零第三方依赖取代 zap/zerolog；`Options{Level, Format, Output}` + `Init` 用 `atomic.Value` 并发安全切换 + `L()` / `Debug/Info/Warn/Error` + 4 个 `DebugCtx/InfoCtx/WarnCtx/ErrorCtx` 链路追踪版本；`internal/config/config.go` `AppConfig` 加 `LogLevel`/`LogFormat`/`LogOutput` 三个 yaml 字段；`cmd/main.go` 启动时调用 `logger.Init(logger.Options{...})` 从 config 注入；`internal/handler/session.go` `StartLoginFailureWorker` + `internal/handler/log_worker.go` `StartVerifyLogWorker` + `StartOperationLogWorker` 3 处 `_ = err` 静默丢弃替换为 `logger.Error("xxx write failed", "err", err, ...业务字段...)` 结构化日志；`internal/logger/logger_test.go` 6 个测试全 PASS：parseLevel 4 级别 + 大小写 + 默认值 / JSON 格式 level/msg/字段断言 / level=warn 过滤 debug+info / text 格式 msg 含空格自动加引号 / L() 非 nil / 空 Options 不 panic）。

> v0.4.0 已落地：全语言 SDK 扩展完成（`sdks/go/keyauth/keyauth.go` 用 `crypto/sha512.New512_256` 原生字节级对齐后端 + 9 个客户端 API 强类型 struct 返回 + 零第三方依赖；`sdks/java/src/main/java/com/keyauth/sdk/KeyAuthClient.java` 用 JDK 11+ HttpClient + Jackson + `Mac.getInstance("HmacSHA512/256")`（JDK 17+，回退 `HmacSHA256`）+ Maven 工程 + `KeyAuthException(code, message, httpStatus)`；`sdks/csharp/src/KeyAuth/KeyAuthClient.cs` 用 .NET 6+ HttpClient + `SHA512_256Provider` 类反射探测 BouncyCastle（`Org.BouncyCastle.Crypto.Digests.Sha512_256Digest`）启用 SHA-512/256 否则回退 `HMACSHA256` + System.Text.Json + 9 个 async API 返回 `Task<JsonElement>`；`sdks/cpp/include/keyauth/keyauth.hpp` + `sdks/cpp/src/keyauth.cpp` 用 C++17 + libcurl + OpenSSL 1.1+ `EVP_sha512_256` 原生对齐（OpenSSL < 1.1 回退 `EVP_sha256` + stderr 警告）+ nlohmann/json + CMake FetchContent 自动下载 + `keyauth::KeyAuthException`；`sdks/epl/keyauth_sdk.e.txt` 易语言纯中文 API + 精易模块 v9.0+ 依赖 + HMAC-SHA256（易语言生态无 SHA-512/256 实现，与后端算法不同，仅在后端 crypto.go:165 回退场景下匹配，已明确标注 mismatch）；`pkg/crypto/sign_alignment_test.go` 从 3 语言扩展到 7 语言自动化：Python/Node/PHP/Go 走解释器模式（`go run`）+ C++ 走 `g++` 编译 + Java 走 JDK 11+ 单文件源码模式（`java Sign.java`）+ C# 走 `dotnet` 临时项目编译运行 + 易语言永久 `t.Skip`（Windows-only，CI 无法运行）；新增 `TestSignAlignment_NewLanguages` 元数据校验测试（5 个新 SDK 目录结构 + README + 入口文件存在性 + 签名函数实现，CI 友好无运行时依赖）；JDK 版本检测：仅 JDK 17+ 才断言签名匹配，否则 `t.Logf` 暴露 mismatch 而非 `t.Skip` 掩盖；C# 在 fallback 场景也用 `t.Logf` 而非 `t.Skip`；11 个测试包全绿，`go vet ./...` + `go build ./...` 通过）。**待核实**：易语言 SDK 仅在后端 `crypto.go:165` sha256 回退分支下匹配，生产环境后端使用 `sha512.New512_256` 时签名不一致，需在易语言客户端文档中明确告知用户后端必须配置 sha256 回退；C# / Java 在 fallback 场景下与后端 sha512.New512_256 也不等价，需用户安装 BouncyCastle（C#）或 JDK 17+（Java）才能保证签名匹配。
