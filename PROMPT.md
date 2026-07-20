# PROMPT.md —— AI 接手指引

> 当新的 AI / 开发者接手本项目时，按本文件指引快速进入工作状态。
>
> 配套 Skill：`web-project-flow`（已全局安装）。输入 `/bhelp` 查看全部 11 份提示词索引；写代码前用 `/bhardcode /bconfig /bhaluc` 加载三铁律；变更后用 `/bdocs` 触发四份文档联动更新；接手项目用 `/bonboard`。

## 一、项目背景速读（2 分钟）

本项目是 **KeyAuth SaaS**：面向开发者的多租户卡密验证 SaaS 平台。

**核心定位**：开发者注册账号 → 创建应用 → 生成卡密 → 客户端 SDK 接入验证。代理通过开发者邀请码注册并分销卡密。平台提供总支付（默认）与开发者自定义易支付（按套餐）双轨支付。

**v0.3.6 已发布**：6 个测试包（crypto/snowflake/epay/quota/heartbeat/middleware）+ 跨语言签名对齐测试全部通过。运行 `cd apps/server && go test ./...` 验证。

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

**v0.4.0（三期商业化，待开始）**：
- 多级代理 + 全语言 SDK + 在线更新 + 数据备份恢复 + 监控告警 + 通知系统

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
