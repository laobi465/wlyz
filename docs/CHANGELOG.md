# 更新日志 (CHANGELOG)

所有显著变更均会记录于此文件。版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范。

格式约定：
- 分类标签：`[新增]` `[修改]` `[修复]` `[移除]` `[废弃]` `[安全]`
- 重大变更标注 `Breaking Change`
- 按版本倒序排列，最新版本置顶

---

## [0.2.5] - 2026-07-19

### [新增] 代理核心页面（响应式 H5）

#### API 模块
- [新增] `api/agent.ts` 代理控制台 API 封装：8 个方法（dashboard / me / card_types / cards / generate / orders / commission / withdraw）+ 9 个类型定义（AgentProfile / AgentDashboard / AgentCardType / AgentCard / AgentOrder / AgentCommission 等）

#### 代理后台页面
- [新增] `views/agent/Dashboard.vue` 代理工作台：4 个数据卡（余额/累计佣金/累计购卡/累计消费）+ 4 个快捷入口 + 最近订单列表，全响应式
- [新增] `views/agent/Cards.vue` 代理购卡页：余额栏（含佣金模式/比例展示）+ 卡类网格（点击进入购卡）+ 购卡对话框（数量/前缀/分组/总价预览/支付后余额预览）+ 购卡结果对话框（卡密列表 + 复制全部）
- [新增] `views/agent/Orders.vue` 代理订单列表：状态筛选 + 订单号/卡类/数量/金额/佣金/状态/渠道/下单时间/支付时间
- [新增] `views/agent/Commission.vue` 佣金记录：4 个统计卡（累计佣金/已提现/可提现/冻结）+ 类型与状态双筛选 + 流水列表（购卡佣金/提现申请/充值/调整）+ 申请提现对话框（金额/收款方式/收款账号/真实姓名/备注）
- [新增] `views/agent/Balance.vue` 余额/提现页：钱包概览大卡 + 3 个统计小卡 + 充值/提现记录列表 + 申请充值对话框

#### 布局与路由
- [重构] `layouts/AgentLayout.vue` 顶部余额标签从占位（¥0.00）改为调用 `/agent/auth/me` 真实获取，并在路由切换时自动刷新
- [修改] `router/index.ts` 代理后台 5 个核心路由由 PlaceholderView 替换为真实页面（dashboard/cards/orders/balance/commission）

#### 业务特性
- [新增] 代理购卡流程：选卡类 → 输入数量/前缀 → 二次确认 → 调用 `/agent/cards/generate` 扣余额生成 → 展示卡密列表
- [新增] 代理提现流程：余额校验 → 收款方式选择（支付宝/微信/银行卡）→ 提交后进入冻结状态待开发者审核
- [新增] 代理充值申请：通过 `/agent/withdraw` 端点提交（type 区分，待 v0.3.0 拆分独立 recharge 端点）
- [新增] 购卡金额预览：实时计算总价 + 支付后余额 + 余额不足前置校验

#### 响应式适配
- [新增] 数据卡 grid：桌面 4 列 / 平板 2 列 / 手机 2 列，字号随断点缩小
- [新增] 卡类网格：`auto-fill minmax(280px, 1fr)`，手机自动堆叠为单列
- [新增] 钱包概览：桌面 `1fr 2fr`（余额卡 + 3 统计卡），手机单列堆叠
- [新增] 表格移动端切换为卡片列表（购卡订单/佣金流水/充值提现记录），关键字段保留

#### 待核实项（铁律 06）
- [待核实] 后端 `/agent/dashboard` `/agent/card_types` `/agent/cards` `/agent/cards/generate` `/agent/orders` `/agent/commission` `/agent/withdraw` 当前均为 501 占位（v0.3.0 交付），前端调用会触发业务错误提示，列表保持空状态（铁律 04 不编造数据）
- [待核实] `/agent/auth/me` 复用 `handler.CurrentUser`，可能正常返回 JWT 中的基本信息但不含 balance/total_commission 字段，待 v0.3.0 扩展
- [待核实] 代理充值暂复用 `/agent/withdraw` 端点提交（remark 标识 `[充值申请]`），待 v0.3.0 实现独立 `/agent/recharge` 端点
- [待核实] 代理佣金模式（percentage / diff）字段需后端在 `agent` 表或 sys_config 中提供，待 v0.3.0 确认数据来源

#### 编译验证
- [验证] `npx vue-tsc --noEmit` 通过（0 错误）
- [验证] `npx vite build` 通过，输出 5 个代理页 chunk（Dashboard 4.80KB / Cards 9.79KB / Orders 3.41KB / Balance 7.16KB / Commission 8.24KB）

---

## [0.2.4] - 2026-07-19

### [新增] 前端响应式 H5 全栈（admin/tenant/agent/官网/终端用户 H5）

#### 全局基础设施
- [新增] `styles/variables.scss` 响应式断点 + mixin（mobile/tablet/desktop）+ 明亮配色变量
- [新增] `styles/index.scss` 响应式工具类（hidden-mobile/visible-mobile-only/card-list）+ 移动端表格紧凑模式 + 移动端对话框/抽屉适配
- [修复] `layouts/AdminLayout.vue` 移除违反铁律 03 的暗黑侧边栏（#001529 → #fff），改为薄包装 BasicLayout
- [修复] `layouts/AgentLayout.vue` 移除暗黑侧边栏（#1f2937 → #fff），改为薄包装 BasicLayout
- [重构] `layouts/TenantLayout.vue` 简化为薄包装 BasicLayout
- [新增] `layouts/BasicLayout.vue` 通用响应式布局（桌面固定侧边栏 + 平板可折叠 + 移动端抽屉式 + 公告横幅插槽 + 顶部右侧插槽）

#### API 模块化
- [新增] `api/auth.ts` 三角色统一登录 + 注册 + refresh + logout + currentUser
- [新增] `api/apps.ts` 应用 CRUD + 重置密钥（支持 all/app_key/app_secret/sign_secret 4 种范围）
- [新增] `api/cards.ts` 卡类 CRUD + 卡密列表/生成/封禁/解禁/删除
- [新增] `api/pay.ts` 终端用户下单 + 订单查询 + 超管结算列表/手动结算 + 支付配置测试
- [重构] `api/http.ts` 请求拦截器注入 Bearer token + 响应拦截器自动 refresh token（含并发去重 + 401 重试 + refresh 失败登出）

#### 状态管理
- [重构] `stores/auth.ts` JWT 双 token（access 2h + refresh 7d）+ 自动续期定时器（提前 5 分钟）+ 持久化 + Cookie 同步 + 登出调用后端黑名单
- [保留] `stores/sysConfig.ts` 平台配置（从 sys_config 加载，铁律 05）

#### 通用组件
- [新增] `components/PageHeader.vue` 响应式页面标题（桌面一行，移动两行）
- [新增] `components/EmptyState.vue` 空状态
- [新增] `components/ResponsiveTable.vue` 桌面端表格 + 移动端自动切换卡片列表 + 分页响应式

#### 登录与注册
- [重构] `views/login/index.vue` 三角色 Tab 切换 + 真实接口对接 + 2FA TOTP 二阶段验证 + 响应式
- [新增] `views/register/TenantRegister.vue` 开发者注册页（账号/密码/邮箱/手机/公司/邀请码）+ 响应式

#### 官网首页（Landing）
- [新增] `views/landing/index.vue` 完整官网首页：顶部导航滚动效果 + Hero 区 + 9 个核心功能 + 6 个适用场景 + 3 个套餐预览 + 5 个 FAQ + CTA + 底部，全部响应式

#### 终端用户 H5（移动端优先）
- [新增] `layouts/H5Layout.vue` H5 专属布局（顶部 Logo + 底部 Tabbar 购卡/查卡，桌面访问也以移动样式呈现）
- [新增] `views/h5/Home.vue` 购卡首页：AppKey 输入 + 卡类列表 + 数量 + 支付方式 + 跳转易支付
- [新增] `views/h5/PayResult.vue` 支付结果页：状态图标 + 轮询订单 + 卡密列表 + 复制
- [新增] `views/h5/Query.vue` 卡密查询页：输入 AppKey + 卡密 + 显示详情
- [新增] `views/h5/CardDetail.vue` 卡密详情页

#### 超管后台
- [新增] `views/admin/Settlements.vue` 结算管理：列表 + 筛选 + 手动结算对话框
- [新增] `views/admin/SysConfig.vue` 系统配置：分组标签页 + 敏感配置隐藏 + 编辑对话框（铁律 05 可视化入口）

#### 开发者控制台
- [新增] `views/tenant/Apps.vue` 应用管理：列表 + 新建/编辑 + 详情 + 重置密钥 + 删除（4 种范围）
- [新增] `views/tenant/CardTypes.vue` 卡类管理：列表 + 新建/编辑（5 种类型：时长/次数/永久/试用/功能）
- [新增] `views/tenant/Cards.vue` 卡密管理：列表 + 批量生成（最多 1000 张/次）+ 封禁/解禁/删除 + 生成结果展示

#### 路由
- [新增] `/` 官网首页路由
- [新增] `/register/tenant` 开发者注册路由
- [新增] `/h5`、`/h5/pay/:orderNo`、`/h5/query`、`/h5/card/:cardKey` 终端用户 H5 路由组
- [新增] `/admin/settlements`、`/admin/sys-config` 超管新页面路由
- [新增] `/tenant/apps`、`/tenant/card-types`、`/tenant/cards` 开发者新页面路由

#### 编译验证
- [验证] `npx vue-tsc --noEmit` 通过（0 错误）
- [验证] `npx vite build` 通过，输出 26 个 JS chunk + 6 个 CSS chunk

---

## [0.2.3] - 2026-07-19

### [新增] 平台总支付（彩虹易支付）下单/回调/自动发卡/抽成结算（P0 核心闭环）

#### 彩虹易支付工具包 `pkg/epay/epay.go`（新建）
- [新增] `Config`：易支付配置（GatewayURL/PID/Secret/SignType）
- [新增] `OrderParams`：下单参数（OutTradeNo/Name/Money/PayType/NotifyURL/ReturnURL/ClientIP）
- [新增] `BuildSubmitURL`：构造 GET 跳转 URL（submit.php，前端直接 location.href）
- [新增] `NotifyParams` + `ParseNotify`：解析异步/同步回调参数
- [新增] `VerifyNotify`：校验回调签名
- [新增] `IsSuccess`：判断回调是否支付成功（TRADE_SUCCESS）

#### 加密工具扩展 `pkg/crypto/crypto.go`
- [新增] `MD5Hex`：MD5 十六进制（32 位小写）
- [新增] `SignEpayParams`：彩虹易支付签名（参数排序 + 拼接 + 追加密钥 + MD5）
- [新增] `VerifyEpaySign`：校验彩虹易支付签名（常量时间比较防时序攻击）

#### 支付 Handler `internal/handler/pay.go`（新建）
- [新增] `CreatePayOrder`：终端用户下单（校验应用/卡类/开发者状态 → 创建 AppOrder pending → 构造易支付跳转 URL → 返回 pay_url）
- [新增] `GetPayOrder`：查询订单状态（含超时自动关单逻辑）
- [新增] `EpayNotify`：异步回调（合并 GET+POST 参数 → 验签 → Redis SETNX 防重入 → 校验金额 → 事务内自动发卡 + 写抽成记录 → 返回 "success"）
- [新增] `EpayReturn`：同步跳转（302 重定向到前端结果页 `/pay/result?order_no=xxx`）
- [新增] `EpayTenantNotify`：开发者自有易支付回调占位（v0.3.0 实现）
- [新增] `AdminListSettlements`：超管查询结算记录列表（分页 + 按 tenant_id/status 筛选）
- [新增] `AdminSettleOrder`：超管手动标记订单已结算（生成结算批次号 STL+日期+ID）
- [新增] `AdminTestPayConfig`：测试平台易支付配置（校验配置完整性 + 解密是否成功）

#### 平台抽成结算记录表 `platform_settlement`（新增）
- [新增] 字段：tenant_id / order_id / order_no / gross_amount / commission_rate / commission_amount / net_amount / status / settled_at / settle_batch_no / settle_method / settle_remark
- [新增] 索引：uk_order（订单 ID 唯一）/ idx_tenant_status（租户+状态联合索引）/ idx_order_no

#### 数据库迁移
- [新增] `migrations/005_pay_settlement.up.sql`：
  - 创建 `platform_settlement` 表
  - 修正 `pay.platform.notify_path` 默认值（`/api/v1/pay/platform/notify` → `/api/v1/pay/notify/epay`）以对齐实际路由
  - 修正 `pay.platform.return_path` 默认值（`/pay/return` → `/api/v1/pay/return/epay`）
  - 新增 6 项配置：sign_type / return_front_url / order_name_prefix / callback_retry_max / settlement.min_amount / settlement.auto_enabled
- [新增] `migrations/005_pay_settlement.down.sql`：回滚脚本

#### 路由（新增端点）
- [新增] `POST /api/v1/pay/order` 终端用户下单
- [新增] `GET /api/v1/pay/order/:order_no` 查询订单状态
- [新增] `GET /api/v1/admin/settlements` 结算记录列表
- [新增] `POST /api/v1/admin/settlements/:id/settle` 手动结算
- [新增] `POST /api/v1/admin/pay/test` 测试支付配置

#### 安全特性
- [安全] 异步回调强制验签（MD5 + 常量时间比较防时序攻击）
- [安全] 回调金额校验（防止伪造回调）
- [安全] Redis SETNX 防重入（同一订单号 60 秒内只处理一次，处理失败释放锁以便重试）
- [安全] 订单状态机校验（仅 pending → paid，已 paid 直接返回成功实现幂等）
- [安全] 商户密钥 AES-256-GCM 加密入库，回调时解密
- [安全] 下单时校验开发者账号状态（active）+ 套餐过期时间
- [安全] 自动发卡事务原子性（订单更新 + 卡密生成 + 抽成记录同时成功或同时失败）

#### 业务特性
- [新增] 抽成计算：优先取套餐 `platform_commission_rate`，为 0 时回退到 `pay.platform.commission_default`（默认 5%）
- [新增] 自动发卡：支付成功后系统自动生成 N 张卡密（CreatorType=auto，OrderID 关联订单）
- [新增] 订单超时关闭：查询订单时若 pending 状态超过 `pay.order_expire_seconds`（默认 1800 秒）自动关闭
- [新增] 同步跳转：支付完成后浏览器 302 跳转到前端 `/pay/result?order_no=xxx`
- [新增] 支持三种支付方式：alipay / wxpay / qqpay
- [新增] 订单号用雪花算法生成（ORD 前缀）
- [新增] 批次号格式：P+YYYYMMDD+6位订单ID余数（区别于开发者手动生成的 B 前缀）

#### 待核实项（铁律 06）
- [待核实] `resolveNotifyURL` 优先用请求头 Host，待核实生产环境是否应单独配置 `notify_base_url`
- [待核实] 自动发卡时 `CreatedBy` 字段填 `order.TenantID`，待核实是否应改为 0（系统）或新增 system_user_id
- [待核实] 订单超时关闭仅在查询时触发，待核实是否应增加定时任务主动扫描
- [待核实] 彩虹易支付部分实现支持 `mapi.php`（API 模式，无页面跳转），当前仅实现 `submit.php`（GET 跳转）

---

## [0.2.2] - 2026-07-19

### [新增] 应用管理 + 卡密管理 + 客户端验证 API（P0 核心闭环）

#### 应用密钥生成器 `pkg/crypto/crypto.go`
- [新增] `GenerateAppKey`：生成 AppKey（ak_ 前缀 + 32 位 hex）
- [新增] `GenerateAppSecret`：生成 AppSecret（as_ 前缀 + 64 位 hex）
- [新增] `GenerateSignSecret`：生成 HMAC 签名密钥（ss_ 前缀 + 64 位 hex）
- [新增] `GenerateHWID`：设备指纹生成（SHA-512(CPU+主板+MAC+磁盘)）

#### 应用管理 Handler `internal/handler/app.go`
- [新增] `TenantCreateApp`：创建应用（含套餐配额校验 + 密钥生成 + AES 加密入库）
- [新增] `TenantListApps`：应用列表（分页 + 关键词搜索）
- [新增] `TenantGetApp`：应用详情
- [新增] `TenantUpdateApp`：更新应用
- [新增] `TenantResetAppKey`：重置 AppSecret/SignSecret（支持密钥轮换，旧 SignSecret 保留 7 天）
- [新增] `TenantDeleteApp`：软删除应用（状态置 disabled）

#### 卡类管理 Handler `internal/handler/card.go`
- [新增] `TenantCreateCardType`：创建卡类（5 种类型：duration/count/permanent/trial/feature）
- [新增] `TenantListCardTypes`：卡类列表
- [新增] `TenantUpdateCardType`：更新卡类

#### 卡密管理 Handler `internal/handler/card.go`
- [新增] `TenantGenerateCards`：批量生成卡密（事务 + 套餐配额校验 + 批次号）
- [新增] `TenantListCards`：卡密列表（多条件筛选 + 分页）
- [新增] `TenantGetCard`：卡密详情
- [新增] `TenantBanCard`：封禁卡密（含状态机校验）
- [新增] `TenantUnbanCard`：解封卡密（根据激活状态恢复到 unused/active/expired）
- [新增] `TenantDeleteCard`：删除卡密（仅 unused 状态可删）

#### 心跳保活服务 `internal/heartbeat/heartbeat.go`
- [新增] `Record`：记录心跳（Redis Sorted Set + Hash 双写）
- [新增] `IsOnline`：检查设备在线状态
- [新增] `Remove`：移除设备心跳（解绑/封禁时调用）
- [新增] `CountOnline`：统计应用在线设备数
- [新增] `ListOnline`：列出在线设备 ID
- [新增] `GetLastHeartbeatAt`：获取最后心跳时间

#### 客户端验证 API `internal/handler/client.go`（全部实现）
- [新增] `ClientLogin`：登录（卡密校验 + 设备自动绑定 + 激活卡密 + 心跳初始化）
- [新增] `ClientVerify`：验证（不增加使用次数，校验设备绑定 + 离线宽限期）
- [新增] `ClientHeartbeat`：心跳（更新 DB + Redis Sorted Set）
- [新增] `ClientBind`：手动绑定设备（多机场景）
- [新增] `ClientUnbind`：解绑设备（扣时 + 移除在线状态）
- [新增] `ClientLogout`：退出登录（仅记录日志）
- [新增] `ClientGetVar`：获取云变量（需校验卡密有效性）
- [新增] `ClientNotice`：获取应用公告
- [新增] `ClientVersion`：检查版本更新

#### 路由（新增端点）
- [新增] `GET /api/v1/tenant/apps/:id` 应用详情
- [新增] `DELETE /api/v1/tenant/apps/:id` 软删除应用
- [新增] `POST /api/v1/tenant/apps/:id/reset_key` 重置应用密钥
- [新增] `GET/POST /api/v1/tenant/card_types` 卡类列表/创建
- [新增] `PUT /api/v1/tenant/card_types/:id` 更新卡类
- [新增] `GET /api/v1/tenant/cards/:id` 卡密详情
- [新增] `POST /api/v1/tenant/cards/:id/ban` 封禁卡密
- [新增] `POST /api/v1/tenant/cards/:id/unban` 解封卡密
- [新增] `DELETE /api/v1/tenant/cards/:id` 删除卡密

#### 数据库迁移
- [新增] `migrations/004_app_card_config.up.sql`：11 项应用/卡密/验证日志配置
  - 应用默认参数：max_devices/heartbeat_interval/heartbeat_timeout/offline_grace/unbind_deduct_seconds
  - 卡密生成：max_batch/charset/segment_length/segment_count
  - 验证日志：async_enabled/sample_rate
- [新增] `migrations/004_app_card_config.down.sql`：回滚脚本

#### 安全特性
- [安全] 创建应用时校验开发者账号状态 + 套餐过期时间
- [安全] 创建应用/卡密时校验套餐配额（MaxApps/MaxCards）
- [安全] AppSecret/SignSecret 使用 AES-256-GCM 加密入库
- [安全] 重置 SignSecret 时旧密钥迁移到 SignSecretPrev 保留 7 天
- [安全] 卡密按 SHA-512 hash 查询（防穷举）
- [安全] 删除应用为软删除（保留审计轨迹）
- [安全] 删除卡密仅 unused 状态可删（防止误删已激活卡密）
- [安全] 卡密封禁/解封有严格状态机（unused/active 可封禁，仅 banned 可解封）
- [安全] 解绑设备扣时（防滥用）
- [安全] 客户端 verify 校验离线宽限期（防断网绕过）

#### 业务特性
- [新增] 卡密 5 种类型支持：duration/count/permanent/trial/feature
- [新增] 一机一卡密绑定（MaxDevices=1）+ 多机绑定（MaxDevices>1）
- [新增] 设备指纹：SHA-512(CPU+主板+MAC+磁盘)
- [新增] 离线宽限期判定（应用级配置）
- [新增] 解绑扣时机制（应用级配置）
- [新增] 卡密批次号管理（B + 日期 + 用户 ID）
- [新增] 卡密明文仅生成时返回一次

#### 待核实项（铁律 06）
- [待核实] `loadCardByCardKey` 优先按 hash 查询，但 SDK 默认传明文，建议确认客户端是否预计算 hash
- [待核实] `ClientVersion` 版本号比较为字符串比较，建议改用 semver 库
- [待核实] `writeVerifyLog` 当前同步写入，v0.3.0 应改为异步队列

---

## [0.2.1] - 2026-07-19

### [新增] 认证模块（P0 核心闭环）

#### 认证工具包 `internal/auth/`
- [新增] `jwt.go`：JWT 双 Token 机制（access + refresh），支持 Token 解析、黑名单、Bearer 提取
- [新增] `totp.go`：TOTP 2FA 工具包（基于 pquerna/otp），支持生成密钥、校验验证码、备用码、AES 加密入库
- [新增] `login_lock.go`：登录失败计数器 + 账号锁定（Redis 滑动窗口），支持锁定状态查询、人类可读剩余时间格式化
- [新增] `context.go`：Redis 操作默认 context

#### 认证处理器 `internal/handler/auth.go`
- [新增] `AdminLogin`：超管登录（用户名/密码/TOTP 校验 + JWT 签发 + 失败计数）
- [新增] `TenantLogin`：开发者登录（同上）
- [新增] `TenantRegister`：开发者注册（注册开关 + 密码长度校验 + 用户名唯一 + 默认套餐 + 试用天数 + 自动签发 Token）
- [新增] `AgentLogin`：代理登录（同超管登录流程）
- [新增] `RefreshToken`：三角色共用 Token 刷新（refresh token 轮换 + 旧 token 加入黑名单）
- [新增] `Logout`：登出（refresh token 加入黑名单）
- [新增] `CurrentUser`：返回 JWT 中的当前用户信息

#### 路由
- [新增] `POST /api/v1/public/auth/refresh`：三角色共用刷新端点
- [新增] `POST /api/v1/{admin|tenant|agent}/auth/logout`：三角色登出端点
- [新增] `GET /api/v1/{admin|tenant|agent}/auth/me`：当前用户信息端点

#### 数据库迁移
- [新增] `migrations/003_auth_config.up.sql`：19 项认证相关 sys_config 配置
  - 登录失败锁定：max_attempts/lock_seconds/window_seconds/require_captcha
  - JWT：access_ttl_seconds/refresh_ttl_seconds/issuer
  - TOTP：issuer/period/digits/algorithm/skew/backup_codes_count
  - 2FA 强制策略：admin/tenant/agent 2fa_required
  - 开发者注册：enabled/default_package_id/trial_days
- [新增] `migrations/003_auth_config.down.sql`：回滚脚本

#### 修复
- [修复] `pkg/crypto/crypto.go`：DecryptAES 函数 ciphertext 变量重用导致类型冲突的 bug
- [修复] `apps/server/go.mod`：module path 缺少 `/apps/server` 后缀导致内部包导入失败

#### 安全特性
- [安全] 登录失败 5 次锁定账号 15 分钟（参数均可后台调整）
- [安全] 账号不存在时不暴露存在性，统一返回「用户名或密码错误」
- [安全] refresh token 轮换机制（旧的立即失效）
- [安全] refresh token 黑名单（登出 / 改密后旧 token 失效）
- [安全] TOTP 密钥 AES-256-GCM 加密入库
- [安全] 2FA 验证码错误不计入账号锁定计数（防止遗忘手机导致账号被锁）
- [安全] 2FA 强制策略可按角色独立配置

#### 待核实项（铁律 06）
- [待核实] `genTenantCode` 当前用纳秒时间戳简化实现，生产环境应改用 crypto/rand 生成不可预测的随机部分

---

## [0.2.0] - 2026-07-19

### [新增] 一期 MVP 骨架（计划中 → 已完成骨架）

#### 数据库层
- [新增] `migrations/001_init_schema.up.sql`：26 张表完整 DDL（平台层 / 应用层 / 代理层 / 公告层 / 安全日志层）
- [新增] `migrations/001_init_schema.down.sql`：回滚脚本
- [新增] `migrations/002_seed_data.up.sql`：默认 sys_config（47 项配置）+ 三档套餐 + 默认超管 + 平台欢迎公告
- [新增] `migrations/002_seed_data.down.sql`：seed 回滚脚本
- [新增] `log_verify` 表按月分区（RANGE PARTITION on created_at）

#### 后端 Go 骨架
- [新增] `cmd/main.go`：HTTP 服务入口 + 优雅关闭
- [新增] `internal/config/config.go`：YAML + 环境变量双层配置加载（铁律 04）
- [新增] `internal/config/cache.go`：sys_config 缓存（GetString/GetInt/GetBool/GetFloat64/GetJSON）+ 缓存穿透保护
- [新增] `internal/model/model.go`：26 张表 GORM 模型 + TableName 方法
- [新增] `internal/middleware/auth.go`：JWT 认证中间件
- [新增] `internal/middleware/tenant.go`：多租户隔离中间件
- [新增] `internal/middleware/signature.go`：HMAC-SHA256 签名验证 + nonce 防重放 + 密钥轮换支持
- [新增] `internal/middleware/ratelimit.go`：Redis 滑动窗口限流 + 失败计数 + 自动 IP 封禁
- [新增] `internal/handler/admin.go`：管理员后台 handler 骨架（含 sys_config CRUD 完整实现）
- [新增] `internal/handler/client.go`：客户端验证 API 骨架（login/verify/heartbeat/bind/unbind/get_var/notice/version/logout）
- [新增] `internal/router/router.go`：5 个路由组（client / admin / tenant / agent / public / pay）
- [新增] `pkg/crypto/crypto.go`：AES-256-GCM + RSA-4096 + HMAC-SHA256 + bcrypt(cost=12) + 卡密生成（4×4 字符 + 8 位校验和）
- [新增] `pkg/snowflake/snowflake.go`：雪花 ID 订单号生成器

#### 前端 admin 骨架
- [新增] Vue3 + TypeScript + Element Plus + Vite + Pinia 项目初始化
- [新增] 三套布局组件：`AdminLayout` / `TenantLayout` / `AgentLayout`（差异化侧边栏主题色）
- [新增] 三类公告横幅组件：`PlatformNoticeBanner` / `DeveloperNoticeBanner` / `AgentNotifyBanner`（同屏显示）
- [新增] 路由配置：30 个子路由 + 角色守卫 + NProgress 进度条
- [新增] `stores/auth.ts`：登录态管理（Pinia + persistedstate + Cookie 同步）
- [新增] `stores/sysConfig.ts`：sys_config 缓存（铁律 05 前端实现）
- [新增] `api/http.ts`：axios 封装 + 401 自动跳转 + 统一错误提示
- [新增] `api/sysConfig.ts`：sys_config 接口封装
- [新增] `views/login/index.vue`：三角色 Tab 切换登录页
- [新增] `views/agent/Register.vue`：代理注册三步流程（邀请码 → 支付 → 完成）
- [新增] `views/error/404.vue`：404 页面
- [新增] `components/PlaceholderView.vue`：业务页面占位组件

#### 部署与运维
- [新增] `Dockerfile`：后端多阶段构建（builder → alpine runtime）
- [新增] `Dockerfile.admin`：前端多阶段构建（node → nginx）
- [新增] `docker-compose.yml`：5 服务编排（mysql / redis / server / admin / gateway）+ 健康检查 + 生产 profile
- [新增] `deploy/nginx/admin.conf`：前端 nginx 配置（SPA 路由 + gzip + 安全头）
- [新增] `deploy/nginx/gateway.conf`：总入口网关（HTTPS + 反向代理 + HSTS）
- [新增] `scripts/baota_deploy.sh`：宝塔面板一键部署脚本
- [新增] `scripts/reset_admin_password.sh`：超管密码重置脚本
- [新增] `.env.example`：环境变量样例（铁律 04：所有敏感字段从环境变量传入）
- [新增] `configs/config.yaml.example`：后端配置文件样例

#### 项目文档
- [新增] `README.md`：项目概览 + 快速部署指南
- [新增] `PROMPT.md`：AI 接手指引文档（铁律 07 实践）
- [新增] `.gitignore`：标准 Go + Vue + 密钥忽略规则

### [安全] 铁律合规
- [安全] 所有可变参数（47 项）已写入 sys_config seed，业务代码通过 `cfgCache.GetXxx` 读取（铁律 05）
- [安全] AES_KEY / JWT_SECRET / DB 密码 / RSA 私钥全部从环境变量传入（铁律 04）
- [安全] 默认超管密码哈希为占位符，强制要求部署后通过 `reset_admin_password.sh` 重置
- [安全] 标注「待核实」项：HMAC-SHA256 变体、Snowflake twepoch、bcrypt 哈希生成命令

### [待实现] v0.2.0 后续任务
- 后端各 handler 业务逻辑（当前均为 501 占位）
- 前端各业务页面（当前均为 PlaceholderView 占位）
- 客户端 SDK（Python / Node.js / PHP）
- 单元测试与集成测试覆盖

---

## [0.1.0] - 2026-07-19

### [新增] 项目初始规划版本

#### 平台基础架构
- [新增] 确定技术栈：Go 1.22 + Gin + GORM（后端）、Vue3 + TypeScript + Element Plus + Vite + Pinia（前端）
- [新增] 确定数据库：MySQL 8.0 + Redis 7
- [新增] 确定部署方式：Docker Compose + 宝塔面板 Docker
- [新增] 确定反代与 SSL：Nginx + Let's Encrypt

#### 多租户体系
- [新增] 租户（开发者）注册、登录、2FA、JWT 认证
- [新增] 多租户数据隔离中间件（自动注入 tenant_id）
- [新增] 套餐体系：免费版 / 专业版 / 企业版
- [新增] 套餐字段：`allow_custom_pay`、`custom_pay_fee`、`platform_commission_rate`

#### 应用管理
- [新增] 应用 CRUD、AppKey/AppSecret/SignSecret 生成与轮换
- [新增] 应用配置：单卡密最大设备数（默认 1，一机一卡）、心跳间隔/超时、离线宽限期、解绑扣时规则
- [新增] 代理佣金模式字段：`agent_commission_mode`（percentage / diff）

#### 卡密体系
- [新增] 卡密类型：时长卡 / 次数卡 / 永久卡 / 试用卡 / 功能解锁卡
- [新增] 卡密生成：手动批量生成、自定义前缀/分组、SHA-512 校验位防伪、SecureRandom 系统熵源
- [新增] 卡密状态机：unused / active / expired / banned / disabled
- [新增] 一机一卡密绑定：设备指纹（CPU+主板+MAC+磁盘多重哈希）、解绑扣时、强制下线

#### 在线验证 API
- [新增] 客户端验证接口：login / verify / heartbeat / bind / unbind / get_var / notice / version / logout
- [新增] HMAC-SHA256 签名协议：X-App-Key / X-Timestamp / X-Nonce / X-Signature
- [新增] 时间戳 ±5 分钟校验、Nonce 5 分钟防重放（Redis 去重）
- [新增] RSA-4096 响应签名（fail-closed）
- [新增] 心跳保活：Redis Sorted Set 维护在线状态、超时判定、离线宽限期

#### 支付系统（双层模式）
- [新增] 平台总支付：超管后台 S-06 配置易支付网关/商户号/密钥、平台抽成比例、结算周期
- [新增] 开发者自有易支付：套餐允许时开发者可开通，资金直达开发者账户
- [新增] 切换支付方式时自动通知代理（站内信 + 控制台横幅 + 强制确认弹窗）
- [新增] `tenant_pay_config` 表（租户支付配置，AES-256-GCM 加密密钥）

#### 代理体系
- [新增] 代理注册机制：开发者生成邀请码（含有效期/次数/授权范围）→ 代理填写邀请码 → 支付平台注册费 → 关联开发者
- [新增] 代理邀请码表 `agent_invite_code`、代理注册订单表 `agent_registration_order`
- [新增] 代理购买卡密两种方式：预付余额扣款（推荐）/ 实时支付购卡（备用）
- [新增] 代理余额流水表 `agent_balance_log`、代理提现表 `agent_withdraw`
- [新增] 代理佣金模式：按比例（percentage）/ 按差价（diff，默认推荐）
- [新增] 代理独立门户（P-06）：仅展示品牌/定价，收款统一走开发者支付通道

#### 公告系统（三级体系）
- [新增] 平台总公告（type=platform）：超管发布，开发者+代理同看，显眼"平台公告"红色标签
- [新增] 开发者公告（type=developer）：超管发布，所有开发者可见
- [新增] 应用公告（type=app）：开发者发布，该应用终端用户可见
- [新增] 代理通知（type=agent_notify）：系统自动通知代理（如支付方式变更）
- [新增] 公告精准投递表 `notice_target`、已读记录表 `notice_read`
- [新增] 公告特性：置顶、强制弹窗、显眼标签、起止时间、富文本编辑

#### 安全防护（借鉴布丁卡密七层防御）
- [新增] DDoS 防御：Cloudflare WAF + Nginx 限流 100r/s + 敏感接口 10r/min + 自动封禁
- [新增] Web 安全：CSP / HSTS / X-Frame-Options / CSRF Token 双重验证 / XSS 转义
- [新增] 接口签名：HMAC-SHA256 + Nonce + Timestamp + 常量时间比较防时序攻击
- [新增] 注入防护：GORM 参数化查询 + 路径遍历拦截
- [新增] 卡密防伪：SHA-512 校验位 + SecureRandom（约 10^18 次尝试）
- [新增] 隐私保护：敏感字段 AES-256-GCM 加密 + bcrypt (cost=12) 密码 + API 脱敏
- [新增] IP 风控：黑名单（手动+自动）+ 异地登录告警 + 频率限制

#### 数据统计与日志
- [新增] 数据看板：卡密总数 / 在线设备 / 今日销量 / 本月收入 / 验证趋势图 / 销量 TOP
- [新增] 验证日志表 `log_verify`（按月分区）
- [新增] 操作日志表 `log_operation`

#### 客户端 SDK
- [新增] 规划 8 语言 SDK：Python / Node.js / Java / C# / Go / PHP / C++ / 易语言
- [新增] SDK 核心方法：verify() / bind() / heartbeat() / get_var()

#### 部署与运维
- [新增] Docker Compose 一键部署方案
- [新增] 宝塔面板 Docker 安装脚本
- [新增] 健康检查接口 `/health`、Docker healthcheck
- [新增] 在线更新机制（references/11）：Webhook 接收 GitHub Push + 自动拉取构建重启
- [新增] 数据备份：每日全量 + 每小时增量 + Redis RDB

---

## 待发布版本规划

### [0.2.0] - 一期 MVP（计划中）
- 核心验证闭环：租户注册 → 应用创建 → 卡密生成 → 客户端登录验证 → 心跳保活 → 解绑
- 多租户隔离中间件
- 平台总支付 + 自动发卡
- 开发者控制台核心页面
- 代理控制台核心页面
- 平台超管后台核心页面
- Docker Compose 部署

### [0.3.0] - 二期增值版（计划中）
- 开发者自有易支付通道
- 代理注册付费流程
- 代理佣金结算与提现
- 三级公告体系
- 云变量远程下发
- 版本管理与强制更新
- 数据统计看板
- Python / Node.js / PHP SDK

### [0.4.0] - 三期商业化完整版（计划中）
- 多级代理（二级/三级）
- 全语言 SDK（Java / C# / Go / C++ / 易语言）
- 灰度发布
- API 开放平台与 Webhook 事件推送
- 在线更新管理系统
- 数据备份与恢复面板
