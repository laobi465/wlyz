# KeyAuth SaaS

> 面向开发者的多租户卡密验证 SaaS 平台

[![Version](https://img.shields.io/badge/version-0.6.8-blue.svg)](docs/CHANGELOG.md)
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

### v0.6.8 重要修复（UpdateNotifier setInterval 累积导致 /admin/dashboard 卡死）

v0.6.8 修复 P0 紧急 bug：v0.6.7 修复 scheduleRefresh 死循环后，管理员登录依然卡死，定位到第二个根因——UpdateNotifier 组件的 setInterval 累积。

- **症状**：v0.6.7 升级后管理员登录依然进不去 `/admin/dashboard`，浏览器卡死
- **根因**：`apps/admin/src/components/UpdateNotifier.vue` 的 `scheduleNext` 使用 `setInterval` 调度异步轮询：
  - `setInterval` 不等待 async 回调完成，当 `pollOnce()` 耗时超过 `interval` 时多个回调并发执行
  - `interval_seconds` 变化时执行 `stopPolling() + scheduleNext(next)` 创建新 setInterval，但**事件循环中排队中的旧回调仍会执行并创建更多 setInterval**，新 setInterval 引用覆盖 `timer.value`，旧 setInterval 永远无法被 `clearInterval` 清除 → 引用覆盖泄漏
  - 组件卸载时 `onBeforeUnmount` 调 `stopPolling()` 只能清除当前 `timer.value`，**排队中但尚未执行的 setInterval 回调在组件卸载后继续执行** → 内存泄漏 + 卡死
- **修复 1**：`scheduleNext` 改用 `setTimeout` 自递归——每次 `pollOnce()` 完成后才调度下一次，从根上消除并发（不再有多个回调同时运行）
- **修复 2**：`timer` 类型 `ReturnType<typeof setInterval>` → `ReturnType<typeof setTimeout>`，`stopPolling` 用 `clearTimeout`
- **修复 3**：新增 `isUnmounted` 标志位，`onBeforeUnmount` 中先标记 `isUnmounted = true` 再 `stopPolling`，setTimeout 回调开头与 await 之后双重检查，防止卸载后回调继续执行
- **修复 4**：`startPolling` 首次 await 期间防御性检查 `isUnmounted`，避免首次轮询未完成时组件已卸载还调度定时器
- **简化逻辑**：原 `if (next !== intervalSec) { stopPolling(); scheduleNext(next) }` 简化为始终 `scheduleNext(next)` 自递归（间隔变化时自动用新间隔，无需二步操作）

**已有数据安全修复命令**（不删除数据卷）：

```bash
cd /www/wwwroot/keyauth
git pull origin main
docker compose up -d --build admin
# 浏览器：清除 localStorage（DevTools → Application → Local Storage → 删除 keyauth-auth）后重新登录
# 或在 Console 执行：localStorage.clear() 然后刷新
```

详见 [v0.6.8 CHANGELOG](docs/CHANGELOG.md#068---2026-07-21updatenotifier-setinterval-累积导致-admindashboard-卡死修复)。

### v0.6.7 重要修复（管理员登录后死机 + token 续期死循环）

v0.6.7 修复 P0 紧急 bug：管理员登录后浏览器卡死。

- **症状**：管理员登录成功后页面卡住，CPU 飙升，浏览器最终死机
- **根因**：`apps/admin/src/stores/auth.ts` 的 `scheduleRefresh` 在 `delay <= 0` 时不 await 调 `doRefresh`，`doRefresh` 成功后又调 `scheduleRefresh`，若新 `expires_at` 仍让 `delay <= 0`（如 `jwt.access_ttl_seconds` 被配为 ≤ 300 秒、后端返回异常值、客户端时钟超前），形成无限异步递归，每秒发数十个 HTTP 请求
- **修复 1**：新增 `_refreshing` 并发锁，正在刷新时不重复触发，从结构上杜绝递归
- **修复 2**：`doRefresh` 校验后端返回的 `expires_at` 合法性（必须 > now + 60s），异常值直接跳过更新与重排
- **修复 3**：`scheduleRefresh` 最小延迟保护（兜底 30s），避免 sys_config 配置临界值触发高频刷新
- **修复 4**：`http.ts` 401 拦截器加固：`doRefresh` 后若 token 为空则登出，避免无效重试
- **修复 5**：后端 `AdminUpdateConfig` 新增 `jwt.access_ttl_seconds` (≥600s) 与 `jwt.refresh_ttl_seconds` (≥3600s) 最小值校验，从源头防止管理员配错触发死循环
- **登出时重置 `_refreshing`**，避免下次登录复用 stale 状态

**已有数据安全修复命令**（不删除数据卷）：

```bash
cd /www/wwwroot/keyauth
git pull origin main
docker compose up -d --build admin server
# 浏览器：清除 localStorage（DevTools → Application → Local Storage → 删除 keyauth-auth）后重新登录
# 或在 Console 执行：localStorage.clear() 然后刷新
```

详见 [v0.6.7 CHANGELOG](docs/CHANGELOG.md#067---2026-07-21管理员登录后死机--token-续期死循环修复)。

### v0.6.5 重要修复（前端登录跳转 + 后台进不去 + v0.6.4 GORM datetime 兼容 + v0.6.3 migration 030 列数 + v0.6.2 dirty 迁移恢复）

v0.6.5 在 v0.6.4 基础上追加修复前端登录跳转逻辑：

- **症状 1**：管理员登录后没有自动跳转后台页面
- **症状 2**：后台进不去（访问 `/admin/*` 报 404 或跳转到错误页面）
- **根因**：`auth.role` 为空字符串时（localStorage 持久化数据损坏 / 旧版本字段缺失 / 手动篡改），`homePath` 计算为 `//dashboard` → Vue Router 规范化为 `/dashboard` → 不匹配任何路由 → 404；路由守卫回退也用 `/${auth.role}/dashboard` 导致同样问题
- **修复 1**：`stores/auth.ts` 的 `homePath` getter 兜底，role 为空时返回 `/login`
- **修复 2**：`router/index.ts` 守卫新增 stale state 检测，已登录但 role 为空时强制 logout 回登录页
- **修复 3**：`views/login/index.vue` 改为 Promise 风格（原 callback 风格下 `await` 是 no-op，finally 立即执行）
- **修复 4**：`views/login/index.vue` 优先使用后端返回的 `resp.user.role`（权威来源），兜底用 UI Tab 选择的角色（防幻觉铁律）
- **修复 5**：`views/login/index.vue` redirect 白名单校验，只允许 `/admin /tenant /agent` 开头的相对路径，防止篡改跳到 404 或外部 URL
- **v0.6.4 修复**（前置）：GORM AutoMigrate datetime(3) 兼容
- **v0.6.3 修复**（前置）：migration 030 列数不匹配
- **v0.6.2 修复**（前置）：dirty 迁移恢复 + MySQL 8.0 兼容

**已有数据安全修复命令**（不删除数据卷）：

```bash
cd /www/wwwroot/keyauth
git pull origin main
docker compose up -d --build admin server
# 浏览器：清除 localStorage（DevTools → Application → Local Storage → 删除 keyauth-auth）后重新登录
# 或在 Console 执行：localStorage.clear() 然后刷新
```

详见 [v0.6.5 CHANGELOG](docs/CHANGELOG.md#065---2026-07-21前端登录跳转--后台进不去修复)、[v0.6.4 CHANGELOG](docs/CHANGELOG.md#064---2026-07-21gorm-automigrate-与-mysql-80-datetime3-兼容修复)、[v0.6.3 CHANGELOG](docs/CHANGELOG.md#063---2026-07-21migration-030-列数不匹配修复) 与 [v0.6.2 CHANGELOG](docs/CHANGELOG.md#062---2026-07-21dirty-迁移恢复--mysql-80-兼容修复)。

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

## 当前版本（v0.6.8 UpdateNotifier setInterval 累积卡死 P0 修复 + v0.6.7 管理员登录死机 P0 修复 + v0.6.6 并发安全加固 + v0.6.5 前端登录跳转 + v0.6.4 GORM datetime + v0.6.3 migration 030 + v0.6.2 dirty 迁移 完成）

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
| **migration 030 列数不匹配修复** | ✅ 已完成 v0.6.3 | **根因**：`030_v0.5.0_notify_webhook.up.sql` 的 INSERT 声明 6 列但每行 VALUES 写了 7 个值（多余空字符串），MySQL 报 `Error 1136 (Column count doesn't match value count at row 1)`；**修复** 移除多余字段，列顺序对齐 sys_config 表 schema；**全量扫描** Python 脚本验证所有 `migrations/*.up.sql` 的 `INSERT INTO sys_config` 列数一致性，031/032/033 全部匹配，仅 030 一处 bug |
| **GORM AutoMigrate datetime(3) 兼容修复** | ✅ 已完成 v0.6.4 | **根因**：`db.AutoMigrate(&model.SysConfig{})` 默认把 `time.Time` 推断为 `datetime(3)`（带毫秒），但 migration 001 用 `DATETIME`（无毫秒）建表，AutoMigrate 触发 `ALTER TABLE MODIFY COLUMN created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP`，在 MySQL 8.0 + sql_mode=STRICT_TRANS_TABLES 下报 `Error 1067 (Invalid default value for 'created_at')`；**修复** `SysConfig` struct 的 `CreatedAt`/`UpdatedAt` GORM tag 显式声明 `type:datetime`，让 GORM 不再修改列定义，保持 migration 001 schema 不变 |
| **前端登录跳转 + 后台进不去修复** | ✅ 已完成 v0.6.5 | **根因**：`auth.role` 为空字符串时（localStorage 持久化数据损坏 / 旧版本字段缺失 / 手动篡改），`homePath` 计算为 `//dashboard` → 404；路由守卫回退同样问题；**修复 1** `stores/auth.ts` homePath 兜底 role 为空返回 `/login`；**修复 2** `router/index.ts` 守卫检测 stale state 强制 logout；**修复 3** `login/index.vue` 改为 Promise 风格（原 callback 下 await 是 no-op）；**修复 4** 优先使用后端返回的 `resp.user.role`（防幻觉）；**修复 5** redirect 白名单校验只允许 `/admin /tenant /agent` 开头 |
| **充值/提现/支付回调并发安全加固** | ✅ 已完成 v0.6.6 | **DB 级幂等状态转换**：充值审核/提现审核/支付回调所有 `pending->settled/rejected/paid` 转换用 `Where(status=pending)+RowsAffected` 检查，第二个并发请求 RowsAffected=0 直接失败；**GORM clause.Locking 行锁**：10 处 `tx.Set("gorm:query_option", "FOR UPDATE")` 替换为 `tx.Clauses(clause.Locking{Strength: "UPDATE"})`，类型安全且可读；**frozen_balance 守门**：管理员打款/驳回时不足即事务失败，禁止强制设为 0；**支付回调 DB 级幂等**：Redis SETNX 仅辅助，DB 状态守门为最终保障（防 Redis 故障/失效窗口）；**AppliedAmount 审计字段**：migration 034 新增 `applied_amount` 列，审核时若调整 Amount 则保留原始申请金额供对账；**11 个并发/回滚测试全 PASS**（含 `-race` 竞态检测，覆盖 TenantApproveRecharge / TenantRejectWithdraw / TenantPayWithdraw / ProcessPaidOrder / AdminPayTenantWithdraw 全链路） |
| **管理员登录死机 P0 修复** | ✅ 已完成 v0.6.7 | **根因**：`stores/auth.ts` `scheduleRefresh` 在 `delay<=0` 时不 await 调 `doRefresh`，`doRefresh` 成功后又调 `scheduleRefresh`，若新 `expires_at` 仍让 `delay<=0`（`jwt.access_ttl_seconds≤300` 或后端异常返回）形成无限异步递归，每秒数十个 HTTP 请求导致浏览器死机；**修复 1** 新增 `_refreshing` 并发锁杜绝递归；**修复 2** `doRefresh` 校验新 `expires_at > now+60s`，异常值跳过；**修复 3** `scheduleRefresh` 最小延迟兜底 30s；**修复 4** `http.ts` 401 拦截器加固空 token 登出；**修复 5** 后端 `AdminUpdateConfig` 新增 `jwt.access_ttl_seconds≥600`/`jwt.refresh_ttl_seconds≥3600` 最小值校验，源头防止配错；**修复 6** `logout` 重置 `_refreshing`，避免 stale 状态 |
| **UpdateNotifier setInterval 累积卡死 P0 修复** | ✅ 已完成 v0.6.8 | **根因**：v0.6.7 修复 scheduleRefresh 死循环后管理员登录依然卡死，定位到第二个根因——`components/UpdateNotifier.vue` `scheduleNext` 使用 `setInterval` 调度 async 轮询：setInterval 不等待 async 回调完成 → pollOnce 耗时超过 interval 时多个回调并发执行；interval 变化时 stopPolling+scheduleNext 创建新 setInterval 但排队中的旧回调仍执行并创建更多 setInterval → 引用覆盖泄漏（timer.value 被覆盖，旧 setInterval 永远无法 clearInterval）；组件卸载后排队回调继续执行 → 内存泄漏；**修复 1** `scheduleNext` 改用 `setTimeout` 自递归（每次 pollOnce 完成后才调度下一次，从根上消除并发）；**修复 2** timer 类型 setInterval→setTimeout，stopPolling 用 clearTimeout；**修复 3** 新增 `isUnmounted` 标志位，onBeforeUnmount 先标记再 stopPolling，回调开头与 await 之后双重检查；**修复 4** `startPolling` 首次 await 期间防御性检查 isUnmounted |

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
| **专项** | 并发安全 | 11 测试 | ✅ v0.6.6 | DB 级幂等状态转换（Where status=pending + RowsAffected）+ GORM `clause.Locking{Strength:"UPDATE"}` 行锁（10 处）+ frozen_balance 守门（不足即事务失败）+ 支付回调 DB 级幂等（Redis 仅辅助）+ AppliedAmount 审计字段（migration 034）+ 11 个并发/回滚测试全 PASS（含 -race 竞态检测） |
| **专项** | 前端死循环 | 2 P0 | ✅ v0.6.7+v0.6.8 | v0.6.7 `scheduleRefresh` 无限异步递归修复：`_refreshing` 并发锁 + `expires_at` 合法性校验 + 最小延迟 30s 兜底 + 后端 sys_config 最小值校验（`jwt.access_ttl_seconds≥600`）；v0.6.8 `UpdateNotifier.scheduleNext` setInterval 累积修复：改用 `setTimeout` 自递归（每次 pollOnce 完成后才调度下一次）+ `isUnmounted` 标志位防止卸载后回调执行 + `startPolling` 首次 await 期间防御性检查 |

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
- **v0.6.3（已完成 2026-07-21）**：**Critical Bug 修复** —— migration 030 (`030_v0.5.0_notify_webhook`) 在 `INSERT INTO sys_config` 阶段报 `Error 1136 (Column count doesn't match value count at row 1)`。根因：INSERT 声明 6 列但每行 VALUES 写了 7 个值（多余空字符串）。修复：移除多余字段，列顺序对齐 sys_config 表 schema；Python 脚本全量扫描所有 migration 文件确认仅此一处 bug
- **v0.6.4（已完成 2026-07-21）**：**Critical Bug 修复** —— 33 个 migration 全部应用后，`db.AutoMigrate(&model.SysConfig{})` 触发 `ALTER TABLE MODIFY COLUMN created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP` 失败，MySQL 8.0 报 `Error 1067 (Invalid default value for 'created_at')`。根因：GORM 默认把 `time.Time` 推断为 `datetime(3)`（带毫秒），但 migration 001 用 `DATETIME`（无毫秒）建表。修复：`SysConfig` struct 的 `CreatedAt`/`UpdatedAt` GORM tag 显式声明 `type:datetime`，让 GORM 不再修改列定义
- **v0.6.5（已完成 2026-07-21）**：**前端登录跳转 + 后台进不去修复** —— 管理员登录后没自动跳后台 + 后台进不去。根因：`auth.role` 为空时（localStorage 持久化数据损坏 / 旧版本字段缺失 / 手动篡改），`homePath` 计算为 `//dashboard` → 404；路由守卫回退同样问题；登录页 callback 风格 `await` 是 no-op。修复：① `stores/auth.ts` homePath 兜底；② `router/index.ts` 守卫检测 stale state 强制 logout；③ `login/index.vue` 改 Promise 风格；④ 优先用后端返回的 role；⑤ redirect 白名单校验
- **v0.6.6（已完成 2026-07-21）**：**充值/提现/支付回调并发安全加固** —— 修复 v0.6.0/v0.6.1 遗留的并发竞态隐患。**DB 级幂等状态转换**：所有 `pending->settled/rejected/paid` 转换用 `Where(status=pending)+RowsAffected` 检查，第二个并发请求直接失败；**GORM `clause.Locking{Strength:"UPDATE"}` 行锁**：10 处 `tx.Set("gorm:query_option", "FOR UPDATE")` 替换为类型安全的 clause API；**frozen_balance 守门**：管理员打款/驳回时不足即事务失败，禁止强制设为 0；**支付回调 DB 级幂等**：Redis SETNX 仅辅助，DB 状态守门为最终保障（防 Redis 故障/失效窗口）；**AppliedAmount 审计字段**：migration 034 新增 `applied_amount` 列保留原始申请金额；**11 个并发/回滚测试全 PASS**（含 -race 竞态检测，覆盖 TenantApproveRecharge / TenantRejectWithdraw / TenantPayWithdraw / ProcessPaidOrder / AdminPayTenantWithdraw 全链路）；全量 27 个测试包 PASS + admin build PASS
- **v0.6.7（已完成 2026-07-21）**：**P0 紧急 bug 修复 —— 管理员登录后死机**。根因：`apps/admin/src/stores/auth.ts` 的 `scheduleRefresh` 在 `delay<=0` 时不 await 调 `doRefresh`，`doRefresh` 成功后又调 `scheduleRefresh`，若新 `expires_at` 仍让 `delay<=0`（如 `jwt.access_ttl_seconds` 被配为 ≤300 秒、后端返回异常值、客户端时钟超前），形成无限异步递归，每秒数十个 HTTP 请求导致浏览器死机。修复：① `stores/auth.ts` 新增 `_refreshing` 并发锁杜绝递归；② `doRefresh` 校验新 `expires_at > now+60s`，异常值跳过；③ `scheduleRefresh` 最小延迟兜底 30s；④ `http.ts` 401 拦截器加固空 token 登出；⑤ 后端 `AdminUpdateConfig` 新增 `jwt.access_ttl_seconds≥600`/`jwt.refresh_ttl_seconds≥3600` 最小值校验，源头防止配错；⑥ `logout` 重置 `_refreshing`，避免 stale 状态；admin build PASS（vue-tsc 类型检查通过）
- **v0.6.8（已完成 2026-07-21）**：**P0 紧急 bug 修复 —— v0.6.7 后管理员登录依然进不去 /admin/dashboard 卡死**。根因：v0.6.7 修复 scheduleRefresh 死循环后，定位到第二个根因——`apps/admin/src/components/UpdateNotifier.vue` 的 `scheduleNext` 使用 `setInterval` 调度 async 轮询：setInterval 不等待 async 回调完成，pollOnce 耗时超过 interval 时多个回调并发执行；interval 变化时 stopPolling+scheduleNext 创建新 setInterval 但排队中的旧回调仍执行并创建更多 setInterval → 引用覆盖泄漏（timer.value 被覆盖，旧 setInterval 永远无法 clearInterval）；组件卸载后排队回调继续执行 → 内存泄漏 + 卡死。修复：① `scheduleNext` 改用 `setTimeout` 自递归（每次 pollOnce 完成后才调度下一次，从根上消除并发）；② timer 类型 setInterval→setTimeout，stopPolling 用 clearTimeout；③ 新增 `isUnmounted` 标志位，onBeforeUnmount 先标记再 stopPolling，回调开头与 await 之后双重检查；④ `startPolling` 首次 await 期间防御性检查 isUnmounted；admin build PASS（16.85s）
- **后续**：analysis.go 等 ~40 处参数校验类错误泄露清理、openapi.go/enduser.go c.JSON 直接泄露 ~20 处清理（非阻断性，按需迭代）+ MySQL 8.0 真实环境集成测试验证

## 开发约束（铁律）

本项目严格遵守 `web-project-flow` skill（已全局安装）的三份铁律：

1. **禁硬编码**：密钥 / token / 域名 / 价格 / 接口地址 全部抽离到环境变量或 sys_config
2. **配置后台化**：所有可调参数走 `sys_config` 表 + Redis 缓存 + 后台可视化编辑
3. **防幻觉**：不确定处标注「待核实」，代码不确定处标注「需验证」

可通过 `/bhelp` 查看 skill 全部 11 份提示词索引；写业务代码前用 `/bhardcode /bconfig /bhaluc` 一次性加载三铁律；变更/加功能后用 `/bdocs` 触发四份核心文档联动更新。

## 许可证

Proprietary —— 未经授权禁止商用
