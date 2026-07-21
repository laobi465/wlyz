# KeyAuth SaaS

> 面向开发者的多租户卡密验证 SaaS 平台

[![Version](https://img.shields.io/badge/version-0.6.2-blue.svg)](docs/CHANGELOG.md)
[![License](https://img.shields.io/badge/license-Proprietary-red.svg)](#许可证)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4+-42b883.svg)](https://vuejs.org)
[![Deploy](https://img.shields.io/badge/deploy-one--click-success.svg)](#一键部署)
[![Security](https://img.shields.io/badge/security-audit%20passed-brightgreen.svg)](#安全审计)
[![MySQL](https://img.shields.io/badge/MySQL-8.0.36-4479A1.svg)](https://www.mysql.com/)

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

### v0.6.2 重要修复（dirty 迁移恢复 + MySQL 8.0 兼容）

v0.6.2 修复了 v0.6.0/v0.6.1 在 MySQL 8.0 上一键部署失败的 critical bug：

- **根因**：`migration 015` 使用了 MariaDB-only 语法 `ADD COLUMN IF NOT EXISTS` / `ADD INDEX IF NOT EXISTS`，MySQL 8.0 任何小版本都不支持，导致部署必然失败并留下 `schema_migrations.dirty=1` 状态
- **修复 1**：重写 `migration 015` 全部改用 `INFORMATION_SCHEMA + PREPARE/EXECUTE` 兼容方案，保证幂等可重试
- **修复 2**：迁移器新增 `MIGRATION_REPAIR_DIRTY=true` 显式 dirty 恢复流程 + MySQL advisory lock 并发保护
- **修复 3**：一键部署脚本移除破坏性 `DELETE FROM schema_migrations` 操作，改为自动备份 + 走幂等修复流程
- **修复 4**：`mysql:8.0` → `mysql:8.0.36` 固定小版本，避免版本漂移

**已有数据安全修复命令**（不删除数据卷）：

```bash
cd /www/wwwroot/keyauth
git pull origin main
bash scripts/one_click_deploy.sh
# 脚本会自动检测 dirty → 备份 → 走 MIGRATION_REPAIR_DIRTY=true 修复流程
```

详见 [v0.6.2 CHANGELOG](docs/CHANGELOG.md#062---2026-07-21dirty-迁移恢复--mysql-80-兼容修复)。

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
| 9 | **初始化超管账号 admin/admin123** | 检测占位 hash → 自动生成 bcrypt 写入 → 验证 |
| 10 | **生成部署信息 txt** | 所有信息写入 `/root/keyauth_deploy_info.txt`（chmod 600） |

**部署完成后自动生成的 txt 文件包含**：
- 宝塔面板入口（地址/账号/密码/端口）
- KeyAuth 访问地址（前端后台 + API）
- **超管账号 admin/admin123**（首次部署自动初始化，登录后请立即改密）
- 所有密钥（MySQL/AES/JWT）
- 服务状态、运维命令、备份建议
- **第七章：宝塔反代 + 免费 SSL 完整教程**（6 步 + 常见问题）

**部署完成后必做**：
1. `cat /root/keyauth_deploy_info.txt` 查看完整部署信息（含反代教程）
2. 用 **admin/admin123** 登录后台 → 立即修改默认密码
3. 域名 A 记录解析到本服务器 IP
4. 宝塔面板「网站」添加站点 → 反向代理到 `http://127.0.0.1:8081`
5. 宝塔面板「SSL」申请 Let's Encrypt 免费证书 + 强制 HTTPS
6. 宝塔面板「安全」关闭 8081/8080/3306/6379 公网端口
7. 登录后台「系统配置 > 支付」配置易支付参数

> 脚本严格遵守三铁律：禁硬编码（密钥现场生成）/ 配置走 sys_config / 反幻觉（脚本含详细日志和错误处理）。完整说明见 [scripts/one_click_deploy.sh](scripts/one_click_deploy.sh)。

## 当前版本（v0.6.2 安全审计 + dirty 迁移恢复 + MySQL 8.0 兼容修复完成）

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
| 客户端 SDK（7 语言全栈对齐） | ✅ 已完成 v0.4.0 | `sdks/python/` + `sdks/nodejs/` + `sdks/php/` + `sdks/go/` + `sdks/java/` + `sdks/csharp/` + `sdks/cpp/` + `sdks/epl/`（易语言）+ HMAC-SHA512/256 签名对齐测试 |
| v0.4.x 16 项迁移全绿 | ✅ 已完成 v0.4.0 | UA 解析 + JWT jti 精准踢出 + 2FA 备用码 DB 持久化 + 日志结构化 slog + 全语言 SDK + 多级代理 + 灰度发布 + 在线更新 + 备份恢复 + 监控告警 + 通知系统 + 终端用户体系 + API 开放平台 + 更新弹窗 + 高级安全 + 公告增强 + 数据统计；17 个测试包全 PASS |
| **全项目安全审计 P0 + P2 修复** | ✅ 已完成 v0.6.0 | 13 P0 高危 + 15 P2 联调（部署链路 5 bug + 认证绕过 + SQL 注入 + 权限提升 + 敏感信息泄露 + 并发竞态 + 字段映射 + 枚举对齐 + 分页参数）|
| **全项目安全审计 P1 + P3 修复** | ✅ 已完成 v0.6.1 | 21 P1 普通 + 34 P3 优化（JWT Subject 校验 + Nonce 顺序 + IP 黑名单 fail-open + CF IP 伪造 + v-html XSS + Cookie Secure + 充值 FOR UPDATE + 提现流水精确匹配 + HMAC 名实一致 + TOTP skew + 30 处错误泄露 + 4 处 N+1 查询 + 4 处 HTTP 超时）|
| **dirty 迁移恢复 + MySQL 8.0 兼容修复** | ✅ 已完成 v0.6.2 | **根因**：v0.6.0/v0.6.1 migration 015 使用 MariaDB-only 语法 `ADD COLUMN/INDEX IF NOT EXISTS`，MySQL 8.0 必然失败并留下 `schema_migrations.dirty=1`；**修复 1** 重写 `015_v0.4.0_end_user_system.up.sql` 全部改用 `INFORMATION_SCHEMA + PREPARE/EXECUTE` 兼容方案 + `INSERT ON DUPLICATE KEY UPDATE` 幂等；**修复 2** `migrator.go` 新增 `MIGRATION_REPAIR_DIRTY=true` 显式 dirty 恢复流程 + MySQL advisory lock（`GET_LOCK`/`RELEASE_LOCK`）并发保护 + 详细错误消息（dirty 版本 + 迁移文件 + DBTarget + 备份建议 + 禁止行为清单）；**修复 3** `one_click_deploy.sh` 新增 `--reset-data` 显式确认 + 移除破坏性 `DELETE FROM schema_migrations` + 自动备份 + 自动走幂等修复流程 + MySQL 健康检查（mysqladmin ping + SQL 双校验，120s 超时）+ 失败诊断（`docker compose ps` / mysql 日志 / server 日志 / `schema_migrations` 状态）；**修复 4** `clean_dirty_migration.sh` 完全重写为 `--show` / `--dry-run` / `--repair` / `--force-delete` 四模式 + 检查 v15 对应所有对象 + 强制备份 + 显式禁止 `docker compose down -v`；**修复 5** `mysql:8.0` → `mysql:8.0.36` 固定小版本；6 个 Go 单元测试全 PASS + SQL 静态验证 10/10 通过 + shellcheck -S warning 通过 + YAML 语法有效；MySQL 集成测试 13 个用例已编写待在真实 MySQL 8.0 环境运行 |

详细变更见 [CHANGELOG.md](docs/CHANGELOG.md)，任务进度见 [TODO.md](docs/TODO.md)。

## 技术栈

| 层级 | 技术选型 |
|---|---|
| 后端 | Go 1.22 + Gin + GORM |
| 前端 | Vue 3.4 + TypeScript + Element Plus + Vite + Pinia |
| 数据库 | MySQL 8.0.36（v0.6.1 固定小版本，避免漂移）+ Redis 7 |
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
│   ├── one_click_deploy.sh        # SSH 一键自动化部署（推荐，v0.6.1 增加 --reset-data + dirty 自动修复）
│   ├── baota_deploy.sh            # 宝塔面板手动部署
│   ├── clean_dirty_migration.sh   # dirty 迁移修复专用脚本（v0.6.1 新增 --show/--dry-run/--repair/--force-delete 四模式）
│   ├── verify_migration_015.sh    # migration 015 静态验证（v0.6.1 新增，10 项兼容性检查）
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

# AES-256 密钥：必须正好 32 字节字符串（不是 base64 解码后 32 字节）
openssl rand -hex 16
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

## 安全审计

本项目已完成全项目安全审计，共发现并修复 **82 个 bug**（4 类优先级全覆盖）：

| 优先级 | 类别 | 数量 | 状态 | 修复内容 |
|---|---|---|---|---|
| **P0** | 高危 | 13 | ✅ v0.6.0 | 部署链路 5 bug + 认证绕过 + SQL 注入 + 权限提升 + 敏感信息泄露 + 并发竞态 |
| **P1** | 普通 | 21 | ✅ v0.6.1 | Migration 兼容性 + 加密名实一致 + JWT Subject + Nonce 顺序 + IP 黑名单 + CF IP 伪造 + v-html XSS + Cookie Secure + 充值 FOR UPDATE + 提现流水精确匹配 + 版本号比较 + TOTP skew |
| **P2** | 联调 | 15 | ✅ v0.6.0 | 字段映射 + 枚举对齐 + 分页参数 + 云变量字段 + 收入趋势 + Top 应用 + 邀请码 + 设备 location + 支付配置 + 版本 channel + 公告 type/status + 佣金 type/status |
| **P3** | 优化 | 34 | ✅ v0.6.1 | 30 处错误信息泄露 + 4 处 N+1 查询 + 4 处 HTTP 客户端超时 |

### 安全审计亮点

**[认证与授权加固]**
- JWT 强制校验 `Subject == "access"`，refresh token 不能访问业务接口
- SignatureAuth Nonce 防重放移到签名校验之后，防 Redis 命名空间污染
- publicGroup 公开端点（登录/注册/refresh）统一挂限流，防暴力枚举
- IPBlacklist Redis 故障时回退 MySQL，避免 fail-open
- CloudflareRealIP trustedCIDRs 为空时回退 `c.ClientIP()`，防 CF 头伪造

**[业务逻辑修复]**
- 充值审核事务内对 agent 行加 `FOR UPDATE` 锁，防并发双倍加余额
- 提现审核流水新增 `related_withdraw_id` 字段精确匹配，防错配
- 月费订单查询条件不限 `pay_status`，防重复扣费
- AdminReconciliation `tenant_id` 条件传入 stats 查询，超管按开发者过滤生效
- ClientVersion 用 `compareVersions` 逐段数值比较，修复字典序错误

**[加密与签名]**
- `crypto.decodeSegment` 改用 `crypto/rand.Int` 拒绝采样，消除模偏差
- `HMACSHA256` 改用标准 SHA-256 实现名实一致，新增 `HMACSHA512_256` 保留 SDK 兼容
- `update.ReleaseLock` 加 UUID token + Lua 脚本原子比较删除，防误删他人锁
- `TOTP.ValidateTOTP` 用 `totp.ValidateCustom` 让 skew 参数真正生效

**[前端安全]**
- v-html 接入 DOMPurify 防 XSS 注入
- Cookie 补 `secure: import.meta.env.PROD` 防 HTTPS 嗅探
- 浮点金额改整数分计算避免 IEEE 754 误差
- H5 401 并发 refresh 增加独立队列防误登出

**[性能与可靠性]**
- 30 处 `err.Error()` 错误泄露改为 `logger.Error` + 通用消息
- 4 处 N+1 查询改为批量聚合（admin/tenant 列表）
- 4 处 HTTP 客户端增加 10s 超时，防下游不可用挂起

详细修复记录见 [CHANGELOG.md](docs/CHANGELOG.md) v0.6.0 和 v0.6.1 章节。

## 安全说明

- 所有敏感字段（AES_KEY / JWT_SECRET / RSA 私钥 / DB 密码）通过环境变量传入，不进代码
- 客户端 API 强制 HMAC-SHA256 签名 + nonce 防重放（签名校验后再写 nonce）
- 服务端响应可选 RSA-4096 签名（fail-closed）
- 密码 bcrypt cost=12，敏感字段 AES-256-GCM 加密
- IP 自动封禁机制（失败次数达阈值自动拉黑）+ Redis 故障回退 MySQL
- 心跳通过 Redis Sorted Set 维护，支持百万级在线设备
- 公开端点统一限流（默认 10 次/分钟，可后台调整）
- v-html 渲染前 DOMPurify sanitize 防 XSS
- Cookie 启用 Secure + SameSite 防 HTTPS 嗅探和 CSRF

## 后续版本规划

详见 [TODO.md](docs/TODO.md)：

- **v0.3.6（已完成 2026-07-20）**：剩余 P1 收尾（卡密 CSV 导入导出 + 设备强制下线 + 安装向导 + 代理注册付费流程 + 开发者自有易支付 + 双层支付模式切换 + 客户端 SDK 三语言 + 单元测试 + 跨语言签名对齐测试 + 文档同步）
- **v0.4.0（已完成 2026-07-20）**：UA 解析迁移 + JWT jti 精准单点踢出 + 2FA backup_codes DB 持久化 + 登录失败日志结构化 slog + 全语言 SDK 扩展 + 多级代理体系 + 灰度发布体系 + 在线更新体系 + 数据备份恢复 + 监控告警 + 通知系统 + 终端用户体系 + API 开放平台 + 管理员更新弹窗通知 + 高级安全 + 公告增强 + 数据统计 已完成（16 项迁移全绿；17 个测试包全 PASS）
- **v0.6.0（已完成 2026-07-20）**：全项目安全审计 P0 高危 13 个 + P2 联调 15 个修复（部署链路 + 认证 + SQL + 权限 + 并发 + 联调字段对齐）
- **v0.6.1（已完成 2026-07-20）**：全项目安全审计 P1 普通 21 个 + P3 优化 34 个修复（认证加固 + 业务逻辑 + 加密签名 + 前端安全 + 性能可靠性）
- **v0.6.2（已完成 2026-07-21）**：**Critical Bug 修复** —— Docker Compose 一键部署在 MySQL 8.0 上失败（`schema_migrations.dirty=1, version=15`）。根因：migration 015 使用 MariaDB-only 语法 `ADD COLUMN/INDEX IF NOT EXISTS`。修复内容：① migration 015 重写为 INFORMATION_SCHEMA + PREPARE/EXECUTE 兼容方案；② migrator.go 新增 `MIGRATION_REPAIR_DIRTY=true` 显式 dirty 恢复流程 + MySQL advisory lock（`GET_LOCK`/`RELEASE_LOCK`）并发保护；③ one_click_deploy.sh 移除破坏性 DELETE 操作，改为自动备份 + 幂等修复；④ clean_dirty_migration.sh 重写为四模式（show/dry-run/repair/force-delete）；⑤ mysql:8.0 → mysql:8.0.36 固定小版本；⑥ 新增 13 个迁移器测试用例（含 6 个单元测试 + 7 个集成测试，集成测试需 MySQL 8.0 环境）
- **后续**：analysis.go 等 ~40 处参数校验类错误泄露清理、openapi.go/enduser.go c.JSON 直接泄露 ~20 处清理（非阻断性，按需迭代）+ MySQL 8.0 真实环境集成测试验证

## 开发约束（铁律）

本项目严格遵守 `web-project-flow` skill（已全局安装）的三份铁律：

1. **禁硬编码**：密钥 / token / 域名 / 价格 / 接口地址 全部抽离到环境变量或 sys_config
2. **配置后台化**：所有可调参数走 `sys_config` 表 + Redis 缓存 + 后台可视化编辑
3. **防幻觉**：不确定处标注「待核实」，代码不确定处标注「需验证」

可通过 `/bhelp` 查看 skill 全部 11 份提示词索引；写业务代码前用 `/bhardcode /bconfig /bhaluc` 一次性加载三铁律；变更/加功能后用 `/bdocs` 触发四份核心文档联动更新。

## 许可证

Proprietary —— 未经授权禁止商用
