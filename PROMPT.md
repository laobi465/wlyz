# PROMPT.md —— AI 接手指引

> 当新的 AI / 开发者接手本项目时，按本文件指引快速进入工作状态。

## 一、项目背景速读（2 分钟）

本项目是 **KeyAuth SaaS**：面向开发者的多租户卡密验证 SaaS 平台。

**核心定位**：开发者注册账号 → 创建应用 → 生成卡密 → 客户端 SDK 接入验证。代理通过开发者邀请码注册并分销卡密。平台提供总支付（默认）与开发者自定义易支付（按套餐）双轨支付。

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

**v0.2.0（骨架阶段）已完成**：
- ✅ 后端 Go 项目结构（main / config / model / middleware / handler / router）
- ✅ 26 张表 DDL + 默认 seed 数据
- ✅ 三套前端布局 + 登录页 + 代理注册页 + 404
- ✅ Docker Compose + 宝塔部署脚本

**v0.2.0 待实现**：
- ⏳ 后端各 handler 业务逻辑（当前均为 501 占位）
- ⏳ 前端各业务页面（当前均为 PlaceholderView 占位）
- ⏳ 客户端 SDK（Go / C# / Python 三语言）
- ⏳ 单元测试 + 集成测试

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
- `migrations/002_seed_data.up.sql` 中默认超管密码哈希为占位，部署后必须重置
- `scripts/reset_admin_password.sh` 中后端 `--reset-admin-password` subcommand 需在 main.go 实现
- 前端各 `TODO` 注释处的真实接口对接
