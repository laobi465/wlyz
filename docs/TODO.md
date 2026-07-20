# 待完成文档 (TODO / Backlog)

格式：`优先级 + 状态 + 条目 + 预计版本 + 备注`

- 优先级：`P0`（紧急）/ `P1`（高）/ `P2`（中）/ `P3`（低）
- 状态：`[待开始]` `[进行中]` `[已阻塞]` `[已延期]` `[已完成]`

---

## P0 紧急（一期 MVP 必须）

### [P0] 一期 MVP 核心闭环

#### 项目骨架搭建 ✅ 已完成
- [x] [已完成] 初始化 Go 项目结构（cmd/internal/pkg/migrations） - v0.2.0
- [x] [已完成] 初始化 Vue3 admin 项目（超管/开发者/代理三布局） - v0.2.0
- [ ] [待开始] 初始化 Vue3 H5 项目（终端用户） - v0.2.0
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
- [ ] [待开始] 应用列表/详情前端页面 - v0.2.0

#### 卡密体系
- [x] [已完成] 卡类套餐 CRUD - v0.2.2
- [x] [已完成] 卡密批量生成算法（SecureRandom + SHA-512 校验位） - v0.2.0
- [x] [已完成] 卡密查询/封禁/解封/删除 - v0.2.2
- [ ] [待开始] 卡密导入导出 CSV - v0.2.0
- [x] [已完成] 卡密状态机（unused/active/expired/banned/disabled） - v0.2.2
- [ ] [待开始] 卡密批量生成前端页面（含弹窗） - v0.2.0

#### 设备绑定
- [x] [已完成] 设备指纹生成算法（CPU+主板+MAC+磁盘哈希） - v0.2.2
- [x] [已完成] 一机一卡密绑定逻辑 - v0.2.2
- [x] [已完成] 设备解绑扣时逻辑 - v0.2.2
- [ ] [待开始] 设备强制下线 - v0.2.0
- [ ] [待开始] 设备列表/封禁前端页面 - v0.2.0

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
- [ ] [待开始] 用户登录/注册（终端用户体系） - v0.4.x

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
- [ ] [待开始] 安装向导页面（/install） - v0.2.0

---

## P1 高（二期增值）

### [P1] 二期增值功能

#### 开发者自有易支付
- [ ] [待开始] 套餐 allow_custom_pay 字段生效 - v0.3.6
- [x] [已完成] 开发者审核充值（D-19） - v0.3.0（v0.3.2 充值审核闭环）
- [ ] [待开始] 开发者支付配置页（D-18）双层支付模式 - v0.3.6
- [x] [已完成] tenant_pay_config 表读写（CRUD 接口已实现） - v0.3.0
- [ ] [待开始] 双层支付模式切换逻辑 - v0.3.6
- [ ] [待开始] 开发者自有支付下单/回调接口（pay.go:528 EpayTenantNotify 仍返回 "fail"） - v0.3.6
- [ ] [待开始] 开发自有支付附加月费订单 - v0.4.x
- [ ] [待开始] 切换支付方式时通知所有代理（站内信+横幅+弹窗） - v0.4.x

#### 代理注册付费流程
- [x] [已完成] 开发者生成邀请码（D-08） - v0.3.0
- [x] [已完成] 邀请码 CRUD + 状态机（active/disabled/exhausted/expired） - v0.3.0
- [x] [已完成] 代理注册页（REG-01，3 步流程）前端骨架 - v0.2.4
- [ ] [待开始] 代理注册费支付（走平台总支付，auth.go:443 AgentRegister 仍为 501 占位） - v0.3.6
- [ ] [待开始] 代理账号自动创建 + 关联开发者 - v0.3.6
- [ ] [待开始] 超管后台代理注册管理（S-17） - v0.3.6

#### 代理购买卡密
- [x] [已完成] 代理充值申请（P-09） - v0.3.0（v0.3.1 AgentRecharge + v0.3.2 审核闭环）
- [x] [已完成] 开发者审核充值（D-19） - v0.3.0（v0.3.2 充值审核闭环）
- [x] [已完成] 代理余额扣款生成卡密 - v0.3.0（AgentGenerateCards 事务化：扣余额→生成卡密→写 deduct 流水→加佣金→写 commission 流水）
- [ ] [待开始] 代理实时扫码购卡（P-10，备用） - v0.4.x
- [x] [已完成] 代理佣金计算（percentage / diff 两种模式） - v0.3.0（AgentGenerateCards 内联实现）
- [x] [已完成] 代理提现申请（P-05） - v0.3.0（v0.3.0 AgentWithdraw）
- [x] [已完成] 开发者审核提现 + 打款（D-14） - v0.3.0（v0.3.2 提现审核闭环：pay + reject）
- [ ] [待开始] 代理独立门户（P-06，仅展示，收款走开发者） - v0.4.x
- [ ] [待开始] 代理子域名绑定 - v0.4.x

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
- [迁移] 2FA `backup_codes` Redis 持久化 → v0.4.x 加表字段后迁移
- [迁移] UA 解析库（mileusna/ua 或 ua-parser）→ v0.4.x 引入
- [迁移] 登录失败日志结构化记录 → v0.4.x 引入 zap/zerolog
- [迁移] JWT jti 精确单设备踢出 → v0.4.x

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
- [ ] [待开始] 首次登录强制弹窗 - v0.4.x
- [ ] [待开始] 公告置顶 + 显眼标签 - v0.4.x
- [ ] [待开始] 平台总公告富文本编辑 - v0.4.x（当前为纯文本）

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
- [ ] [待开始] 验证趋势图（近 30 天独立页） - v0.4.x
- [ ] [待开始] 代理业绩排行 - v0.4.x

#### 客户端 SDK（3 语言）
- [ ] [待开始] Python SDK（keyauth-py） - v0.3.6
- [ ] [待开始] Node.js SDK（keyauth-node） - v0.3.6
- [ ] [待开始] PHP SDK（keyauth-php） - v0.3.6
- [ ] [待开始] SDK 文档与示例 - v0.3.6

#### 日志系统
- [x] [已完成] 验证日志写入（异步 channel worker，容量 4096，超出丢弃） - v0.3.3
- [x] [已完成] 操作日志写入（RecordOperation 切面 helper，容量 2048） - v0.3.3
- [x] [已完成] 日志检索与导出（3 表独立查询 + CSV 导出含 UTF-8 BOM，上限 10000 条） - v0.3.3
- [x] [已完成] 日志按月分区表结构（log_verify RANGE PARTITION） - v0.3.0

---

## P2 中（三期商业化）

### [P2] 三期商业化完整版

#### 多级代理
- [ ] [待开始] 二级代理支持 - v0.4.0
- [ ] [待开始] 三级代理支持 - v0.4.0
- [ ] [待开始] 跨级佣金结算 - v0.4.0

#### 全语言 SDK
- [ ] [待开始] Java SDK（含 Android） - v0.4.0
- [ ] [待开始] C# SDK（.NET 全平台） - v0.4.0
- [ ] [待开始] Go SDK - v0.4.0
- [ ] [待开始] C/C++ SDK（含 JNI） - v0.4.0
- [ ] [待开始] 易语言模块 - v0.4.0

#### 高级安全
- [ ] [待开始] 异地登录告警 - v0.4.0
- [ ] [待开始] 风控规则引擎 - v0.4.0
- [ ] [待开始] 设备指纹升级（多维度） - v0.4.0
- [ ] [待开始] Cloudflare WAF 集成 - v0.4.0

#### 灰度发布
- [ ] [待开始] 应用版本灰度推送 - v0.4.0
- [ ] [待开始] 灰度规则配置（按地区/比例） - v0.4.0

#### 数据备份与恢复
- [ ] [待开始] 数据库自动备份（每日全量+每小时增量） - v0.4.0
- [ ] [待开始] 一键恢复面板 - v0.4.0
- [ ] [待开始] 备份文件下载 - v0.4.0

#### 在线更新系统
- [ ] [待开始] Webhook 接收 GitHub Push - v0.4.0
- [ ] [待开始] 自动拉取构建重启 - v0.4.0
- [ ] [待开始] 后台更新管理面板（S-13） - v0.4.0
- [ ] [待开始] 管理员弹窗通知 - v0.4.0
- [ ] [待开始] 版本回滚 - v0.4.0
- 详见 references/11-github-auto-update.md

#### API 开放平台
- [ ] [待开始] 第三方接入授权 - v0.4.0
- [ ] [待开始] Webhook 事件推送 - v0.4.0
- [ ] [待开始] 开发者 API Token 管理 - v0.4.0

#### 监控告警
- [ ] [待开始] Prometheus + Grafana 集成 - v0.4.0
- [ ] [待开始] 异常 QPS 告警 - v0.4.0
- [ ] [待开始] 错误率 > 1% 告警 - v0.4.0
- [ ] [待开始] CPU/磁盘阈值告警 - v0.4.0

#### 通知系统
- [ ] [待开始] 短信模板（阿里云/腾讯云） - v0.4.0
- [ ] [待开始] 邮件模板（SMTP） - v0.4.0
- [ ] [待开始] 站内信 - v0.4.0

---

## P3 低（优化与扩展）

### [P3] 优化与扩展

#### 性能优化
- [ ] [待开始] MySQL 读写分离 - v0.5.0
- [ ] [待开始] API 水平扩展（无状态化） - v0.5.0
- [ ] [待开始] 卡密生成性能优化（目标 10000 条/秒） - v0.5.0
- [ ] [待开始] Redis 集群模式 - v0.5.0

#### 用户体验
- [ ] [待开始] 后台多主题切换 - v0.5.0
- [ ] [待开始] 暗黑模式（可选主题） - v0.5.0
- [ ] [待开始] 移动端响应式优化 - v0.5.0
- [ ] [待开始] 数据看板数字滚动动效 - v0.5.0

#### 国际化
- [ ] [待开始] 后台 i18n（中/英） - v0.5.0
- [ ] [待开始] SDK 多语言文档 - v0.5.0

#### 集成扩展
- [ ] [待开始] 独角数卡对接 - v0.5.0
- [ ] [待开始] 蓝米发卡对接 - v0.5.0
- [ ] [待开始] USDT/加密货币支付 - v0.5.0
- [ ] [待开始] 钉钉/企业微信通知 - v0.5.0

#### 主题市场
- [ ] [待开始] 多套主题模板 - v0.6.0
- [ ] [待开始] 主题编辑器 - v0.6.0

#### 高级分析
- [ ] [待开始] 用户行为分析 - v0.6.0
- [ ] [待开始] 卡密使用画像 - v0.6.0
- [ ] [待开始] 风险用户识别 - v0.6.0

---

## 已阻塞项

（暂无）

---

## 已延期项

（暂无）

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
├─ 多级代理
├─ 全语言 SDK
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
| v0.3.6 | 待定 | [进行中] 剩余 P1 收尾（文档同步 + 卡密 CSV + 设备强制下线 + 安装向导 + 代理注册付费 + 开发者自有易支付 + SDK 三语言） |
| v0.4.0 | 待定 | [待开始] 三期商业化 |

---

## v0.3.5 进度统计

- **总任务数**：约 110 项
- **已完成**：约 90 项（v0.2.0 ~ v0.3.5 全部已发布版本累积）
- **进行中**：1 项（v0.3.6 文档同步）
- **待开始**：约 19 项（v0.3.6 收尾 + v0.4.x 商业化）

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

### 待完成项（约 19 项）

**v0.3.6 剩余 P1 收尾（约 10 项）**：
- [x] [已完成 2026-07-20] 卡密 CSV 导入导出（card.go 新增 TenantExportCardsCSV/TenantImportCardsCSV + 前端 Cards.vue 导出/导入对话框）
- [x] [已完成 2026-07-20] 设备强制下线（card.go TenantBanCard 联动 heartbeat.Remove 清 Redis 心跳 + DB 标记 banned）
- [ ] [待开始] 安装向导页面（/install，后端 install handler + 前端 Install.vue 4 步向导）
- [ ] [待开始] 代理注册付费流程（auth.go:443 AgentRegister + Register.vue 三处 TODO）
- [ ] [待开始] 开发者自有易支付回调实现（pay.go:528 EpayTenantNotify）
- [ ] [待开始] 双层支付模式切换逻辑
- [ ] [待开始] 套餐 allow_custom_pay 字段生效
- [ ] [待开始] 客户端 SDK（Python / Node.js / PHP 三语言）
- [ ] [待开始] 单元测试 + 集成测试
- [x] [已完成 2026-07-20] v0.3.6 文档同步

**v0.4.x 三期商业化（约 9 项）**：
- 多级代理（二级 + 三级 + 跨级佣金）
- 全语言 SDK（Java / C# / Go / C++ / 易语言）
- 高级安全（异地登录告警 + 风控引擎 + Cloudflare WAF）
- 灰度发布 + Webhook 自动更新
- 数据备份恢复
- API 开放平台
- 监控告警（Prometheus + Grafana）
- 通知系统（短信 / 邮件 / 站内信）
- 终端用户体系（H5 用户登录/注册/中心/订单）

---

**文档版本**：0.3.6  
**最后更新**：2026-07-20  
**维护者**：KeyAuth SaaS Team
