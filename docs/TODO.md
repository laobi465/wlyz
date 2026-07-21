# 待完成文档 (TODO / Backlog)

格式：`优先级 + 状态 + 条目 + 预计版本 + 备注`

- 优先级：`P0`（紧急）/ `P1`（高）/ `P2`（中）/ `P3`（低）
- 状态：`[待开始]` `[进行中]` `[已阻塞]` `[已延期]` `[无限延期]` `[已完成]`

---

## v0.6.4 Critical Bug 修复 ✅ 已完成 2026-07-21

### [P0] GORM AutoMigrate 触发 Error 1067 (Invalid default value for 'created_at') ✅ 已完成 v0.6.4
- [x] [已完成 2026-07-21] **根因**：33 个 migration 全部应用后，`db.AutoMigrate(&model.SysConfig{})` 触发 `ALTER TABLE MODIFY COLUMN created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP`，MySQL 8.0 + sql_mode=STRICT_TRANS_TABLES 下报 `Error 1067 (42000): Invalid default value for 'created_at'`
- [x] [已完成 2026-07-21] **根因分析**：GORM 默认把 `time.Time` 推断为 `datetime(3)`（带毫秒精度），但 migration 001 用 `DATETIME`（无毫秒）建表，AutoMigrate 检测到列定义不匹配试图 ALTER MODIFY，MySQL 8.0 在 MODIFY 时对 `CURRENT_TIMESTAMP` 默认值与 `datetime(3)` 类型校验更严格导致报错
- [x] [已完成 2026-07-21] **修复**：`SysConfig` struct 的 `CreatedAt`/`UpdatedAt` GORM tag 显式声明 `type:datetime`，让 GORM 不再修改列定义，保持 migration 001 的 schema 不变

---

## v0.6.3 Critical Bug 修复 ✅ 已完成 2026-07-21

### [P0] migration 030 报 Error 1136 (Column count doesn't match value count) ✅ 已完成 v0.6.3
- [x] [已完成 2026-07-21] **根因**：`030_v0.5.0_notify_webhook.up.sql` 的 INSERT 声明 6 列但每行 VALUES 写了 7 个值（在 `config_value` 与 `config_type` 之间多了一个空字符串 `''`），MySQL 报 `Error 1136 (21S01): Column count doesn't match value count at row 1`
- [x] [已完成 2026-07-21] **修复**：全部 10 行 VALUES 元组移除多余空字符串字段，列顺序对齐 sys_config 表 schema（config_key, config_value, config_type, config_name, config_group, remark）
- [x] [已完成 2026-07-21] **全量扫描验证**：Python 脚本扫描所有 `migrations/*.up.sql` 的 `INSERT INTO sys_config` 列数一致性，031/032/033 全部匹配，仅 030 一处 bug

---

## v0.6.2 Critical Bug 修复 ✅ 已完成 2026-07-21

### [P0] Docker Compose 一键部署在 MySQL 8.0 上失败 ✅ 已完成 v0.6.2
- [x] [已完成 2026-07-21] **根因**：migration 015 使用 MariaDB-only 语法 `ADD COLUMN/INDEX IF NOT EXISTS`，MySQL 8.0 不支持，导致 `schema_migrations.dirty=1, version=15`
- [x] [已完成 2026-07-21] **修复 1**：重写 `015_v0.4.0_end_user_system.up.sql` 全部改用 `INFORMATION_SCHEMA + PREPARE/EXECUTE` 兼容方案 + `INSERT ON DUPLICATE KEY UPDATE` 幂等
- [x] [已完成 2026-07-21] **修复 2**：`migrator.go` 新增 `MIGRATION_REPAIR_DIRTY=true` 显式 dirty 恢复流程 + MySQL advisory lock（`GET_LOCK`/`RELEASE_LOCK`）并发保护 + 详细错误消息
- [x] [已完成 2026-07-21] **修复 3**：`one_click_deploy.sh` 新增 `--reset-data` 显式确认 + 移除破坏性 `DELETE FROM schema_migrations` + 自动备份 + 自动走幂等修复流程 + MySQL 健康检查 + 失败诊断
- [x] [已完成 2026-07-21] **修复 4**：`clean_dirty_migration.sh` 完全重写为 `--show` / `--dry-run` / `--repair` / `--force-delete` 四模式
- [x] [已完成 2026-07-21] **修复 5**：`mysql:8.0` → `mysql:8.0.36` 固定小版本，避免版本漂移
- [x] [已完成 2026-07-21] **修复 6**：新增 13 个迁移器测试用例（6 个单元测试全 PASS + 7 个集成测试待真实 MySQL 8.0 环境）+ `verify_migration_015.sh` 静态验证脚本（10/10 通过）

### v0.6.2 待办（需真实环境验证）
- [ ] [待开始] MySQL 8.0 真实环境集成测试（13 个 `TestIntegration_*` 用例）
- [ ] [待开始] Docker Compose 真实部署端到端测试
- [ ] [待开始] `MIGRATION_REPAIR_DIRTY=true` 端到端流程在真实 dirty 数据库上验证

---

## 安全审计（4 类优先级全覆盖 ✅ v0.6.0 + v0.6.1）

### [P0] 高危 13 个 ✅ 已完成 v0.6.0
- [x] [已完成 2026-07-20] 部署链路 5 个 bug（端口冲突 / migration dirty / SQL 语法 / nginx / API 契约）
- [x] [已完成 2026-07-20] P0 高危安全 bug 13 个（认证绕过 / SQL 注入 / 权限提升 / 敏感信息泄露 / 并发竞态）

### [P1] 普通 21 个 ✅ 已完成 v0.6.1
- [x] [已完成 2026-07-20] Migration + 加密 + 工具 7 个（migration 032/015 兼容性 + crypto 模偏差 + HMAC 名实一致 + update 锁值校验 + TOTP skew）
- [x] [已完成 2026-07-20] 认证中间件安全 5 个（JWT Subject + Nonce 顺序 + IP 黑名单 fail-open + CF IP 伪造 + public 限流）
- [x] [已完成 2026-07-20] 业务 handler 5 个（提现流水错配 + 充值 FOR UPDATE + 月费重复 + 对账 tenant_id + 版本号比较）
- [x] [已完成 2026-07-20] 前端 4 个（v-html XSS + Cookie Secure + 浮点精度 + H5 401 队列）

### [P2] 联调 15 个 ✅ 已完成 v0.6.0
- [x] [已完成 2026-07-20] 前后端联调 15 个 bug（字段映射 / 枚举对齐 / 分页参数 / 云变量字段 / 收入趋势 / Top 应用 / 邀请码 / 设备 location / 支付配置 / 版本 channel / 公告 type/status / 佣金 type/status）

### [P3] 优化 34 个 ✅ 已完成 v0.6.1
- [x] [已完成 2026-07-20] 错误信息泄露 30 处（err.Error() 改 logger.Error + 通用消息）
- [x] [已完成 2026-07-20] N+1 查询 4 处（admin/tenant 列表批量聚合）
- [x] [已完成 2026-07-20] HTTP 客户端超时 4 处（notify 包 webhook + SMS 10s 超时）

### 后续可选清理（非阻断性，按需迭代）
- [ ] [待开始] analysis.go 等 ~40 处参数校验类错误泄露（`参数错误: "+err.Error()`）
- [ ] [待开始] openapi.go / enduser.go 用 `c.JSON` 直接泄露 err.Error() 共 ~20 处
- [ ] [待开始] risk.go 还有 8 处 5002 类错误泄露

---

## P0 紧急（一期 MVP 必须）

### [P0] 一期 MVP 核心闭环

#### 项目骨架搭建 ✅ 已完成
- [x] [已完成] 初始化 Go 项目结构（cmd/internal/pkg/migrations） - v0.2.0
- [x] [已完成] 初始化 Vue3 admin 项目（超管/开发者/代理三布局） - v0.2.0
- [x] [已完成 2026-07-19] 初始化 Vue3 H5 项目（终端用户） - v0.2.4（apps/admin/src/layouts/H5Layout.vue + views/h5/{Home,Query,PayResult,CardDetail}.vue + 路由 /h5，H5 作为 admin 子应用而非独立工程）
- [x] [已完成] 编写 docker-compose.yml（mysql/redis/api/admin/nginx） - v0.2.0
- [x] [已完成] 编写 Dockerfile（多阶段构建） - v0.2.0
- [x] [已完成] 编写宝塔面板安装脚本 baota_deploy.sh - v0.2.0
- [x] [已完成] 编写 .env.example 和配置加载逻辑 - v0.2.0
- [x] [已完成] 生成 RSA-4096 密钥对工具脚本（scripts/gen_rsa_key.sh，支持 --force / 自定义输出目录 / 密钥配对校验） - v0.3.5

#### 数据库初始化 ✅ 已完成
- [x] [已完成] 编写全部 26 张表的 migration 文件 - v0.2.0
- [x] [已完成] 编写 seed 数据（超管账号、默认套餐、默认 sys_config 47 项） - v0.2.0
- [x] [已完成] 实现 golang-migrate 自动迁移机制（轻量级 SQL 文件迁移 + schema_migrations 表 + dirty 状态 + 事务保护） - v0.3.5

#### 认证与多租户 ⏳ 下一步重点
- [x] [已完成] 平台超管登录 + JWT - v0.2.0
- [x] [已完成] 开发者注册/登录/2FA - v0.2.0
- [x] [已完成] 代理登录 + JWT - v0.2.0
- [x] [已完成] 多租户隔离中间件骨架（自动注入 tenant_id） - v0.2.0
- [x] [已完成] 套餐配额检查中间件（internal/quota 包：CheckMaxApps/MaxCards/MaxAgents/MaxDevices + ExceededError + 4 个 handler 接入） - v0.3.5
- [x] [已完成] 密码 bcrypt (cost=12) 工具函数 - v0.2.0
- [x] [已完成] JWT Token 刷新机制 - v0.2.0

#### 应用管理
- [x] [已完成] 应用 CRUD API - v0.2.2
- [x] [已完成] AppKey/AppSecret/SignSecret 生成 - v0.2.2
- [x] [已完成] 密钥轮换（保留旧密钥 7 天） - v0.2.2
- [x] [已完成] 应用配置（一机一卡/心跳/宽限/解绑扣时） - v0.2.2
- [x] [已完成 2026-07-19] 应用列表/详情前端页面 - v0.2.4（apps/admin/src/views/tenant/Apps.vue 387 行：列表 + 新建/编辑/详情/重置密钥/删除对话框）

#### 卡密体系
- [x] [已完成] 卡类套餐 CRUD - v0.2.2
- [x] [已完成] 卡密批量生成算法（SecureRandom + SHA-512 校验位） - v0.2.0
- [x] [已完成] 卡密查询/封禁/解封/删除 - v0.2.2
- [x] [已完成 2026-07-20] 卡密导入导出 CSV - v0.3.6（card.go TenantExportCardsCSV/TenantImportCardsCSV + 前端 Cards.vue 导出/导入对话框）
- [x] [已完成] 卡密状态机（unused/active/expired/banned/disabled） - v0.2.2
- [x] [已完成 2026-07-19] 卡密批量生成前端页面（含弹窗） - v0.2.4（apps/admin/src/views/tenant/Cards.vue 567 行：批量生成对话框 + 表单校验 + 生成结果展示 + 复制全部 + CSV 导入导出 v0.3.6）

#### 设备绑定
- [x] [已完成] 设备指纹生成算法（CPU+主板+MAC+磁盘哈希） - v0.2.2
- [x] [已完成] 一机一卡密绑定逻辑 - v0.2.2
- [x] [已完成] 设备解绑扣时逻辑 - v0.2.2
- [x] [已完成 2026-07-20] 设备强制下线 - v0.3.6（card.go TenantBanCard 联动 heartbeat.Remove 清 Redis 心跳 + DB 标记 banned；tenant_business.go TenantKickDevice 强制下线单设备；client.go unbind 流程）
- [x] [已完成 2026-07-19] 设备列表/封禁前端页面 - v0.2.7（apps/admin/src/views/tenant/Devices.vue 180 行：列表 + 应用/关键字/在线状态筛选 + 强制下线按钮 + 移动端卡片视图；后端 TenantBanCard 封禁卡密时级联将设备 status=banned）

#### 客户端验证 API
- [x] [已完成] HMAC-SHA256 签名中间件骨架 - v0.2.0
- [x] [已完成] Nonce 防重放（Redis） - v0.2.0
- [x] [已完成] Timestamp ±5 分钟校验 - v0.2.0
- [x] [已完成] /api/v1/client/login 接口实现 - v0.2.2
- [x] [已完成] /api/v1/client/verify 接口实现 - v0.2.2
- [x] [已完成] /api/v1/client/heartbeat 接口实现 - v0.2.2
- [x] [已完成] /api/v1/client/bind 接口实现 - v0.2.2
- [x] [已完成] /api/v1/client/unbind 接口实现 - v0.2.2
- [x] [已完成] /api/v1/client/logout 接口实现 - v0.2.2
- [x] [已完成] RSA-4096 响应签名工具 - v0.2.0
- [x] [已完成] 心跳保活 Redis Sorted Set - v0.2.2
- [x] [已完成] 离线宽限期判定 - v0.2.2

#### 平台总支付
- [x] [已完成] 超管后台支付配置页（S-06） - v0.2.3（通过 sys_config CRUD + AdminTestPayConfig 实现）
- [x] [已完成] 彩虹易支付下单接口 - v0.2.3
- [x] [已完成] 彩虹易支付异步回调接口 - v0.2.3
- [x] [已完成] 彩虹易支付同步跳转接口 - v0.2.3
- [x] [已完成] 支付成功自动发卡 - v0.2.3
- [x] [已完成] 支付回调防重入（Redis 锁） - v0.2.3
- [x] [已完成] 30 分钟订单超时关闭 - v0.2.3（查询时触发）
- [x] [已完成] 平台抽成计算与结算记录 - v0.2.3（platform_settlement 表 + AdminSettleOrder）

#### 管理后台基础页面
- [x] [已完成] 超管后台：登录 + 系统配置 + 结算管理（响应式） - v0.2.4（其余页面占位，v0.3.0+ 逐步补齐）
- [x] [已完成] 开发者控制台：登录 + 应用管理 + 卡类管理 + 卡密管理（响应式） - v0.2.4（其余页面占位，v0.3.0+ 逐步补齐）
- [x] [已完成] 代理控制台：登录 + 注册 - v0.2.4（核心页面 v0.2.5 补齐）
- [x] [已完成] 通用响应式布局 BasicLayout（桌面侧边栏 + 移动端抽屉）+ ResponsiveTable（桌面表格 + 移动卡片）+ PageHeader - v0.2.4
- [x] [已完成] 三角色登录（含 2FA TOTP 二阶段）+ JWT 双 token 自动续期 - v0.2.4
- [x] [已完成] 开发者注册页 - v0.2.4
- [x] [已完成] 官网首页（Hero + 功能 + 场景 + 套餐 + FAQ + CTA） - v0.2.4

#### 终端用户 H5
- [x] [已完成] H5 布局 + 购卡首页 + 支付结果 + 卡密查询 + 卡密详情 - v0.2.4
- [x] [已完成 2026-07-20] 终端用户体系后端（H5 注册/登录/绑卡/单点踢出） - v0.4.0（migration 015 end_user + end_user_card + end_user_token 表 + ALTER app_card.end_user_id + 10 项 enduser.* 配置；Manager.Register/Login/RefreshToken/Logout/RevokeSession/RevokeAllSessions/ListSessions/BindCard/UnbindCard/ListMyCards/GetCardDetail/GetProfile/UpdateProfile/ChangePassword/ResetPassword；H5EndUserAuth 中间件 HMAC-SHA256 签名校验 + 常量时间比较防时序攻击；bcrypt cost=12 密码哈希；refresh token SHA-512 哈希存储 + jti 单点踢出；19 个端点：5 公开 + 10 H5 + 4 admin）
- [x] [已完成 2026-07-20] H5 用户登录/注册前端页面（接入新后端 API） - v0.4.x（apps/admin/src/views/h5/{Login,Register,ResetPassword}.vue + api/enduser.ts + stores/enduser.ts；登录调 /public/enduser/login + 注册调 /public/enduser/register + 验证码 60s 倒计时 + 找回密码）
- [x] [已完成 2026-07-20] H5 个人中心前端页面（卡密绑定/会话管理/密码修改） - v0.4.x（apps/admin/src/views/h5/{Profile,MyCards,Sessions,EditProfile,ChangePassword}.vue；H5Layout 新增「我的」tab + 路由守卫 enduser 角色判定 + guestOnly 已登录跳转；http.ts 按路径前缀分流 H5/三角色 token + 401 续期链路）

#### 安全防护（基础）
- [x] [已完成] Nginx 限流配置（gateway.conf） - v0.2.0
- [x] [已完成] IP 黑名单手动管理（表结构 + 中间件） - v0.2.0
- [x] [已完成] 卡密错误 5 次封 IP 1h（ratelimit.go） - v0.2.0
- [x] [已完成] 安全响应头配置 - v0.2.0
- [x] [已完成] HTTPS 强制跳转 - v0.2.0

#### 部署与运维 ✅ 已完成
- [x] [已完成] /health 健康检查接口 - v0.2.0
- [x] [已完成] Docker healthcheck - v0.2.0
- [x] [已完成] 部署文档（Docker Compose + 宝塔两种方式） - v0.2.0
- [x] [已完成 2026-07-20] 安装向导页面（/install） - v0.3.6（前端 Install.vue 4 步向导：环境检测 → 超管账号 → 平台配置 → 完成；后端 install.go InstallStatus/Install；路由 /install public）
- [x] [已完成 2026-07-20] SSH 一行命令一键自动化部署脚本 - v0.6.0（scripts/one_click_deploy.sh：10 步流程 = OS 检测 → 基础工具 → 宝塔官方脚本自动安装 → Docker 自动安装 → git clone → RSA 密钥生成 → .env 强随机密钥自动填充 → docker compose up -d --build → 初始化超管账号 admin/admin123（检测占位 hash → htpasswd 生成 bcrypt(cost=12) → SQL UPDATE + 验证；非首次部署自动跳过） → 生成 /root/keyauth_deploy_info.txt 部署信息文件；远程一行命令执行 `sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh)"`；支持本地项目内执行 + 远程执行 + 已装宝塔复用三种模式；txt 文件含 9 大章节：宝塔入口/密钥/访问地址/管理员账号/密钥配置/服务状态/反代+SSL 完整教程/运维命令/备份建议 + chmod 600 权限；bash -n 语法检查通过）

---

## P1 高（二期增值）

### [P1] 二期增值功能

#### 开发者自有易支付
- [x] [已完成 2026-07-20] 套餐 allow_custom_pay 字段生效 - v0.3.6（CreatePayOrder 内查 SysPackage.AllowCustomPay）
- [x] [已完成] 开发者审核充值（D-19） - v0.3.0（v0.3.2 充值审核闭环）
- [x] [已完成 2026-07-20] 开发者支付配置页（D-18）双层支付模式 - v0.3.6（CreatePayOrder 双层切换 + pay_mode 响应字段）
- [x] [已完成] tenant_pay_config 表读写（CRUD 接口已实现） - v0.3.0
- [x] [已完成 2026-07-20] 双层支付模式切换逻辑 - v0.3.6（TOP/ORD 前缀分发）
- [x] [已完成 2026-07-20] 开发者自有支付下单/回调接口 - v0.3.6（EpayTenantNotify 完整实现 + processTenantOwnPaidOrder 事务 + loadTenantPayConfig 解密）
- [x] [已完成 2026-07-20] 开发自有支付附加月费订单 - v0.4.x（migration 027 tenant_monthly_fee_order 表 + 3 项 pay.tenant_monthly_fee.* sys_config；TenantMonthlyFeeOrder model；admin_finance.go 3 个端点：orders 列表/stats 统计/mark_paid 手动标记；tenant_finance.go 2 个端点：current 当前周期/pay 支付；pay.go dispatchPaidOrder 支持 MFD 前缀 + processMonthlyFeePaid 事务处理；10 个测试覆盖）
- [x] [已完成 2026-07-20] 切换支付方式时通知所有代理（站内信+横幅+弹窗） - v0.4.x（notify.go 新增 TemplatePayModeChanged 常量 + CfgKeyPayModeChangedEnabled 配置开关 + NotifyAgentsByTenant 辅助函数：查启用代理 → 创建 Notice + NoticeTarget(all_agents) + 批量 notify_log；tenant_business.go TenantSavePayConfig 检测 enabled 状态变更触发通知；migration 022 新增 pay_mode_changed 模板 + 配置开关；7 个测试覆盖开关/无代理/完整链路/跨租户隔离/模板兜底）

#### 代理注册付费流程
- [x] [已完成] 开发者生成邀请码（D-08） - v0.3.0
- [x] [已完成] 邀请码 CRUD + 状态机（active/disabled/exhausted/expired） - v0.3.0
- [x] [已完成] 代理注册页（REG-01，3 步流程）前端骨架 - v0.2.4
- [x] [已完成 2026-07-20] 代理注册费支付（走平台总支付，AgentRegister 创建 REG 订单 + 返回 pay_url） - v0.3.6
- [x] [已完成 2026-07-20] 代理账号自动创建 + 关联开发者（processAgentRegisterPaid 事务内建 Agent + 回填 AgentID + 邀请码 used_count++） - v0.3.6
- [x] [已完成 2026-07-20] 超管后台代理注册管理（S-17，含退款/收入统计） - v0.4.x（migration 026 agent_registration_order 加 5 退款字段；admin_business.go 4 个端点：list 列表/stats 收入统计/refund 退款/detail 详情；refund 事务：更新 refund_status + 退还代理注册费到 agent.balance + 记录 agent_balance_log + 禁用代理账号；4 个测试覆盖成功/未支付/已退款/无代理）

#### 代理购买卡密
- [x] [已完成] 代理充值申请（P-09） - v0.3.0（v0.3.1 AgentRecharge + v0.3.2 审核闭环）
- [x] [已完成] 开发者审核充值（D-19） - v0.3.0（v0.3.2 充值审核闭环）
- [x] [已完成] 代理余额扣款生成卡密 - v0.3.0（AgentGenerateCards 事务化：扣余额→生成卡密→写 deduct 流水→加佣金→写 commission 流水）
- [x] [已完成 2026-07-20] 代理实时扫码购卡（P-10，备用） - v0.4.x（agent_business.go AgentPortalQrCode 端点返回代理专属购卡 URL；apps/admin/src/views/agent/QrCode.vue 渲染二维码 + 复制链接 + 下载按钮；4 个测试覆盖 NoSubdomain/WithApprovedSubdomain/PendingSubdomain/AgentNotFound）
- [x] [已完成] 代理佣金计算（percentage / diff 两种模式） - v0.3.0（AgentGenerateCards 内联实现）
- [x] [已完成] 代理提现申请（P-05） - v0.3.0（v0.3.0 AgentWithdraw）
- [x] [已完成] 开发者审核提现 + 打款（D-14） - v0.3.0（v0.3.2 提现审核闭环：pay + reject）
- [x] [已完成 2026-07-20] 代理独立门户（P-06，仅展示，收款走开发者） - v0.4.x（apps/admin/src/views/h5/AgentPortal.vue 门户页 + AgentPortalBuy.vue 结算页；api/portal.ts；后端 public.go PublicPortal + PublicPortalOrder 端点（body-rewriting 委托 CreatePayOrder 注入 agent_id）；DTO 隐藏 agent_base_price 防成本泄露；4 个测试覆盖 Success/AgentDisabled/NotFound/TenantDisabled）
- [x] [已完成 2026-07-20] 代理子域名绑定 - v0.4.x（migration 024 agent 加 subdomain_status 字段；agent_business.go 3 端点：status/apply/unbind；admin_business.go 3 端点：list/approve/reject；2 项 sys_config agent.subdomain.enabled/pattern；apps/admin/src/api/agent.ts 扩展；23 个测试覆盖全流程）

#### 代理核心页面（响应式 H5） ✅ v0.2.5 已完成
- [x] [已完成] 代理 API 模块 `api/agent.ts`（dashboard/me/card_types/cards/generate/orders/commission/withdraw + 9 个类型） - v0.2.5
- [x] [已完成] 代理概览 `views/agent/Dashboard.vue`（4 数据卡 + 4 快捷入口 + 最近订单） - v0.2.5
- [x] [已完成] 代理购卡 `views/agent/Cards.vue`（余额栏 + 卡类网格 + 购卡对话框 + 结果展示） - v0.2.5
- [x] [已完成] 代理订单 `views/agent/Orders.vue`（状态筛选 + 列表 + 分页） - v0.2.5
- [x] [已完成] 代理佣金 `views/agent/Commission.vue`（4 统计卡 + 类型/状态双筛选 + 提现对话框） - v0.2.5
- [x] [已完成] 代理余额/提现 `views/agent/Balance.vue`（钱包概览 + 充值申请 + 充值/提现记录） - v0.2.5
- [x] [已完成] AgentLayout 顶部余额标签改为调用 `/agent/auth/me` 真实获取 + 路由切换自动刷新 - v0.2.5
- [x] [已完成] 后端 agent 路由 501 占位已全部升级为真实实现 - v0.3.0（v0.3.1 字段补齐）

#### 三角色 Profile + 双 Dashboard（响应式 H5） ✅ v0.2.6 已完成
- [x] [已完成] 三角色共享账号设置 API 模块 `api/profile.ts`（currentUser/updateProfile/changePassword/setup2FA/verify2FA/disable2FA/listLoginDevices/kickDevice + 5 个类型） - v0.2.6
- [x] [已完成] 超管后台 API 模块 `api/admin.ts`（dashboard/tenants/packages/agents/notices/logs/security + 8 个类型） - v0.2.6
- [x] [已完成] 开发者控制台 API 模块 `api/tenant.ts`（dashboard/devices/orders/cloud-vars/versions/agents/invite-codes/pay-config/notices + 9 个类型） - v0.2.6
- [x] [已完成] 超管账号设置 `views/admin/Profile.vue`（基础资料 + 修改密码 + 2FA TOTP + 登录设备） - v0.2.6
- [x] [已完成] 开发者账号设置 `views/tenant/Profile.vue`（基础资料 + 公司信息 + 修改密码 + 2FA） - v0.2.6
- [x] [已完成] 代理账号设置 `views/agent/Profile.vue`（账户概览 + 基础资料 + 提现账户 + 修改密码） - v0.2.6
- [x] [已完成] 超管平台概览 `views/admin/Dashboard.vue`（8 数据卡 + 待办列表 + 收入趋势 + 最近开发者/订单表） - v0.2.6
- [x] [已完成] 开发者工作台 `views/tenant/Dashboard.vue`（8 数据卡 + 8 快捷入口 + 收入趋势 + 应用 TOP5 + 最近订单） - v0.2.6
- [x] [已完成] 路由 5 个 PlaceholderView 替换为真实页面（admin/Dashboard + admin/Profile + tenant/Dashboard + tenant/Profile + agent/Profile） - v0.2.6
- [x] [已完成] 后端 `/admin/dashboard` `/tenant/dashboard` 及 Profile 相关接口 501 占位已升级为真实实现 - v0.3.0（v0.3.1 字段补齐）
- [x] [已完成] 后端 `/{role}/auth/me` ProfileMe/AgentMe 返回完整字段（email/phone/real_name/totp_enabled/last_login_at/last_login_ip） - v0.3.0（v0.3.1 字段补齐）

#### 全部剩余 PlaceholderView 替换为真实页面（响应式 H5 完整覆盖） ✅ v0.2.7 已完成
- [x] [已完成] Admin 7 页：Tenants/Packages/Agents/Notices/PayConfig/Logs/Security - v0.2.7
- [x] [已完成] Tenant 8 页：Devices/Orders/CloudVars/Versions/Agents/InviteCodes/PayConfig/Notices - v0.2.7
- [x] [已完成] Agent 1 页：Notices（含 agent.ts 扩展 listAgentNoticesApi/readAgentNoticeApi） - v0.2.7
- [x] [已完成] 路由 16 个 PlaceholderView 全部替换为懒加载真实组件 + 移除 PlaceholderView 导入 - v0.2.7
- [里程碑] PlaceholderView 占位阶段彻底结束，前端三角色所有路由全部由真实响应式 H5 页面承载 - v0.2.7
- [x] [已完成] 后端 admin/tenant/agent 业务接口（dashboard/profile/CRUD 等）从 501 升级为真实实现 - v0.3.0

#### 后端业务 API 全量实现（替换全部 501 占位） ✅ v0.3.0 已完成
- [x] [已完成] `internal/handler/admin_business.go` 18 个超管接口（公开平台公告 + 工作台 + 租户/套餐/代理/公告 CRUD + 日志审计 + 安全中心） - v0.3.0
- [x] [已完成] `internal/handler/tenant_business.go` 22 个开发者接口（工作台 + 设备/订单/云变量/版本/代理/邀请码/支付配置/公告 全套 CRUD） - v0.3.0
- [x] [已完成] `internal/handler/agent_business.go` 11 个代理接口（工作台 + AgentMe + 卡类/卡密/订单/佣金/提现/通知） - v0.3.0
- [x] [已完成] `internal/handler/profile.go` 三角色统一账号设置（ProfileMe + UpdateProfile + ChangePassword + 2FA 全流程 + LoginDevices） - v0.3.0
- [x] [已完成] `internal/router/router.go` 注册 40+ 新端点 + 三角色 `/auth/me` 切换为 `ProfileMe` - v0.3.0
- [x] [已完成] admin.go 清理：移除 12 个 501 占位函数，保留 AdminListConfig/AdminUpdateConfig 真实实现 - v0.3.0
- [x] [已完成] AgentGenerateCards 事务化（扣余额 → 生成卡密 → 写扣费日志 → 结算佣金 + 写佣金日志） - v0.3.0
- [x] [已完成] 2FA 全流程（setup Redis 中转 10min → verify AES 加密入库 + 备用码 Redis 持久化 → disable 黑名单 refresh token） - v0.3.0
- [x] [已完成] 三级公告体系后端实现（平台/开发者/代理 notice 表统一读写 + notice_read 已读记录） - v0.3.0
- [x] [已完成] 邀请码生成（crypto/rand 16 位 + 5 次重试唯一性 + 状态机 active/disabled/exhausted/expired） - v0.3.0
- [x] [已完成] 云变量 CRUD + 版本管理 CRUD - v0.3.0
- [x] [已完成] `go build ./...` + `go vet ./...` 全部通过（0 错误 0 警告） - v0.3.0
- [x] [已完成] `sys_tenant/sys_package/notice/log_operation/sec_ip_blacklist/AppCloudVar/AppVersion/Agent/AgentInviteCode` 字段全部补齐（migration 006） - v0.3.1
- [x] [已完成] `AgentRecharge` 完整实现（pending 申请 + sys_config 限额校验） - v0.3.1
- [x] [已完成] agent 表 `totp_secret` 字段已加，代理 2FA setup/verify/disable 全部可用 - v0.3.1
- [x] [已完成] `ListLoginDevices` 完整实现（refresh_token_device 表 + recordLoginSession + KickDevice） - v0.3.1

#### v0.3.1 字段补全与待核实项归零 ✅ v0.3.1 已完成
- [x] [已完成] migration 006 ALTER TABLE 补齐所有缺失字段 - v0.3.1
- [x] [已完成] `log_login_failed` 表 + 异步 channel worker（容量 1024）+ `securityFailedLoginToday`/`securityBlockedIPsToday` 助手 - v0.3.1
- [x] [已完成] `refresh_token_device` 表 + `recordLoginSession`/`markAllSessionsRevoked` - v0.3.1
- [x] [已完成] admin_business.go 全部字段使用真实 model（Remark/Description/CommissionMode/InviterUsername/Sort/CreatedBy） - v0.3.1
- [x] [已完成] tenant_business.go 全部字段使用真实 model（ReadOnly/Channel/used_by_username/Sort/级联删除） - v0.3.1
- [x] [已完成] agent_business.go AgentMe 真实返回 email/totp_enabled/last_login_ip/inviter_username；Dashboard today_spent 真实计算 - v0.3.1
- [x] [已完成] profile.go 启用 agent email 更新；移除三处 agent 2FA 501 阻断 - v0.3.1
- [x] [已完成] 前端 4 个 .ts API + 5 个 .vue 文件清理过时「待核实 v0.3.0」标记 - v0.3.1
- [x] [已完成] `api/tenant.ts` 补齐 `updateTenantNoticeApi` + `deleteTenantNoticeApi`，Notices.vue 启用删除按钮 - v0.3.1
- [x] [已完成] `go build` + `go vet` + `pnpm run build`（admin）三重编译验证通过 - v0.3.1

#### v0.3.2 代理充值/提现审核闭环 ✅ v0.3.2 已完成
- [x] [已完成] `tenant_finance.go` 6 个 handler（List/Approve/Reject × Recharge + List/Pay/Reject × Withdraw） - v0.3.2
- [x] [已完成] 路由注册 6 条新路由（`/tenant/recharge_requests` + `/tenant/withdrawals`） - v0.3.2
- [x] [已完成] 前端 `RechargeReview.vue` + `WithdrawalReview.vue` 两个审核页面（响应式 H5） - v0.3.2
- [x] [已完成] 路由注册 `/tenant/recharge-review` + `/tenant/withdrawal-review` - v0.3.2
- [x] [已完成] `api/tenant.ts` 补齐 `TenantRechargeRequest` / `TenantWithdrawal` 类型 + 6 个审核 API - v0.3.2
- [x] [已完成] 修复 `agent/Balance.vue` 误用 withdrawApi 提交充值 → 改用 `agentRechargeApi` + pay_method/pay_voucher 字段 - v0.3.2
- [x] [已完成] 充值审核通过支持调整实际到账金额（actual_amount） - v0.3.2
- [x] [已完成] 提现驳回事务：退回余额 + withdraw.status=rejected + balance_log.status=rejected - v0.3.2
- [x] [已完成] 提现打款事务：withdraw.status=paid + paid_at + pay_trade_no + balance_log.status=settled - v0.3.2
- [x] [已完成] `go build` + `go vet` + `pnpm run build` 三重编译验证通过 - v0.3.2

#### v0.3.3 日志系统 ✅ v0.3.3 已完成
- [x] [已完成] `log_worker.go` 验证日志异步 worker（`verifyLogCh` 容量 4096，超出丢弃保证验证 API 性能） - v0.3.3
- [x] [已完成] `log_worker.go` 操作日志异步 worker（`operationLogCh` 容量 2048） - v0.3.3
- [x] [已完成] `RecordOperation` 切面 helper：从 gin.Context 抽取 role/userID/username/IP/UA - v0.3.3
- [x] [已完成] `client.go` 14 处 `writeVerifyLog` → `writeVerifyLogCtx(deps, c, ...)` 升级，捕获 IP/UA - v0.3.3
- [x] [已完成] `AdminListOperationLogs` / `AdminListVerifyLogs` / `AdminListLoginFailedLogs` 三表独立查询 Handler - v0.3.3
- [x] [已完成] `AdminExportLogs` CSV 导出（UTF-8 BOM `\xEF\xBB\xBF` 兼容 Excel，上限 10000 条） - v0.3.3
- [x] [已完成] 路由注册 4 条新路由：`/admin/logs/operations` + `/verify` + `/login_failed` + `/export` - v0.3.3
- [x] [已完成] `main.go` 启动 `StartVerifyLogWorker` + `StartOperationLogWorker` - v0.3.3
- [x] [已完成] `go build` + `go vet` 后端编译验证通过 - v0.3.3
- [x] [已完成] 前端 `api/admin.ts` 增补 `LogOperation` / `LogVerify` / `LogLoginFailed` 类型 + 3 个 list API + 1 个 export API - v0.3.3
- [x] [已完成] 前端 `admin/Logs.vue` 升级：el-tabs 三表切换 + 每表独立筛选 + 顶部「导出 CSV」按钮（响应式 H5） - v0.3.3
- [x] [已完成] `pnpm run build` 前端编译验证通过 - v0.3.3
- [迁移] avatar 字段（三表均无对应列）→ v0.4.x 加列后落库

#### v0.3.4 结算与对账闭环 ✅ v0.3.4 已完成
- [x] [已完成] 迁移 007：sys_tenant 增 balance/frozen_balance；新建 tenant_balance_log + tenant_withdraw 表 - v0.3.4
- [x] [已完成] model.go：SysTenant 增 Balance/FrozenBalance；新增 TenantBalanceLog / TenantWithdraw struct - v0.3.4
- [x] [已完成] `pay.go` AdminSettleOrder 改造：事务保护 + FOR UPDATE + 累加开发者 balance + 写 tenant_balance_log - v0.3.4
- [x] [已完成] `tenant_settle.go` 新建：5 个开发者侧 Handler（结算查询/余额概览/流水/我的提现/发起提现） - v0.3.4
- [x] [已完成] `admin_finance.go` 新建：5 个超管 Handler（提现列表/打款/驳回/批量结算/对账报表） - v0.3.4
- [x] [已完成] 批量结算按 tenant_id 分组累计 net_amount，单次最多 100 条 - v0.3.4
- [x] [已完成] 对账报表聚合 SQL（SUM + CASE WHEN）统计订单总额/抽成/应结/已结/未结/已提现/理论余额 - v0.3.4
- [x] [已完成] 路由注册 10 条新路由（adminAuth 5 + tenantAuth 5） - v0.3.4
- [x] [已完成] `go build` + `go vet` 后端编译验证通过 - v0.3.4
- [x] [已完成] 前端 `api/tenantFinance.ts`：6 类型 + 10 API 函数（开发者侧 5 + 超管侧 5） - v0.3.4
- [x] [已完成] 前端 `tenant/Settlements.vue` 开发者结算记录页（余额概览 + 双 Tab：结算记录/余额流水，响应式 H5） - v0.3.4
- [x] [已完成] 前端 `tenant/Withdrawal.vue` 开发者提现申请页（余额概览 + 提现表单 + 提现记录，响应式 H5） - v0.3.4
- [x] [已完成] 前端 `admin/TenantWithdrawalReview.vue` 超管审核页（列表 + 打款对话框 + 驳回对话框，响应式 H5） - v0.3.4
- [x] [已完成] 前端 `admin/Settlements.vue` 升级：双 Tab（结算记录 + 批量结算 / 对账报表 9 卡片） - v0.3.4
- [x] [已完成] 路由注册：admin 加 `/tenant-withdrawal-review`；tenant 加 `/settlements` + `/withdrawal` - v0.3.4
- [x] [已完成] `pnpm run build` 前端编译验证通过 - v0.3.4
- [x] [已完成 2026-07-20] 2FA `backup_codes` DB 持久化（migration 008 三表加 backup_codes 字段 + model 字段 + profile.go `loadUserBackupCodes`/`updateUserBackupCodes`/`consumeBackupCode` + Verify2FA/Disable2FA 改造为 DB 落库 + 兼容 v0.3.x Redis 回退读取；13 个 handler 测试全 PASS） - v0.4.0
- [x] [已完成 2026-07-20] UA 解析库引入（自实现 `pkg/ua` 包，零第三方依赖，20 个测试全 PASS；handler 层 `parseDeviceName` / `detectDeviceType` / `ListLoginDevicesFull` 全部接入） - v0.4.0
- [x] [已完成 2026-07-20] 登录失败日志结构化记录（新建 `internal/logger` 包基于 Go 标准库 `log/slog`，零依赖；`AppConfig` 加 LogLevel/LogFormat/LogOutput；3 处 `_ = err` 替换为 `logger.Error` 结构化日志；6 个 logger 测试全 PASS） - v0.4.0
- [x] [已完成 2026-07-20] 全语言 SDK 扩展（Java / C# / Go / C++ / 易语言 5 个新 SDK；每个含 9 个客户端 API + 独立签名对齐脚本；`sign_alignment_test.go` 从 3 语言扩展到 7 语言 + 元数据校验；11 个测试包全 PASS） - v0.4.0
- [x] [已完成 2026-07-20] JWT jti 精准单点踢出（jti 嵌入 JWT RegisteredClaims.ID + `auth.BlacklistRefreshTokenByJTI` + `revokeSessionByJTI` + KickDevice/Logout/RefreshToken 全部改造为 jti 维度；18 个 auth 测试 + 1 个 middleware JTI 注入测试全 PASS，8 个测试包全绿） - v0.4.0

#### v0.3.5 P0 修复：RSA 脚本 / 数据库迁移 / H5 公共 API / 套餐配额 ✅ v0.3.5 已完成
- [x] [已完成] `scripts/gen_rsa_key.sh` 独立 RSA-4096 密钥对生成脚本（从 baota_deploy.sh 抽取，支持 --force / 自定义输出目录 / 密钥配对校验） - v0.3.5
- [x] [已完成] `internal/migration/migrator.go` 轻量级 SQL 文件迁移机制（schema_migrations 表 + dirty 状态 + 单迁移事务） - v0.3.5
- [x] [已完成] `config.go` 增 MigrationConfig（Auto/Dir）+ MIGRATION_AUTO/MIGRATION_DIR 环境变量 + DSN 加 multiStatements=true + InitContainer 调用 migration.Run - v0.3.5
- [x] [已完成] `docker-compose.yml` 移除 mysql entrypoint 挂载（避免 .down.sql 误执行）+ server 加 MIGRATION_AUTO/MIGRATION_DIR 环境变量 - v0.3.5
- [x] [已完成] `configs/config.yaml.example` 完全重写对齐 Config struct yaml tag - v0.3.5
- [x] [已完成] `internal/handler/public.go` 新建：PublicAppInfo + PublicCardTypes 两个 H5 公共 API（无需鉴权 + DTO 过滤敏感字段） - v0.3.5
- [x] [已完成] `pay.go` GetPayOrder 订单已支付时返回卡密明文（card_keys 字段，供 H5 终端用户查看） - v0.3.5
- [x] [已完成] `router.go` publicGroup 新增 /apps/info + /card_types 两条路由 - v0.3.5
- [x] [已完成] `api/pay.ts` PayOrder 接口加 card_keys 字段 - v0.3.5
- [x] [已完成] `h5/Home.vue` + `h5/PayResult.vue` 接入真实 H5 公共 API + 展示卡密明文 - v0.3.5
- [x] [已完成] `internal/quota/quota.go` 新建套餐配额检查包：ExceededError + CheckMaxApps/MaxCards/MaxAgents/MaxDevices - v0.3.5
- [x] [已完成] `handler/app.go` TenantCreateApp 接入 quota.CheckMaxApps - v0.3.5
- [x] [已完成] `handler/card.go` TenantGenerateCards 接入 quota.CheckMaxCards（替换内联检查） - v0.3.5
- [x] [已完成] `handler/tenant_business.go` TenantGenInviteCode 接入 quota.CheckMaxAgents（区分 Limit==0 不支持招募代理场景） - v0.3.5
- [x] [已完成] `handler/client.go` ClientLogin + ClientBind 接入 quota.CheckMaxDevices（替换内联检查） - v0.3.5
- [x] [已完成] `go build` + `go vet` 后端编译验证通过 - v0.3.5
- [x] [已完成] `pnpm run build` 前端编译验证通过 - v0.3.5

#### 三级公告体系
- [x] [已完成] 统一公告表 notice 读写 - v0.3.0
- [x] [已完成] notice_target 精准投递 - v0.3.0
- [x] [已完成] notice_read 已读记录 - v0.3.0
- [x] [已完成] 平台总公告管理（S-15） - v0.3.0
- [x] [已完成] 开发者公告管理（S-16） - v0.3.0
- [x] [已完成] 应用公告管理（D-10） - v0.3.0
- [x] [已完成] 代理通知（type=agent_notify）自动写入 - v0.3.0
- [x] [已完成] 开发者控制台顶部公告显示区组件（平台+开发者同时显示） - v0.3.0
- [x] [已完成] 代理控制台公告中心（P-08） - v0.2.7
- [x] [已完成 2026-07-20] 首次登录强制弹窗 - v0.4.0（migration 019 + 3 项 notice.popup.* sys_config + admin/tenant/agent 三端 popup 接口 + notice_read 标记幂等 + is_popup=true 已发布未读过滤 + max_unread 上限）
- [x] [已完成 2026-07-20] 公告置顶 + 显眼标签 - v0.4.0（Notice.IsPinned/ShowBadge 字段已有 + AdminListNotices 已按 is_pinned DESC 排序 + admin/tenant Create/Update/List 接口支持 is_popup/show_badge/content_format 字段）
- [x] [已完成 2026-07-20] 平台总公告富文本编辑 - v0.4.0（migration 019 notice.content_format 字段 text/html + 2 项 notice.richtext.* sys_config + admin/tenant Create/Update 接口校验 richtext.enabled 开关 + max_length 长度限制）

#### 云变量与版本管理
- [x] [已完成] 云变量 CRUD - v0.3.0
- [x] [已完成] /api/v1/client/get_var 接口 - v0.2.2
- [x] [已完成] 版本管理 CRUD - v0.3.0
- [x] [已完成] /api/v1/client/version 接口（强制更新检查） - v0.2.2
- [x] [已完成] /api/v1/client/notice 接口 - v0.2.2

#### 数据统计看板
- [x] [已完成] 开发者工作台数据看板（8 数据卡 + 应用 TOP5 + 收入趋势 + 最近订单） - v0.2.6
- [x] [已完成] 超管平台看板（S-01，8 数据卡 + 待办列表 + 收入趋势 + 最近开发者/订单） - v0.2.6
- [x] [已完成] 代理工作台（4 数据卡 + 4 快捷入口 + 最近订单） - v0.2.5
- [x] [已完成 2026-07-20] 验证趋势图（近 30 天独立页） - v0.4.0（migration 019 + 2 项 stats.verify_trend.* sys_config + admin/tenant 两端 /stats/verify_trend 接口 + 按 result 维度聚合 success/fail/banned/expired/device_mismatch/rate_limited + action 维度聚合 login/verify/heartbeat/bind/unbind + days 参数受 sys_config 上下限约束）
- [x] [已完成 2026-07-20] 代理业绩排行 - v0.4.0（migration 019 + 2 项 stats.agent_ranking.* sys_config + admin/tenant 两端 /stats/agent_ranking 接口 + 联表 agent + sys_tenant + app_order + sort_by 支持 total_amount/commission/net_amount/order_count 四种排序 + limit 受 sys_config 上下限约束 + rank 字段）

#### 客户端 SDK（3 语言）
- [x] [已完成 2026-07-20] Python SDK（keyauth-py） - v0.3.6（`sdks/python/`：KeyAuthClient 9 API + HMAC-SHA512/256 + KeyAuthError + CardInfo/DeviceInfo 数据类 + setup.py + README）
- [x] [已完成 2026-07-20] Node.js SDK（keyauth-node） - v0.3.6（`sdks/nodejs/`：KeyAuthClient 9 异步 API + crypto.createHmac('sha512/256') + index.d.ts 类型定义 + 无第三方依赖 + README）
- [x] [已完成 2026-07-20] PHP SDK（keyauth-php） - v0.3.6（`sdks/php/`：KeyAuthClient 9 API + hash_hmac('sha512/256') + cURL 无第三方依赖 + KeyAuthError + composer.json PSR-4 + README + `php -l` 校验通过）
- [x] [已完成 2026-07-20] SDK 文档与示例 - v0.3.6（三语言各自 README 含安装/快速开始/9 API 速查表/签名算法/错误码表）

#### 日志系统
- [x] [已完成] 验证日志写入（异步 channel worker，容量 4096，超出丢弃） - v0.3.3
- [x] [已完成] 操作日志写入（RecordOperation 切面 helper，容量 2048） - v0.3.3
- [x] [已完成] 日志检索与导出（3 表独立查询 + CSV 导出含 UTF-8 BOM，上限 10000 条） - v0.3.3
- [x] [已完成] 日志按月分区表结构（log_verify RANGE PARTITION） - v0.3.0

---

## P2 中（三期商业化）

### [P2] 三期商业化完整版

#### 多级代理
- [x] [已完成] 二级代理支持 - v0.4.0（migration 009 agent.parent_id + level + invite_code.creator_type/creator_agent_id）
- [x] [已完成] 三级代理支持 - v0.4.0（max_level=3 sys_config 控制 + CanCreateSubordinate 校验）
- [x] [已完成] 跨级佣金结算 - v0.4.0（multilevel.DistributeCrossCommission 沿 parent_id 链向上分润，cross_level_2_rate=50% / cross_level_3_rate=20% 可后台调整）
- [x] [已完成] 代理下级邀请码管理 - v0.4.0（POST/GET /agent/invite_codes + 禁用，CreatorType='agent' 标识）
- [x] [已完成] 代理树查询（三端） - v0.4.0（GET /admin/agents/:id/tree + /tenant/agents/:id/tree + /agent/tree）

#### 全语言 SDK
- [x] [已完成 2026-07-20] Java SDK（含 Android） - v0.4.0（`sdks/java/`：JDK 11+ HttpClient + HmacSHA512/256（JDK 17+，回退 HmacSHA256）+ Jackson + Maven 工程；KeyAuthClient 9 API + KeyAuthException；`sdks/tests/Sign.java` 签名对齐脚本；JDK 17+ 签名断言匹配，否则 t.Logf 暴露 mismatch）
- [x] [已完成 2026-07-20] C# SDK（.NET 全平台） - v0.4.0（`sdks/csharp/`：.NET 6+ HttpClient + 反射探测 BouncyCastle 启用 SHA-512/256 否则回退 HMACSHA256 + System.Text.Json；KeyAuthClient 9 API + KeyAuth.Sdk.csproj；`sdks/tests/sign.cs` 签名对齐脚本）
- [x] [已完成 2026-07-20] Go SDK - v0.4.0（`sdks/go/`：`crypto/sha512.New512_256` 原生字节级对齐 + 强类型 struct 返回 + 零第三方依赖；KeyAuthClient 9 API；`sdks/tests/sign.go` 签名对齐脚本）
- [x] [已完成 2026-07-20] C/C++ SDK（含 JNI） - v0.4.0（`sdks/cpp/`：C++17 + libcurl + OpenSSL 1.1+ `EVP_sha512_256` 原生对齐（OpenSSL < 1.1 回退 `EVP_sha256`）+ nlohmann/json + CMake FetchContent；KeyAuthClient 9 API + KeyAuthException；`sdks/tests/sign.cpp` 签名对齐脚本）
- [x] [已完成 2026-07-20] 易语言模块 - v0.4.0（`sdks/epl/`：纯中文 API + 精易模块 v9.0+ 依赖 + HMAC-SHA256（易语言生态无 SHA-512/256，仅在后端回退场景匹配）；`sdks/tests/sign.e.txt` 签名对齐脚本；Windows-only 永久 `t.Skip`）

#### 高级安全
- [x] [已完成 2026-07-20] 异地登录告警 - v0.4.0（migration 018 login_geo_alert 表 + IP 网段比较 IPv4 /24 IPv6 /64 无需 GeoIP + 4 项 risk.geo_login_alert.* 配置可后台调整 + 风控引擎自动写入告警 + admin/geo_alerts 列表/确认/关闭 3 端点）
- [x] [已完成 2026-07-20] 风控规则引擎 - v0.4.0（migration 018 risk_rule + risk_event 表 + internal/risk 包：5 条内置规则 geo_login/new_device/abnormal_ua/abnormal_time/high_frequency + custom 自定义 + 评分累计阈值升级 alert→challenge→block + EvaluateLogin 接入登录流程 ShouldBlock 撤销会话 + admin 风控面板 11 端点 + ~30 测试全 PASS）
- [x] [已完成 2026-07-20] 设备指纹升级（多维度） - v0.4.0（migration 018 ALTER app_device 增加 6 字段 hwid_components/user_agent/client_ip_ext/screen_resolution/timezone/language 向前兼容）
- [x] [已完成 2026-07-20] Cloudflare WAF 集成 - v0.4.0（middleware/cloudflare.go CloudflareRealIP 中间件从 CF-Connecting-IP 取真实 IP + 受信 CIDR 列表校验来源 + 4 项 cloudflare.* 配置可后台调整 + RealIP(c) 工具函数统一 IP 获取入口 + ratelimit/IPBlacklist 已接入 + 5 测试全 PASS）

#### 灰度发布
- [x] [已完成 2026-07-20] 应用版本灰度推送 - v0.4.0（migration 010 app_version 5 字段 + `internal/grayscale` 包 Match/HashBucket/ParseList + ClientVersion 遍历候选版本匹配 + 3 项 sys_config）
- [x] [已完成 2026-07-20] 灰度规则配置（按地区/比例） - v0.4.0（平台/渠道/地区白名单 + Hash 桶 SHA-256(salt:appID:clientID) % 100 + 全局开关 + default_rate/hash_salt 可后台调整 + TenantUpdateVersion 编辑接口 + AdminListVersions/AdminGetVersion 跨租户查询 + 33 个测试全 PASS）

#### 数据备份与恢复
- [x] [已完成 2026-07-20] 数据库自动备份 - v0.4.0（migration 012 system_backup_log 表 + 6 项 backup.* 配置；Manager.CreateBackup 全库 SQL INSERT 序列化 + gzip 压缩 + AES-256-GCM 加密 + SHA-256 checksum + 文件写入 + 审计日志）
- [x] [已完成 2026-07-20] 一键恢复面板 - v0.4.0（Manager.RestoreBackup SHA-256 校验 + AES 解密 + gunzip + 事务化 DELETE+INSERT 防 PK 冲突 + restored_from 关联原备份；AdminRestoreBackup 异步触发 + status 校验）
- [x] [已完成 2026-07-20] 备份文件下载 - v0.4.0（AdminDownloadBackup 下载前强制 checksum 校验，损坏文件拒绝下载；AdminListBackups 分页 + status/backup_type 筛选；AdminBackupStatus 配置+统计+最近成功备份）
- [x] [已完成 2026-07-20] 过期备份清理 - v0.4.0（Manager.CleanupExpired 按 retention_days 清理文件 + 更新审计日志状态为 deleted；AdminCleanupBackups 手动触发）
- 详见 references/09-database-backup-restore.md

#### 在线更新系统
- [x] [已完成 2026-07-20] Webhook 接收 GitHub Push - v0.4.0（GitHubWebhook handler POST /api/v1/public/update/webhook；HMAC-SHA256 签名校验 + X-GitHub-Event 事件类型过滤 + push event 解析 + 分支匹配）
- [x] [已完成 2026-07-20] 自动拉取构建重启 - v0.4.0（Manager.ExecuteUpdate 6 步流程：加锁 → pending 日志 → git fetch+reset → bash 部署脚本 → 健康检查 → 失败回滚；scripts/deploy_update.sh 默认脚本支持 systemd/docker/pm2/none 自适应）
- [x] [已完成 2026-07-20] 后台更新管理面板（S-13） - v0.4.0（AdminUpdateStatus 返回当前 commit+锁状态+自动开关+最近审计日志+成功/失败统计；AdminTriggerUpdate 异步触发；AdminListUpdateHistory 分页查询+status/trigger_source 筛选；AdminGetUpdateLog 单条详情含 log_text）
- [x] [已完成 2026-07-20] 版本回滚 - v0.4.0（Manager.Rollback 回滚到失败日志的 commit_before + 重跑脚本 + 健康检查；maybeRollback 自动回滚若 update.rollback.enabled=1；AdminRollbackUpdate 手动回滚接口）
- [x] [已完成 2026-07-20] 管理员弹窗通知 - v0.4.0（migration 017 新增 update.poll.enabled + update.poll.interval_seconds 2 项 sys_config；AdminUpdatePoll GET /admin/update/poll 轻量轮询端点仅返回 commit+锁状态+最近一次更新元信息 8 字段，不含 log_text/steps_json 重字段；PollIntervalMin=10 强制下限防配置错误打爆后端；前端 UpdateNotifier.vue 组件挂载于 AdminLayout，localStorage 持久化 last_known_commit 跨会话检测更新，自适应间隔每次轮询后用响应 interval_seconds 重置定时器后端调整配置即时生效，ElMessageBox.confirm 弹窗 + notifiedCommit ref 防本会话重复弹窗；13 个测试全 PASS）
- 详见 references/11-github-auto-update.md

#### API 开放平台
- [x] [已完成 2026-07-20] 第三方接入授权 - v0.4.0（migration 016 developer_api_token + webhook_endpoint + webhook_delivery 表 + 8 项 openapi.*/webhook.* 配置；APITokenAuth 中间件：Authorization: Bearer pat_xxx → TokenManager.ValidateToken → 注入 api_tenant_id/api_scopes；RequireScope 中间件 OR 语义权限校验；失败响应统一 401 不区分错误类型防信息泄露；不走完整 OAuth2 简化为 Token + scopes）
- [x] [已完成 2026-07-20] Webhook 事件推送 - v0.4.0（WebhookManager.CreateEndpoint/UpdateEndpoint/DeleteEndpoint/ListEndpoints/GetEndpoint + DispatchEvent 同步发送 + RetryDelivery 手动重试 + ProcessPendingRetries 后台 worker；HMAC-SHA256 签名头 + hmac.Equal 常量时间比较防时序攻击；AES-256-GCM 加密存储 secret；2/4/6 分钟退避重试 + 连续失败阈值自动 disable 端点；5 个业务事件接入：card.generated / order.paid / agent.registered / agent.recharge.approved / agent.withdraw.paid）
- [x] [已完成 2026-07-20] 开发者 API Token 管理 - v0.4.0（TokenManager.GenerateToken/ValidateToken/RevokeToken/ListTokens/GetToken；SHA-512 哈希存储不存明文 + 前 8 位 prefix 用于展示识别 + scopes 权限范围 + TTL 过期 + 单租户数量上限；明文 Token 仅生成时返回一次；13 个 tenant 端点 + 1 个 admin 端点 + 1 个 openapi/whoami 调试端点；61 个测试全 PASS）

#### 监控告警
- [x] [已完成 2026-07-20] 系统监控（CPU/内存/磁盘）+ 阈值告警 - v0.4.0（migration 013 system_metric + system_alert 表 + 9 项 monitor.* 配置；Manager.CollectSystemMetrics gopsutil 采集 + DB 查询在线设备/验证数/错误率；EvaluateAlerts 显式 switch 阈值比较 + 静默期去重 + 自动恢复 + webhook 通知）
- [x] [已完成 2026-07-20] 异常 QPS 告警 - v0.4.0（verify_count 指标 + error_rate 阈值告警 + webhook POST JSON 通知 + 静默期去重）
- [x] [已完成 2026-07-20] 错误率 > 1% 告警 - v0.4.0（CfgKeyThresholdErrorRate 默认 10% 可后台调整为 1% + EvaluateAlerts 自动触发）
- [x] [已完成 2026-07-20] CPU/磁盘阈值告警 - v0.4.0（CfgKeyThresholdCPU 默认 90% / CfgKeyThresholdDisk 默认 85% + 4 条规则从 sys_config 动态构造）
- [x] [已完成 2026-07-20] 后台监控面板（S-11） - v0.4.0（AdminMonitorStatus 配置+活跃告警+24h聚合+最近采集；AdminCollectNow 手动触发；AdminMetricHistory 历史查询；AdminListAlerts 分页；AdminAckAlert 确认告警；AdminResendAlert 重发；AdminCleanupMetrics 清理过期）
- [x] [已完成 2026-07-20] Prometheus + Grafana 集成 - v0.4.x（internal/metrics 包定义 HTTP/业务/系统三类指标；internal/middleware/prometheus.go 采集 HTTP 指标 + 路径规范化避免 label 爆炸；handler/metrics.go 暴露 /metrics 端点（开关+BasicAuth+路径全走 sys_config monitor.prometheus.*）；SystemCollector 自定义 Collector 从 monitor.Manager 拉取真实 CPU/内存/磁盘/在线设备/QPS/错误率；业务埋点 ClientVerify/dispatchPaidOrder/processAgentRegisterPaid/AgentGenerateCards；migration 029 添加 4 项 sys_config；docker-compose 新增 prometheus + grafana 服务（profile=monitoring 可选启用）；deploy/prometheus/ 含 prometheus.yml + rules.yml 7 条告警规则；deploy/grafana/ 含 provisioning + 15 面板综合监控仪表盘 JSON；3 个测试包 40+ 测试全 PASS）

#### v0.4.x 收尾批次（S-04 / D-15 / U-11~14）✅ 2026-07-20 已完成
- [x] [已完成 2026-07-20] S-04 应用审核（上架审核、违规下架） - v0.4.x（migration 023 app 加 audit_status/audit_remark/audited_at/audited_by 4 字段 + app.audit.enabled 配置；admin_business.go 4 端点：pending 列表/audit 审核/offline 下架/online 上架；client.go 验证 API 校验 audit_status=approved；6 个测试覆盖）
- [x] [已完成 2026-07-20] D-15 开发者安全设置（IP 黑名单、频率限制） - v0.4.x（migration 025 tenant_security_config 表；model TenantSecurityConfig；tenant_business.go 2 端点：get 获取/put 更新；middleware/tenant_security.go TenantSecurityMiddleware IP 黑名单 + Redis 频率限制；router clientGroup 挂载中间件；6 个测试覆盖）
- [x] [已完成 2026-07-20] U-11 终端用户订单列表 H5 接入 - v0.4.x（enduser.go H5EndUserListOrders + H5EndUserGetOrder 2 端点；apps/admin/src/views/h5/Orders.vue 列表页 + OrderDetail.vue 详情页；api/enduser.ts 2 API；Profile 菜单加「我的订单」入口；5+5 个测试覆盖）
- [x] [已完成 2026-07-20] U-12 公告详情 H5 页面 - v0.4.x（public.go PublicNoticeDetail 端点 + view_count 并发自增；apps/admin/src/views/h5/NoticeDetail.vue 支持 text/html 渲染；Profile 菜单加「平台公告」入口；8 个测试覆盖）
- [x] [已完成 2026-07-20] U-13 帮助中心 H5 页面 - v0.4.x（apps/admin/src/views/h5/Help.vue 4 分类 FAQ 折叠列表：购卡/支付/卡密/账户安全；Profile 菜单加「帮助中心」入口；前端硬编码 v0.5.x 可改后端）
- [x] [已完成 2026-07-20] U-14 联系客服 H5 页面 - v0.4.x（migration 028 4 项 contact.* sys_config：qq_group/wechat/email/phone；public.go PublicContact 端点；apps/admin/src/views/h5/Contact.vue 展示+复制+跳转；Profile 菜单加「联系客服」入口；3 个测试覆盖）
- 详见 references/10-monitoring-alerts.md

#### 通知系统
- [x] [已完成 2026-07-20] 短信模板（阿里云/腾讯云） - v0.4.0（migration 014 notify_template + notify_log 表 + 16 项 notify.* 配置；aliyunSMSProvider 骨架实现 AccessKeyID 为空返回 ErrProviderNotConfig；SMSProvider 接口支持 mock 注入；CfgKeySMSEnabled / CfgKeySMSProvider / CfgKeySMSAccessKeyID / CfgKeySMSAccessSecretEnc / CfgKeySMSSignName 全部从 sys_config 读取）
- [x] [已完成 2026-07-20] 邮件模板（SMTP） - v0.4.0（smtpEmailProvider 真实调用 net/smtp.SendMail；AES-256-GCM 解密 SMTP 密码；完整邮件头 From/To/Subject/Message-ID/MIME-Version/Content-Type；CfgKeyEmailSMTPHost/Port/Username/PasswordEnc/FromAddress/FromName 全部可后台调整）
- [x] [已完成 2026-07-20] 站内信 - v0.4.0（ChannelInApp 直接成功 + 写 notify_log 表；前端拉取日志展示；CfgKeyInAppEnabled 默认开启）
- [x] [已完成 2026-07-20] 通知模板引擎（S-12） - v0.4.0（Manager.Render 用 strings.NewReplacer 替换 {{var}} 占位符防 SSTI；4 个预置模板 verify_code/verify_code_email/order_paid/agent_commission；租户自定义模板优先 + 平台通用回退）
- [x] [已完成 2026-07-20] 通知日志 + 重试 + 限流 - v0.4.0（Manager.Send 写 pending → 调 provider → 更新状态；Manager.Retry 失败重试最大次数从 sys_config 读取；Manager.CheckRateLimit 单租户每分钟限流查 notify_log 表实时计数）
- [x] [已完成 2026-07-20] 后台通知管理面板（S-13） - v0.4.0（AdminNotifyStatus 配置概览+统计+模板数；AdminListNotifyTemplates/Create/Update/Delete 模板 CRUD；AdminListNotifyLogs/Get 日志查询；AdminRetryNotifyLog 手动重试；AdminTestNotify 测试发送绕过模板查找直接 dispatch）
- [x] [已完成 2026-07-20] 阿里云短信 SDK 完整签名实现 - v0.4.x（notify.go signAliyunRequest 纯函数：HMAC-SHA1 + Base64 + 字典序排序；aliyunSMSProvider.Send 重写为真实 Dysms API HTTP POST 调用；新增 CfgKeySMSRegion/Endpoint 配置；migration 020 追加 2 项 sys_config；TestSignAliyunRequest 测试覆盖签名确定性 + 不同入参出不同签名）
- [x] [已完成 2026-07-20] SMTP SSL 包装 - v0.4.x（notify.go dialSMTPClient 函数支持三模式：ssl=465 隐式 TLS / tls=587 STARTTLS / none=25 明文；smtpEmailProvider.Send 重写为 dialSMTPClient + Auth + Mail + Rcpt + Data 流程；新增 CfgKeyEmailSMTPEncryption/TimeoutSeconds 配置；migration 021 追加 2 项 sys_config；TestDialSMTPClient_EncryptionBranch 测试覆盖三模式错误分支）
- 详见 references/11-notification-system.md

---

## P3 低（优化与扩展）

### [P3] 优化与扩展

#### 性能优化
- [x] [已完成 2026-07-20] MySQL 读写分离 - v0.5.0（MySQLConfig 加 Slaves 字段 + MySQLSlaveConfig 子结构；Container.DBRead + ReadDB() 方法；initReadDB 取第一个从库（简化策略）；从库失败降级走主库；环境变量 MYSQL_SLAVES=slave1:3306,slave2:3306 解析；支持 host:port[:user[:pass]] 灵活格式）
- [x] [已完成 2026-07-20] API 水平扩展（无状态化） - v0.5.0（snowflake.InitWorkerFromRedis 通过 Redis INCR 协调多实例分配 workerID；workerID 范围 0-31 足够数百实例；Redis 不可用降级为默认 workerID；main.go 启动时调用；GetCurrentWorkerID 用于健康检查；状态分布梳理：DB 存 TOTP/BackupCodes/Session 天然共享，Redis 存黑名单/限流/心跳/nonce 天然共享，唯一冲突点 snowflake 已修复）
- [x] [已完成 2026-07-20] 卡密生成性能优化（目标 10000 条/秒） - v0.5.0（crypto.GenerateCardKeys 批量生成函数：单次 rand.Read 预取所有熵 + decodeSegment 字节级解码；card.go TenantBatchGenerateCards 改用批量生成；性能基准：10000 张仅需 17.3ms = 577k 张/秒，目标 10k 张/秒超额 57 倍；6 个性能基准 + 7 个功能测试）
- [x] [已完成 2026-07-20] Redis 集群模式 - v0.5.0（RedisConfig 加 Mode/Addrs/MasterName/Username 字段；initRedis 支持 single/sentinel/cluster 三模式；sentinel 用 NewFailoverClient；cluster 模式配置不完整降级 single；环境变量 REDIS_MODE/REDIS_ADDRS/REDIS_MASTER_NAME/REDIS_USERNAME 支持；splitCommaList 工具函数解析地址列表）

#### 用户体验
- [x] [已完成 2026-07-20] 后台多主题切换 - v0.5.0（variables.scss 所有颜色 SCSS 变量改为 var(--xxx) 引用 + 新建 themes.scss 定义 5 主题 light/dark/blue/purple/green + auto 跟随系统 + stores/theme.ts 6 主题 mode/setMode/toggle/init/isDark getter 同步 html.dark class + ThemeSwitcher.vue 下拉切换器接入 BasicLayout 顶栏 + pinia-plugin-persistedstate 持久化 localStorage + main.ts 引入 element-plus/theme-chalk/dark/css-vars.css 作为 EP 暗黑样式兜底 + index.scss 引入 themes.scss）
- [x] [已完成 2026-07-20] 暗黑模式（可选主题） - v0.5.0（以多主题架构落地，作为内置主题之一；themes.scss dark 主题定义全套 --color-*/--el-* 变量；theme store isDark getter + applyToDocument() 同步 html.dark class 触发 EP dark css-vars；auto 模式通过 prefers-color-scheme 媒体查询 + matchMedia 监听动态切换；原铁律 03「禁暗黑」已被多主题架构取代，variables.scss 注释已更新）
- [x] [已完成 2026-07-20] 移动端响应式优化 - v0.5.0（variables.scss 新增 $bp-mobile-sm=480px + mobile-sm mixin；3 处硬编码 @media 断点统一：WithdrawalReview.vue/RechargeReview.vue 改用 @include mobile（同时 style 块加 lang="scss"）；QrCode.vue 改用 @include mobile-sm；4 处 SCSS lighten()/darken() 与 CSS 变量不兼容报错改用 CSS color-mix()）
- [x] [已完成 2026-07-20] 数据看板数字滚动动效 - v0.5.0（新建 CountUp.vue 组件：requestAnimationFrame + easeOutCubic 缓动 + 零依赖（不引入 gsap）+ 支持 prefix/suffix/decimals/separator/duration/autoplay 6 props + onBeforeUnmount 自动取消 raf + font-variant-numeric: tabular-nums 防抖动；接入 admin/tenant/agent 3 个 Dashboard 共 20 处数字滚动（含 ¥ 金额 decimals=2））

#### 国际化
- [x] [已完成 2026-07-20] 后台 i18n（中/英） - v0.5.0（vue-i18n@9 接入 + i18n/index.ts 入口 + i18n/locales/zh-CN.ts & en-US.ts 词汇表 7 大模块：common/theme/language/layout/login/register/route + LanguageSwitcher.vue 语言切换器接入 BasicLayout 顶栏 + 登录页右上角 + main.ts 注册 i18n + applyLocale 应用 lang 属性 + App.vue ElConfigProvider 响应式切换 Element Plus 内置组件语言 + BasicLayout 接入 i18n（账号设置/退出登录/确定退出登录吗/提示/用户） + 路由标题响应式翻译（meta.titleKey + te/t 辅助 + 监听 locale 变化动态更新 document.title） + login/index.vue 全文翻译 + 表单校验规则响应式跟随 locale + router 每条路由加 titleKey 共 47 条 + 首次访问根据浏览器语言自动选择 + localStorage 持久化 keyauth-locale）
- [x] [已完成 2026-07-20] SDK 多语言文档 - v0.5.0（8 个 SDK 各生成 README.en.md：python/nodejs/php/go/java/csharp/cpp/epl，统一结构：标题 + Installation + Quick Start + 9 API Reference + Signature Algorithm + Error Handling + Error Codes + Compliance + License；每个 SDK 嵌入对应签名兼容性说明如 Python OpenSSL 1.1+ 回退 / Go 无回退字节级对齐 / Java JDK 17+ 与 11-16 回退 / C# BouncyCastle 推荐 / C++ EVP_sha512_256 / 易语言 HMAC-SHA256 + crypto.go:165 回退场景；8 个中文 README 顶部加语言切换链接 **中文** | English）

#### 集成扩展
- [ ] [无限延期 2026-07-20] 独角数卡对接 - v0.5.0（用户决策：先推进支付扩展，发卡对接延后；2026-07-20 用户决策无限延期）
- [ ] [无限延期 2026-07-20] 蓝米发卡对接 - v0.5.0（用户决策：先推进支付扩展，发卡对接延后；2026-07-20 用户决策无限延期）
- [x] [已完成 2026-07-20] USDT-TRC20 加密货币支付 - v0.5.0（v0.5.0 集成扩展批次 3：pkg/payment Provider 抽象 + USDTProvider 实现 + 金额唯一后缀匹配算法（baseUSDT + (orderID%100)/100）+ TronGrid TRC20 交易轮询 + big.Float 精确除法（decimals=6）+ 外部监控 webhook HMAC-SHA256 验签 + usdt:// 二维码协议 + 9 项 pay.usdt.* sys_config + 11 个测试用例全 PASS）
- [x] [已完成 2026-07-20] 钉钉机器人通知 - v0.5.0（v0.5.0 集成扩展批次 1：notify 包 WebhookProvider 接口 + dingtalkWebhookProvider 实现 + HMAC-SHA256 加签（timestamp+"\n"+secret）+ base64 编码 + url.QueryEscape + markdown 消息类型 + @mobiles/@all 支持 + 5 项 notify.dingtalk.* sys_config + 4 个 HTTP 测试用例 + 1 个加签算法测试 + 1 个 @mobiles 测试）
- [x] [已完成 2026-07-20] PayPal 海外支付 - v0.5.0（v0.5.0 集成扩展批次 3：PayPalProvider 实现 + OAuth2 client_credentials + access_token 60s 提前过期缓存 + POST /v2/checkout/orders intent=CAPTURE + PayPal-Request-Id 幂等键 + approve 链接提取 + webhook verify-webhook-signature API 验签 + PAYMENT.CAPTURE.COMPLETED 标记 paid + 6 项 pay.paypal.* sys_config + 8 个测试用例全 PASS）
- [x] [已完成 2026-07-20] Stripe 海外支付 - v0.5.0（v0.5.0 集成扩展批次 3：StripeProvider 实现 + POST /v1/payment_intents + Bearer sk_xxx + Stripe-Version 2023-10-16 + automatic_payment_methods + client_secret 返回 + webhook HMAC-SHA256 验签（t=xxx,v1=xxx 头格式）+ 5 分钟时间容差（过去+未来）+ 常量时间比较 + payment_intent.succeeded 标记 paid + 4 项 pay.stripe.* sys_config + 11 个测试用例全 PASS）
- [x] [已完成 2026-07-20] 企业微信机器人通知 - v0.5.0（v0.5.0 集成扩展批次 1：wecomWebhookProvider 实现 + markdown 消息类型 + subject 加粗前缀作为标题 + 2 项 notify.wecom.* sys_config + 2 个 HTTP 测试用例）
- [x] [已完成 2026-07-20] Telegram Bot 通知 - v0.5.0（v0.5.0 集成扩展批次 1：telegramWebhookProvider 实现 + MarkdownV2 渲染 + escapeTelegramMarkdown 16 字符转义（_*[]()~`>#+-=|{}.!）+ 4096 字符上限自动截断 + "（已截断）"提示 + telegramAPIBase 可测试覆盖 + 3 项 notify.telegram.* sys_config + 3 个 HTTP 测试用例含长消息截断场景）

#### 主题市场（已无限延期）
- [ ] [无限延期 2026-07-20] 多套主题模板 - v0.6.0（用户决策：暂停 v0.6.0 主题开发，优先推进高级分析；2026-07-20 用户决策无限延期）
- [ ] [无限延期 2026-07-20] 主题编辑器 - v0.6.0（用户决策：暂停 v0.6.0 主题开发，优先推进高级分析；2026-07-20 用户决策无限延期）

#### 高级分析
- [x] [已完成 2026-07-20] 用户行为分析 - v0.6.0（migration 032 user_behavior_profile 表 + 21 项 analysis.* sys_config；analysis 包 behavior.go：AggregateUserBehaviorForDate 按 (end_user_id, stat_date) 聚合 log_verify 反查 app_card.end_user_id + 内存聚合 action/result 计数 + distinct IP/Device + upsert 唯一索引；GetBehaviorOverview KPI 总览 + ListUserBehaviors 分页列表 + GetUserBehaviorDetail 按日序列 + GetBehaviorTrend 全局趋势；15 个测试用例覆盖空数据/无 card_id/卡密未绑/正常/幂等/Overview/List/Detail/Trend 全 PASS）
- [x] [已完成 2026-07-20] 卡密使用画像 - v0.6.0（migration 032 card_usage_profile 表；analysis 包 card_profile.go：AggregateCardProfileForDate 直接按 card_id 聚合（无需 JOIN app_card）+ device_mismatch_count 卡密共享特征 + maskCardKey 卡密脱敏（前4+****+后4，长度<=8 返回 ****）；GetCardProfileOverview/ListCardProfiles/GetCardProfileDetail/GetCardProfileTrend；6 个测试用例覆盖空/正常/Overview/List 脱敏/Detail/Trend 全 PASS）
- [x] [已完成 2026-07-20] 风险用户识别 - v0.6.0（migration 032 user_risk_score 表；analysis 包 risk_user.go：ReevaluateUserRiskScore 查 risk_event + evaluateEndUserAnomalies 查 24h log_verify 异常模式（失败率 >50% / 多 IP >=3 / 多设备 >=5）→ 计算评分 raw_score=Σ(hits×weight) → DecayScore 时间衰减（daysSinceLastEvent × 1/decayDays）→ upsert user_risk_score → 自动封禁检查（decayed_score >= critical_threshold 标记 banned=true）；ReevaluateAllRiskScores 批量重算；GetRiskUserOverview/ListRiskUsers/GetRiskUserDetail；BanUser/UnbanUser 手动操作；13 个测试用例覆盖 NoEvents/WithRiskEvents/Decay/EndUserAnomalies_FailRate/EndUserAnomalies_MultiIP/AutoBan/ReevaluateAll/Overview/ListFilterByLevel/Detail/BanExisting/BanNew/Unban 全 PASS）
- [x] [已完成 2026-07-20] 聚合 Worker + Admin API + 路由注册 - v0.6.0（analysis 包 worker.go：StartAggregationWorker 阻塞调用 + runAggregationOnce 聚合昨日+今日 + 重算风险评分 + 间隔从 sys_config analysis.aggregate_interval_seconds 读取（默认 3600s，最小 60s 保护）+ RunAggregationOnceSync 同步版本；handler/analysis.go 16 个 admin 端点：4 behavior + 4 card_profile + 4 risk（含 ban/unban）+ reevaluate + reevaluate_all + aggregate/trigger；router.go 注册 /admin/analysis/* 路由组；deps.go 扩展 AnalysisMgr *analysis.Manager；main.go 启动后台 goroutine analysisCtx + defer analysisCancel；44 个测试用例全 PASS，go build ./... 通过）

---

## 已阻塞项

（暂无）

---

## 已延期项

### 无限延期（5 项，2026-07-20 用户决策）

以下 5 项任务经用户决策标记为「无限延期」，不再纳入近期版本规划，除非用户后续显式恢复：

| # | 任务 | 原版本 | 延期原因 |
|---|---|---|---|
| 1 | 独角数卡对接 | v0.5.0 | 先推进支付扩展，发卡对接延后 → 无限延期 |
| 2 | 蓝米发卡对接 | v0.5.0 | 先推进支付扩展，发卡对接延后 → 无限延期 |
| 3 | 多套主题模板 | v0.6.0 | 暂停 v0.6.0 主题开发，优先推进高级分析 → 无限延期 |
| 4 | 主题编辑器 | v0.6.0 | 暂停 v0.6.0 主题开发，优先推进高级分析 → 无限延期 |
| 5 | v0.5.0 集成扩展批次 2（发卡对接 2 项） | v0.5.0 | 即上 #1 #2，里程碑层标记进行中 → 实质无限延期 |

---

## 任务依赖关系

```
一期 MVP（v0.2.0）
├─ 项目骨架 ✅ ──► 数据库迁移 ✅ ──► 认证模块 ⏳ ──► 多租户中间件 ✅骨架
│                                          │
│                                          ├─► 应用管理 ──► 卡密管理 ──► 设备绑定
│                                          │                          │
│                                          │                          ▼
│                                          │              客户端验证 API
│                                          │
│                                          └─► 平台总支付 ──► 自动发卡
│
└─ 前端三套 ✅ ──► 联调测试 ──► Docker 部署 ✅ ──► 文档 ✅

二期（v0.3.0）
├─ 开发者自有支付 ──► 双层切换 ──► 通知代理
├─ 代理注册付费 ──► 邀请码体系
├─ 代理购买卡密 ──► 余额系统 ──► 佣金结算 ──► 提现
├─ 三级公告 ──► 精准投递
├─ 云变量/版本 ──► 客户端接口扩展
└─ SDK（3 语言）

三期（v0.4.0）
├─ 多级代理 ──► 二级/三级 + 跨级佣金 + 代理树 ✓
├─ 全语言 SDK（8 语言） ✓
├─ 在线更新 ──► Webhook
└─ 监控告警
```

---

## 里程碑

| 版本 | 目标完成日期 | 状态 |
|---|---|---|
| v0.1.0 | 2026-07-19 | ✅ 已完成（规划） |
| v0.2.0 | 2026-07-19 | ✅ 已完成（一期 MVP 骨架） |
| v0.2.1 | 2026-07-19 | ✅ 已完成（认证模块：JWT 双 Token + TOTP 2FA + 登录锁定） |
| v0.2.2 | 2026-07-19 | ✅ 已完成（应用管理 + 卡密管理 + 客户端验证 API） |
| v0.2.3 | 2026-07-19 | ✅ 已完成（平台总支付：彩虹易支付下单/回调/自动发卡/抽成结算） |
| v0.2.4 | 2026-07-19 | ✅ 已完成（前端响应式 H5 全栈：三角色 + 官网 + H5） |
| v0.2.5 | 2026-07-19 | ✅ 已完成（代理核心页面：购卡/订单/佣金/提现，响应式 H5） |
| v0.2.6 | 2026-07-19 | ✅ 已完成（三角色 Profile + 双 Dashboard，响应式 H5） |
| v0.2.7 | 2026-07-19 | ✅ 已完成（全部剩余 16 个 PlaceholderView 替换为真实页面，响应式 H5 完整覆盖） |
| v0.3.0 | 2026-07-19 | ✅ 已完成（后端业务 API 全量实现，替换全部 501 占位） |
| v0.3.1 | 2026-07-19 | ✅ 已完成（v0.3.0 全部「待核实 v0.3.x」归零：字段补全 + AgentRecharge + ListLoginDevices + 登录失败日志） |
| v0.3.2 | 2026-07-19 | ✅ 已完成（代理充值审核闭环 + 提现审核闭环：tenant_finance.go + 双审核页面） |
| v0.3.3 | 2026-07-19 | ✅ 已完成（日志系统：异步 Worker + 三表独立查询 + CSV 导出 + 前端 3 Tab 升级） |
| v0.3.4 | 2026-07-19 | ✅ 已完成（结算与对账闭环：开发者 balance/frozen_balance + tenant_balance_log + tenant_withdraw + 批量结算 + 对账报表 + 双审核页面） |
| v0.3.5 | 2026-07-19 | ✅ 已完成（P0 修复：RSA 脚本 / 数据库迁移 / H5 公共 API / 套餐配额） |
| v0.3.6 | 2026-07-20 | ✅ 已完成（剩余 P1 收尾 + 单元测试 + 客户端 SDK 签名对齐测试） |
| v0.4.0 | 进行中 | ⏳ 进行中（UA 解析迁移 + JWT jti 单点踢出 + 2FA backup_codes DB 持久化 + 登录失败日志结构化 + 全语言 SDK 扩展 + 多级代理体系 + 灰度发布 + 在线更新 + 数据备份恢复 + 监控告警 + 通知系统 + 终端用户体系 + API 开放平台 + 管理员弹窗通知 已完成；14 项迁移全绿） |
| v0.4.x | 2026-07-20 | ✅ 已完成（v0.4.0 收尾 + Prometheus/Grafana 集成 + 12 项 P0/P1 残留项：S-04/D-15/U-11~14 全部闭环 + 阿里云 SMS 完整签名 + SMTP SSL 包装 + 套餐审核 + 租户安全 + H5 终端用户 4 页 + 帮助中心 + 联系客服） |
| v0.5.0 | 进行中 | ⏳ 进行中（性能优化批次 4 项已完成：MySQL 读写分离 + Redis 三模式 + snowflake Redis 协调 + 卡密批量生成 577k/s；UX 批次 4 项已完成：多主题切换 + 暗黑模式 + 移动端响应式优化 + 数字滚动动效；国际化批次 2 项已完成：后台 i18n 中英 + SDK 多语言文档；集成扩展批次 1 已完成 3 项：钉钉机器人 / 企业微信机器人 / Telegram Bot 通知（WebhookProvider 抽象 + 3 provider 实现 + migration 030 + 22 个测试全 PASS）；集成扩展批次 3 已完成 3 项：USDT-TRC20 / PayPal / Stripe 海外支付；**无限延期**：发卡对接 2 项（独角数卡 / 蓝米发卡）—— 2026-07-20 用户决策无限延期） |

---

## v0.3.6 进度统计

- **总任务数**：约 110 项
- **已完成**：约 100 项（v0.2.0 ~ v0.3.6 全部已发布版本累积）
- **测试覆盖**：10 个测试包（crypto/snowflake/epay/quota/heartbeat/middleware/ua/auth/logger/handler）+ 跨语言签名对齐，全 PASS
- **进行中**：0 项
- **待开始**：约 10 项（v0.4.x 商业化）

### 已完成版本汇总（v0.2.0 ~ v0.3.5）

| 版本 | 主题 | 关键交付 |
|---|---|---|
| v0.2.0 | 一期 MVP 骨架 | Go 结构 + 26 张表 + Vue3 三布局 + Docker + 宝塔部署 |
| v0.2.1 | 认证模块 | JWT 双 Token + TOTP 2FA + 登录锁定 + 三角色登录 |
| v0.2.2 | 应用/卡密/客户端 | 应用 CRUD + 密钥轮换 + 卡密批量生成 + 9 个客户端 API + 心跳保活 |
| v0.2.3 | 平台总支付 | 彩虹易支付下单/回调/同步跳转/自动发卡/防重入/超时关闭/抽成结算 |
| v0.2.4 | 前端响应式 H5 | BasicLayout + 移动端抽屉 + 官网 + H5 购卡/查卡 + 2FA 登录 |
| v0.2.5 | 代理核心页面 | Dashboard + Cards + Orders + Commission + Balance 全响应式 |
| v0.2.6 | Profile + Dashboard | 三角色账号设置 + admin/tenant 工作台 8 数据卡 + 趋势图 |
| v0.2.7 | 占位阶段终结 | 16 个 PlaceholderView 全部替换为真实页面 |
| v0.3.0 | 后端业务 API 全量实现 | 17 个 handler 文件，51 个 501 占位全部升级 |
| v0.3.1 | 字段补全 | migration 006 + log_login_failed + refresh_token_device |
| v0.3.2 | 充值/提现审核闭环 | tenant_finance.go 6 个 handler + 双审核页面 |
| v0.3.3 | 日志系统 | 异步 worker + 三表独立查询 + UTF-8 BOM CSV 导出 |
| v0.3.4 | 结算与对账闭环 | sys_tenant.balance + tenant_balance_log + tenant_withdraw + 批量结算 + 对账报表 |
| v0.3.5 | P0 修复 | RSA 脚本 + 轻量级迁移机制 + H5 公共 API + quota 包 |

### 待完成项（约 10 项）

**v0.3.6 已完成（v0.3.6 已发布 2026-07-20）**：
- [x] [已完成 2026-07-20] 卡密 CSV 导入导出（card.go 新增 TenantExportCardsCSV/TenantImportCardsCSV + 前端 Cards.vue 导出/导入对话框）
- [x] [已完成 2026-07-20] 设备强制下线（card.go TenantBanCard 联动 heartbeat.Remove 清 Redis 心跳 + DB 标记 banned）
- [x] [已完成 2026-07-20] 安装向导页面（/install，后端 install.go InstallStatus/Install + 前端 Install.vue 4 步向导 + 路由）
- [x] [已完成 2026-07-20] 代理注册付费流程（AgentRegister 创建 REG 订单 + processAgentRegisterPaid 事务建 Agent + 邀请码状态机闭环 + Register.vue 落地 3 处 TODO + 修 install.go 配置键名 bug）
- [x] [已完成 2026-07-20] 开发者自有易支付回调实现（EpayTenantNotify + processTenantOwnPaidOrder + loadTenantPayConfig）
- [x] [已完成 2026-07-20] 双层支付模式切换逻辑（CreatePayOrder 内 SysPackage.AllowCustomPay + TenantPayConfig.Enabled 双开关，TOP/ORD 前缀分发）
- [x] [已完成 2026-07-20] 套餐 allow_custom_pay 字段生效（CreatePayOrder 内读取生效）
- [x] [已完成 2026-07-20] 客户端 SDK（Python / Node.js / PHP 三语言）
- [x] [已完成 2026-07-20] 单元测试 + 客户端 SDK 签名对齐测试（pkg/crypto + pkg/snowflake + pkg/epay + internal/quota + internal/heartbeat + 跨语言签名对齐）
- [x] [已完成 2026-07-20] 中间件层单元测试（internal/middleware：JWT/Tenant/Signature/RateLimit/IPBlacklist/RecordCardFailure/Response，21 个测试全 PASS）
- [x] [已完成 2026-07-20] v0.3.6 文档同步
- [x] [已完成 2026-07-20] UA 解析库迁移（pkg/ua 自实现 + handler 层接入 + ListLoginDevices 响应增强，20 个测试全 PASS） - v0.4.0
- [x] [已完成 2026-07-20] JWT jti 精准单点踢出（jti 嵌入 JWT + BlacklistRefreshTokenByJTI + revokeSessionByJTI + KickDevice/Logout/RefreshToken 改造为 jti 维度，18 个 auth 测试 + 1 个 middleware JTI 注入测试全 PASS） - v0.4.0
- [x] [已完成 2026-07-20] 2FA backup_codes DB 持久化（migration 008 + 三表加字段 + profile.go loadUserBackupCodes/updateUserBackupCodes/consumeBackupCode + Verify2FA/Disable2FA 改造 + 兼容 v0.3.x Redis 回退；13 个 handler 测试全 PASS） - v0.4.0
- [x] [已完成 2026-07-20] 登录失败日志结构化（internal/logger 包基于 log/slog 零依赖 + AppConfig 加 LogLevel/LogFormat/LogOutput + 3 处 _ = err 替换为 logger.Error 结构化日志；6 个 logger 测试全 PASS） - v0.4.0
- [x] [已完成 2026-07-20] 多级代理体系（migration 009 + agent.parent_id/level + invite_code.creator_type/creator_agent_id + 4 项 sys_config + multilevel 包 DistributeCrossCommission/CanCreateSubordinate/ComputeSubordinateLevel/BuildAgentTree/ListSubordinates + pay.go/agent_business.go/tenant_business.go/admin_business.go 接入 + 7 条新路由 + 27 个 multilevel 测试全 PASS） - v0.4.0

**v0.4.x 三期商业化（约 8 项）**：
- ~~多级代理（二级 + 三级 + 跨级佣金）~~ ✓ 已完成
- ~~全语言 SDK（Java / C# / Go / C++ / 易语言）~~ ✓ 已完成
- ~~高级安全（异地登录告警 + 风控引擎 + Cloudflare WAF）~~ ✓ 已完成
- ~~灰度发布 + Webhook 自动更新~~ ✓ 已完成
- ~~数据备份恢复~~ ✓ 已完成
- ~~API 开放平台（开发者 API Token + Webhook 事件推送 + 第三方接入授权）~~ ✓ 已完成
- ~~监控告警（内置：CPU/内存/磁盘/错误率 + 阈值告警 + webhook 通知 + Prometheus + Grafana 集成）~~ ✓ 已完成（v0.4.0 + v0.4.x）
- ~~通知系统（短信 / 邮件 / 站内信）~~ ✓ 已完成；阿里云 SDK 完整签名 + SMTP SSL 包装 已完成（v0.4.x）
- ~~终端用户体系（H5 用户登录/注册/中心/订单）~~ ✓ 已完成后端；前端 H5 页面接入后续

- ~~高级安全（异地登录告警 + 风控引擎 + Cloudflare WAF）~~ ✓ 已完成
- ~~公告增强（首次登录强制弹窗 + 公告置顶 + 显眼标签 + 富文本编辑）~~ ✓ 已完成
- ~~数据统计看板（验证趋势图 + 代理业绩排行）~~ 已完成

---

**文档版本**：0.6.4
**最后更新**：2026-07-21（v0.6.4 Critical Bug 修复：33 个 migration 全部应用后 `db.AutoMigrate(&model.SysConfig{})` 触发 `ALTER TABLE MODIFY COLUMN created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP` 失败，MySQL 8.0 报 Error 1067 (Invalid default value for 'created_at')。根因：GORM 默认把 time.Time 推断为 datetime(3)（带毫秒），但 migration 001 用 DATETIME（无毫秒）建表。修复：SysConfig struct 的 CreatedAt/UpdatedAt GORM tag 显式声明 type:datetime，让 GORM 不再修改列定义）
**维护者**：KeyAuth SaaS Team
