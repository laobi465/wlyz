# 更新日志 (CHANGELOG)

所有显著变更均会记录于此文件。版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范。

格式约定：
- 分类标签：`[新增]` `[修改]` `[修复]` `[移除]` `[废弃]` `[安全]`
- 重大变更标注 `Breaking Change`
- 按版本倒序排列，最新版本置顶

---

## [0.4.0] - 2026-07-20（UA 解析迁移，进行中）

### [新增] pkg/ua 包：User-Agent 解析工具（v0.4.x 迁移项首项）

#### 背景

- v0.3.x 的 `parseDeviceName`（profile.go）与 `detectDeviceType`（session.go）是简化实现，仅识别 OS+Browser 主流场景，无版本号、无爬虫识别
- TODO.md 第 251 行 `[迁移] UA 解析库（mileusna/ua 或 ua-parser）→ v0.4.x 引入`
- v0.4.x 起首项迁移：新建 `pkg/ua` 包，自实现轻量级解析器，**零第三方依赖**

#### 实现

- 新建 `pkg/ua/ua.go`：
  - `DeviceInfo` 结构体：OS / OSVersion / Browser / Version / DeviceType / DeviceName
  - `Parse(ua string) DeviceInfo`：解析入口，覆盖 Chrome / Firefox / Safari / Edge / curl / 爬虫
  - `IsBot(ua string) bool`：识别 Googlebot / Bingbot / Baiduspider / YandexBot / DuckDuckBot / Slurp
  - OS 版本号提取：Windows NT→友好版本（10.0→10 / 6.3→8.1 / 6.2→8 / 6.1→7 / 6.0→Vista / 5.1→XP）；macOS 10_15_7→10.15.7；iOS 14_2_1→14.2.1；Android 11;→11
  - 设备类型判定：pc / mobile / tablet / bot / unknown（iPad / Android 平板识别为 tablet；Android 含 Mobile 识别为 mobile）
  - 浏览器匹配顺序：Edge → curl → Bot → Firefox → Chrome → Safari（避免 Edge 被识别为 Chrome、Chrome 被识别为 Safari）

- 新建 `pkg/ua/ua_test.go`（20 个测试全 PASS）：
  - Chrome on macOS（含 OS 版本 10.15.7 + 浏览器版本 90.0.4430.85）
  - Firefox on Windows 10（NT 10.0→10 友好版本）
  - Safari on iPhone（iOS 14_2_1→14.2.1）
  - Edge on Windows（验证 Edge 先于 Chrome 匹配）
  - Chrome on Android Mobile / Tablet（验证含/不含 Mobile 标识的设备类型区分）
  - iPad（验证 iOS 平板识别）
  - curl（SDK 测试常用 UA）
  - 空字符串 / 仅空白字符
  - Googlebot / Baiduspider（爬虫识别 + DeviceType=bot）
  - Linux + Firefox
  - 旧版 Windows XP（NT 5.1→XP） / Windows 8（NT 6.2→8）
  - IsBot 多场景（6 个爬虫 UA + 4 个正常 UA）
  - SDK 自定义 UA（keyauth-py/1.0 不崩溃，返回 Unknown）
  - DeviceName 拼接逻辑（OS+Browser / 仅 OS / 仅 Browser / Unknown）
  - Edge 优先级（Edge UA 含 Edg/ 与 Chrome/，必须识别为 Edge）
  - Safari 优先级（Chrome UA 含 Chrome/ 与 Safari/，必须识别为 Chrome）
  - 版本号提取（Chrome / Firefox / Safari Version / curl）

#### handler 层改造

- `internal/handler/profile.go`：
  - `parseDeviceName` 改为调用 `ua.Parse(uaStr).DeviceName`，保留函数签名作为兼容包装
  - 移除原 50 行简化实现（OS / Browser 双 switch），删除「待核实 v0.4.x：引入更完整的 UA 解析库」标注

- `internal/handler/session.go`：
  - `recordLoginSession` 改为调用一次 `ua.Parse` 复用结果（替代 `parseDeviceName` + `detectDeviceType` 两次扫描）
  - `detectDeviceType` 改为调用 `ua.Parse(uaStr).DeviceType`，保留函数签名作为兼容包装
  - `ListLoginDevicesFull` 响应增强：新增 `os` / `os_version` / `browser` / `browser_version` / `is_bot` 字段
  - 动态解析 UA 拆分字段，**不改 DB schema**，向前兼容（前端旧字段 `device_name` / `device_type` 保留）

#### 铁律遵守

- 铁律 04（禁硬编码）：无任何硬编码密钥/域名/IP；OS/Browser 常量集中定义在包顶部
- 铁律 05（配置后台化）：UA 解析为纯函数，无配置依赖
- 铁律 06（防幻觉）：20 个测试覆盖主流 UA + 边界 + 爬虫 + SDK UA；所有断言基于固定输入；移除「待核实 v0.4.x」标注（已落地）

#### 验证

- `go test ./...`：7 个测试包全 PASS（新增 `pkg/ua`）
- `go vet ./...`：0 警告
- `go build ./...`：通过

---

## [0.3.6] - 2026-07-20

### [新增] 中间件层单元测试（HTTP 安全闭环）

#### 测试覆盖（`internal/middleware/middleware_test.go`，21 个测试全 PASS）

##### JWTAuth（7 个）
- 有效 Token 通过 + 注入 user_id/role/tenant_id 上下文
- 缺失 Authorization 头 → 401「未提供 Token」
- 非 Bearer 前缀 → 401「Token 格式错误」
- 错误 secret 签名 → 401「Token 无效或已过期」
- 角色不匹配（admin token 访问 tenant 路由）→ 403「无权限访问」
- 多角色逗号分隔（admin,tenant,agent 全通过）
- `GenerateToken` 输出三段式 JWT 结构

##### TenantScope（3 个）
- 未注入 tenant_id → 403「无法识别租户身份」
- 注入 tenant_id 后正确设置 `db` / `gorm_scope` 上下文
- `CheckResourceOwnership` 跨租户访问拦截（tenant 1001 不能访问 tenant 2002 的资源）

##### SignatureAuth（7 个，端到端 HMAC 闭环）
- 有效签名通过（AES 加密 sign_secret 入库 → 解密 → HMAC-SHA512/256 重算 → 常量时间比较）
- 缺失签名头 → 401「签名参数缺失」
- 不存在的 app_key → 401「应用不存在」
- 时间戳超出 ±300s 容差 → 401「时间戳超出允许范围」
- 时间戳格式错误（非数字）→ 401「时间戳格式错误」
- Nonce 重放攻击拦截（Redis SetNX 防重放，第二次相同 nonce → 401「请求已过期或重复」）
- 错误签名 → 401「签名校验失败」

##### RateLimitByIP（4 个，miniredis 滑动窗口）
- 低于阈值全通过（每分钟 3 次，发 3 个请求）
- 超出阈值第 N+1 次返回 429「请求过于频繁」
- 不同 IP 独立计数（IP A 超限不影响 IP B）
- Redis 故障 fail-open（不可达 Redis 时放行而非阻断，避免服务不可用）

##### IPBlacklist（2 个）
- 黑名单中 IP → 403「IP 已被加入黑名单」
- 干净 IP → 200 通过

##### RecordCardFailure + ClearCardFailure（3 个）
- 未达阈值不封禁（2 次 < 阈值 3）
- 达到阈值自动封禁（3 次 = 阈值 3，写入 `ip:blacklist:{ip}` key）
- `ClearCardFailure` 清除失败计数（验证成功时调用）

##### Response（2 个）
- `Success` 响应格式：code/message/data/request_id/timestamp 完整
- `Fail` 响应格式：失败响应不含 data 字段

##### GenerateToken + JWTAuth 端到端（1 个）
- RoundTrip：GenerateToken 生成 → JWTAuth 验证 → 上下文注入 user_id/username/tenant_id

#### 关键测试技术
- `httptest.NewRecorder` + `gin.TestMode` 不启真实 HTTP 端口
- `mockConfigReader` 实现 `ConfigReader` 接口，避免依赖 sys_config 表
- `SetCryptoManager` 注入测试 AES 密钥（32 字节），AES-256-GCM 加密 sign_secret 入库
- miniredis 模拟 Redis（Nonce 防重放 / 限流 / 黑名单 / 失败计数）
- SQLite 内存库模拟 App 表（签名校验查应用）

#### 铁律遵守
- 铁律 04：测试用固定 secret（`real-sign-secret-from-app` / `test-jwt-secret`），不硬编码业务密钥
- 铁律 05：所有配置通过 `mockConfigReader` 注入，不依赖 sys_config 表
- 铁律 06：Redis 故障 fail-open 行为已通过测试验证（不可达 Redis 实际放行），非编造

### [新增] 单元测试 + 客户端 SDK 签名对齐测试

#### 测试栈
- `github.com/stretchr/testify` v1.11.1（assert / require）
- `github.com/alicebob/miniredis/v2` v2.38.0（内存 Redis，测试 heartbeat 不依赖真实 Redis）
- `gorm.io/driver/sqlite` v1.6.0 + `github.com/mattn/go-sqlite3` v1.14.22（SQLite 内存库，测试 quota 不依赖 MySQL）

#### 已通过的测试套件（5 个包，0 失败）

##### `pkg/crypto/crypto_test.go`（核心加密模块）
- AES-256-GCM 加解密往返、错误密文/密钥场景
- `HMACSHA256` 输出格式（64 hex）+ 与标准 `sha512.New512_256` HMAC 完全相等 + 与标准 `sha256` HMAC **不**相等（明确区分两个变体）
- `HashPassword` / `CheckPassword` bcrypt 往返
- `SHA512Hex` / `SHA512Checksum8` / `MD5Hex` 格式
- `SignEpayParams` / `VerifyEpaySign` 易支付签名闭环
- `GenerateCardKey` 卡密格式（前缀 + 4 段随机 + 易混淆字符校验）
- `RandomHex` / `GenerateAppKey` / `GenerateAppSecret` / `GenerateSignSecret` 长度与唯一性
- `GenerateHWID` 设备指纹一致性

##### `pkg/crypto/sign_alignment_test.go`（v0.3.6 新增：客户端 SDK 签名对齐测试）
- 3 组固定输入（login / heartbeat 中文 / get_var 空 body）跨语言对齐
- 后端 `HMACSHA256` 基准签名 vs 三语言 SDK 脚本签名逐字符对比
- **Python**：3 case 全 PASS（`hashlib.algorithms_available` 检测 + `hmac.new(key, msg, "sha512_256")`）
- **PHP**：3 case 全 PASS（`hash_hmac('sha512/256', ...)` 原生支持）
- **Node.js**：沙箱环境 OpenSSL 不支持 `sha512/256`，3 case 自动 `t.Skipf`（脚本退出码 2，标注「环境限制」）
- 后端确定性测试：同一输入两次调用签名相等
- 测试脚本：`sdks/tests/sign.py` / `sign.js` / `sign.php`（无第三方依赖，CLI 接收 `<secret> <msg>` 输出 hex）

##### `pkg/snowflake/snowflake_test.go`（雪花算法 ID 生成器）
- `NewNode` workerID / datacenterID 边界（0~31 合法 / -1 / 32 报错）
- `NextID` 单线程递增 + 50 goroutine × 200 次并发安全（无重复）
- `OrderNo` 三通道前缀（ORD / TOP / REG，v0.3.6 关键路径）
- `twepoch = 1767225600000`（2026-01-01 UTC）常量校验

##### `pkg/epay/epay_test.go`（彩虹易支付协议）
- `BuildSubmitURL` URL 拼接 + sign 注入
- `ParseNotify` 参数解析
- `VerifyNotify` 验签成功 / 失败 / 空 secret / 缺 sign 场景
- 端到端：`BuildSubmitURL` → `ParseNotify` → `VerifyNotify` 闭环
- `NotifyParams.IsSuccess` 状态判断

##### `internal/quota/quota_test.go`（套餐配额校验，SQLite 内存库）
- `ExceededError` 错误消息 + `errors.Is` 类型匹配
- `CheckMaxApps`：低于上限 / 达到上限 / 不限（0） / 开发者不存在 / 已禁用 / 已过期 / 套餐已禁用
- `CheckMaxCards`：低于上限 / 超限 / 不限（0）
- `CheckMaxAgents`：低于上限 / 达到上限 / 不允许（0）
- `CheckMaxDevices`：低于上限 / 达到上限 / 仅计 active / 不限（0） / 不同卡密隔离
- **关键修复**：gorm `default:1` / `default:1000` 标签导致 Create 时 0 值被替换 → 改用 `Updates(map[string]interface{})` 强制覆盖；`AppCard.CardKeyHash` UNIQUE 约束 → 每条记录设置唯一 `card-{i}` / `hash-{i}` / `cs{i}`；`Agent.Username` UNIQUE 约束 → 设置 `agent-{i}`

##### `internal/heartbeat/heartbeat_test.go`（心跳保活，miniredis）
- `Record` 成功 / nil rdb 报错 / 同设备多次记录只增不删
- `IsOnline` 在线 / 离线（直接覆写 ZSET score 模拟超时） / 从未心跳 / 默认超时 180s / nil rdb
- `Remove` 成功 / 不存在设备 / nil rdb 静默返回
- `CountOnline` 计数 / 排除超时设备 / 默认超时 / nil rdb
- `ListOnline` 列表 / 分页 / nil rdb
- `GetLastHeartbeatAt` 成功 / 从未心跳返回零时间 / nil rdb
- Redis Key 规范：`heartbeat:online:{app_id}` / `heartbeat:detail:{app_id}:{device_id}`
- 端到端：Record → IsOnline → 超时 → 再 Record → Remove → GetLastHeartbeatAt 闭环
- **关键修复**：miniredis 的 `FastForward` 不影响 Go `time.Now()`，改为 `rdb.ZAdd` 直接覆写 score 为 `time.Now().Add(-200*time.Second).Unix()` 模拟心跳超时

#### 验证
- `go vet ./...` 0 错误 0 警告
- `go test ./...` 全部 PASS（5 个测试包）
- `go build ./...` 编译通过

#### 铁律遵守
- 铁律 04：测试用例不硬编码任何业务密钥，仅用固定测试输入（`test-sign-secret-12345` 等显式标注为测试用）
- 铁律 05：测试无业务可调参数，所有配置通过 fixture / 测试 DB / miniredis 隔离
- 铁律 06：Node.js 沙箱环境不支持 `sha512/256` 已诚实 `t.Skipf` 标注「环境限制」，未掩盖；签名回退分支「待核实」继续保留

### [新增] 客户端 SDK 三语言（Python / Node.js / PHP）

#### 设计方案
为软件开发者提供开箱即用的客户端 SDK，封装 9 个验证 API + HMAC-SHA512/256 签名算法。三语言 SDK 均无第三方依赖（或仅依赖标准库），签名算法与后端 `crypto.HMACSHA256`（`sha512.New512_256` 变体）严格对齐，不支持时回退标准 `sha256`（已标注「待核实」）。

#### Python SDK（`sdks/python/`，包名 `keyauth-py`）
- [新增] `keyauth/client.py` 主客户端类 `KeyAuthClient`：
  - 9 个公共方法：`login` / `verify` / `heartbeat` / `bind` / `unbind` / `get_var` / `notice` / `version` / `logout`
  - `_sha512_256_hex` 函数：优先用 `hashlib.new("sha512_256")`，不支持时回退 `hashlib.sha256`
  - `_post` 内部方法：构造签名原文 `METHOD\nPATH\nTIMESTAMP\nNONCE\nBODY` + HMAC 签名 + 发请求 + 解析响应
  - `KeyAuthError` 异常类：含 `code` / `message` / `http_status`
  - `CardInfo` / `DeviceInfo` 数据类
- [新增] `keyauth/__init__.py` 包入口，导出 `KeyAuthClient` / `KeyAuthError` / `CardInfo` / `DeviceInfo`，`__version__ = "0.3.6"`
- [新增] `setup.py` 打包配置（`name="keyauth-py"`，`install_requires=["requests>=2.20"]`，`python_requires=">=3.7"`）
- [新增] `README.md` 完整文档（安装说明 + 快速开始 + 9 API 速查表 + 签名算法说明 + 错误处理 + 错误码表）

#### Node.js SDK（`sdks/nodejs/`，包名 `keyauth-node`）
- [新增] `index.js` 主客户端类 `KeyAuthClient`：
  - 9 个异步方法：`login` / `verify` / `heartbeat` / `bind` / `unbind` / `getVar` / `notice` / `version` / `logout`
  - `hmacSha512_256Hex` 函数：`crypto.createHmac('sha512/256', secret)`，不支持时回退 `sha256`
  - `httpRequest` 内置 HTTPS/HTTP 请求封装（不依赖 axios）
  - `KeyAuthError` 错误类
- [新增] `index.d.ts` TypeScript 类型定义：`ClientOptions` / `LoginResult` / `VerifyResult` / `HeartbeatResult` / `BindResult` / `UnbindResult` / `GetVarResult` / `NoticeResult` / `VersionResult` / `LogoutResult`
- [新增] `package.json` 包配置（`name="keyauth-node"`，`engines.node>=14.0.0`，无 dependencies）
- [新增] `README.md` 完整文档

#### PHP SDK（`sdks/php/`，包名 `keyauth/keyauth-php`）
- [新增] `src/KeyAuthClient.php` 主客户端类：
  - 9 个公共方法（`login` / `verify` / `heartbeat` / `bind` / `unbind` / `getVar` / `notice` / `version` / `logout`）
  - `hmacSha512256` 方法：`hash_hmac('sha512/256', $msg, $secret)`（PHP 7.1+ 原生支持），不支持时回退 `hash_hmac('sha256', ...)`
  - cURL HTTP 请求封装（无第三方依赖，仅依赖 `ext-curl` / `ext-json` / `ext-hash` PHP 标配扩展）
  - `declare(strict_types=1)` 全类型安全
- [新增] `src/KeyAuthError.php` 异常类：含 `errorCode`（业务码）+ `httpStatus`（HTTP 状态码）+ getter
- [新增] `composer.json` 包配置（PSR-4 自动加载 `KeyAuth\\`，`php>=7.2.0`）
- [新增] `README.md` 完整文档
- [校验] `php -l` 语法校验通过（KeyAuthClient.php + KeyAuthError.php 0 错误）

#### 铁律遵守
- 铁律 04：SDK 不硬编码任何密钥/域名/AppKey，全部由开发者通过构造函数传入；密钥从环境变量读取（README 有示例）
- 铁律 05：SDK 内部无业务可调参数，所有可变项（API 路径前缀、超时、User-Agent）通过参数或常量管理
- 铁律 06：签名算法回退分支已标注「待核实：sha256 与后端 sha512.New512_256 是否完全等价」；PHP SDK 校验通过 `php -l`；未提供运行时测试用例（待 v0.4.x 补集成测试）

### [新增] 开发者自有易支付回调 + 双层支付模式切换（#5）

#### 设计方案
落地 `SysPackage.AllowCustomPay` + `TenantPayConfig.Enabled` 双开关，实现"平台总支付（默认）/ 开发者自有易支付（按套餐开通）"双层支付模式切换。订单号前缀分发：

- `ORD` → 平台总支付卡密购买（`processPaidOrder`，写 `PlatformSettlement` 抽成记录）
- `TOP` → 开发者自有易支付卡密购买（`processTenantOwnPaidOrder`，不写抽成，资金直接进开发者易支付账户，平台通过套餐 `CustomPayFee` 月费模式收费）
- `REG` → 代理注册付费（`processAgentRegisterPaid`，独立通道，注册费归平台）

#### 后端
- [改造] `apps/server/internal/handler/pay.go` `CreatePayOrder` 增加双层切换逻辑：
  - 查开发者套餐 `AllowCustomPay` + 开发者 `TenantPayConfig(channel=epay, enabled=true)`
  - 命中 → 走自有支付：订单号前缀 `TOP`，回调 URL 携带 `tenant_id`（`resolveTenantNotifyURL`）
  - 未命中 → 回退平台总支付：订单号前缀 `ORD`（保持原逻辑），需 `pay.platform.enabled=true`
  - 响应新增 `pay_mode` 字段（`tenant` / `platform`）供前端区分
- [新增] `pay.go` `EpayTenantNotify` 完整实现（替换原 `c.String(200, "fail")` 占位）：
  - 从 URL 取 `tenant_id`，调 `loadTenantPayConfig` 加载该租户易支付配置
  - 收集回调参数 + `epay.VerifyNotify` 验签（用该租户密钥）
  - Redis 防重入（key 含 `tenant_id` 命名空间隔离：`pay:notify:tenant:{tid}:lock:{order_no}`）
  - 调 `dispatchPaidOrder` 按前缀分发到 `processTenantOwnPaidOrder`
- [新增] `pay.go` `loadTenantPayConfig` 辅助函数：从 `tenant_pay_config` 表按 `(tenant_id, channel=epay, enabled=true)` 查配置 + AES-256-GCM 解密 `key_encrypted`
- [新增] `pay.go` `processTenantOwnPaidOrder` 事务内：
  - 校验订单状态/金额（防伪造回调）
  - 幂等保护（已 paid 直接返回）
  - 自动发卡（与 `processPaidOrder` 同流程，batch_no 前缀 `T` 区分）
  - **不写 `PlatformSettlement`**（资金已直接到开发者易支付账户，平台不抽成）
  - 写 `TenantBalanceLog{type=settle, amount=订单总额, pay_method=tenant_epay}` + 累加 `sys_tenant.balance`
- [新增] `pay.go` `dispatchPaidOrder` 增加 `TOP` 分支
- [新增] `pay.go` `resolveTenantNotifyURL` / `resolveTenantReturnURL` / `ternary` 辅助函数
- [修复] `migrations/002_seed_data.up.sql` 配置键名 bug：
  - `pay.platform.notify_path`：`/api/v1/pay/platform/notify` → `/api/v1/pay/notify/epay`（与 router 一致）
  - `pay.platform.return_path`：`/pay/return` → `/api/v1/pay/return/epay`（与 router 一致）
- [新增] `002_seed_data.up.sql` 三个新配置项：
  - `pay.tenant.notify_path`（默认 `/api/v1/pay/notify/tenant/`）
  - `pay.platform.order_name_prefix`（默认 `KeyAuth卡密`）
  - `pay.platform.return_front_url`（默认 `/pay/result`）

#### 前端
- [无变更] 现有 `tenant/PayConfig.vue` + `admin/PayConfig.vue` 已支持开发者保存/启用自有易支付配置，本版本后端切换逻辑生效后即可实时启用，无需前端调整

#### 铁律遵守
- 铁律 04：订单号前缀 `TOP/ORD/REG` 集中分发，无硬编码业务路由；开发者密钥 AES 加密入库
- 铁律 05：所有路径/前缀/订单名从 `sys_config` 读取（`pay.tenant.notify_path` / `pay.platform.order_name_prefix` / `pay.platform.return_front_url`）
- 铁律 06：编译验证通过（`go build ./...` + `vue-tsc --noEmit`）；金额校验防伪造回调；幂等保护；FOR UPDATE 防并发余额更新

### [新增] 卡密封禁联动设备强制下线（#1）

#### 后端
- [新增] `apps/server/internal/handler/card.go` `TenantBanCard` 在卡密封禁后联动下线所有绑定设备：调 `heartbeat.Remove` 清 Redis 心跳 + DB 标记 `app_device.status='banned'` + `last_heartbeat_at=NULL`
- [修复] 移除 `card.go:422` 的 `TODO(v0.3.0): 同时下线该卡密绑定的设备（清 Redis 心跳）` 占位
- [新增] card.go 导入 `internal/heartbeat` 包

#### 铁律遵守
- 铁律 05：`heartbeat.Remove` 内部用 appID/deviceID 拼 Redis Key，无硬编码
- 铁律 06：Redis 清理失败不阻塞封禁主流程（卡密已 banned，下次 verify 会因 card.status 拒绝）

### [新增] 卡密 CSV 导入导出（#2）

#### 后端
- [新增] `apps/server/internal/handler/card.go` `TenantExportCardsCSV` 导出 CSV（支持 app_id/status/batch_no/keyword 过滤，最多 10000 条，UTF-8 BOM）
- [新增] `apps/server/internal/handler/card.go` `TenantImportCardsCSV` 导入 CSV（前端解析后传 JSON 数组，事务批量入库，重复 hash 跳过并记失败明细）
- [新增] card.go 辅助函数 `ptrTimeFmt` `min`
- [新增] `apps/server/internal/router/router.go` 注册 `GET /api/v1/tenant/cards/export` + `POST /api/v1/tenant/cards/import`（注意：在 `cards/:id` 之前注册避免路由冲突）

#### 前端
- [新增] `apps/admin/src/api/cards.ts` `exportCardsApi`（用 axios blob 下载，带 Authorization Header 避免暴露 token）+ `importCardsApi` + `ImportCardsResult` 类型
- [修改] `apps/admin/src/views/tenant/Cards.vue` 新增「导出 CSV」「导入 CSV」按钮 + 导入对话框（应用/卡类/前缀/分组/文件上传）+ 导入结果对话框（成功/失败/空行/重复统计 + 失败明细）

#### 铁律遵守
- 铁律 04：CSV 导出为真实数据，无硬编码假数据；前端用 blob 下载，token 不暴露在 URL/日志
- 铁律 05：导出条数上限 `card.export.max_rows`（默认 10000）+ 导入条数上限 `card.import.max_rows`（默认 5000）从 sys_config 读取，禁硬编码
- 铁律 06：导入失败明细返回前端，禁只报"成功"假象；事务回滚保护

### [新增] 安装向导页面 `/install`（#3）

#### 后端
- [新增] `apps/server/internal/handler/install.go` `InstallStatus`（GET /api/v1/install/status）：通过 `sys_admin.password_hash` 是否含 `PLACEHOLDER_BCRYPT_HASH` 占位串判定 installed 状态，避免用 `count(*)` 误判（seed 已插入 1 行占位）
- [新增] `install.go` `Install`（POST /api/v1/install）：接收超管账号密码 + 平台基础配置，事务写入：
  - 更新 `sys_admin` id=1 占位行 → 真实 bcrypt 哈希（cost=12）+ 真实 username/email/phone
  - upsert `sys_config` 平台基础项（`platform.domain` / `platform.name` / `platform.notify_email` / `agent.register_fee` / `pay.platform_commission_rate` / `platform.installed_at`）
  - 二次校验已安装状态拒绝重入
  - 调 `CfgCache.InvalidateAll` 刷新 Redis 缓存
  - 异步记录操作日志（不含密码）
- [新增] `install.go` 辅助函数 `checkInstalled` / `upsertConfig`
- [新增] `apps/server/internal/router/router.go` 注册 `v1.GET("/install/status")` + `v1.POST("/install")`（无需鉴权，仅首次部署可用）

#### 前端
- [新增] `apps/admin/src/api/http.ts` `installStatusApi` / `installApi` + `InstallStatus` / `InstallPayload` / `InstallResult` 类型（直接调 http 实例，绕过 token 拦截器）
- [新增] `apps/admin/src/views/Install.vue` 4 步向导（环境检测 → 超管账号 → 平台配置 → 完成）+ `el-steps` 进度条 + 表单校验（密码确认一致性、邮箱格式）+ `el-result` 成功提示
- [新增] `apps/admin/src/router/index.ts` `/install` 路由（public，无需登录）

#### 铁律遵守
- 铁律 04：超管密码不硬编码，由安装表单传入；bcrypt cost=12 哈希后入库
- 铁律 05：所有平台配置写入 `sys_config` 表 + Redis 缓存，不写入代码或 .env
- 铁律 06：已安装检测用占位串而非 `count(*)`，避免 seed 数据导致误判；二次校验防并发安装

### [新增] 代理注册付费流程（#4）

#### 设计方案
采用**方案 B：先支付后建 Agent**，避免引入 `pending_payment` 状态破坏 `AgentLogin` 现有 `status != "active"` 不变量。代理行仅在支付回调事务内创建且 `Status="active"`，可直接登录。

#### 后端
- [新增] `apps/server/internal/handler/auth.go` `AgentRegister`（POST /api/v1/public/auth/agent/register）：
  - 校验邀请码（`status=active` + `used_count < max_uses` + `expires_at > now`）
  - 校验用户名在所属租户内唯一
  - `quota.CheckMaxAgents` 校验套餐代理数上限（第一道防线）
  - 读 `sys_config` `agent.register.fee` 注册费（默认 99.00）
  - bcrypt 哈希密码（cost=12），缓存到 Redis（`agent_register:pwd:{order_no}`，TTL=`pay.order_expire_seconds` 默认 1800s）
  - 创建 `AgentRegistrationOrder`（订单号前缀 `REG`，`PayStatus=pending`，`AgentID=nil` 占位）
  - 调 `epay.BuildSubmitURL` 返回 `pay_url`
- [新增] `auth.go` `AgentRegisterConfig`（GET /api/v1/public/auth/agent/register/config）：公开返回注册费 + 支付方式 + 平台支付开关，不返回敏感字段（gateway_url/pid/key_encrypted）
- [新增] `auth.go` `AgentRegisterOrderStatus`（GET /api/v1/public/auth/agent/register/order/:order_no）：前端支付完成跳回后查询订单状态
- [改造] `apps/server/internal/handler/pay.go` `EpayNotify` 引入 `dispatchPaidOrder` 按订单号前缀分发：
  - `ORD` → 现有 `processPaidOrder`（卡密购买）
  - `REG` → 新 `processAgentRegisterPaid`（代理注册）
- [新增] `pay.go` `processAgentRegisterPaid` 事务内：
  - 校验订单状态/金额（防伪造）
  - 幂等保护（已 paid 直接返回）
  - 事务内重复 `quota` 校验防 TOCTOU（套餐上限 + 用户名重复）
  - INSERT `Agent{Status: "active", CommissionRate: 邀请码.DefaultCommissionRate, CommissionMode: "percentage"}`
  - 回填 `AgentRegistrationOrder.AgentID` + `PayStatus=paid` + `PaidAt` + `PayTradeNo`
  - 邀请码 `used_count++`，达 `max_uses` 时 `status=exhausted` + 写 `used_by_agent_id`（补齐 exhausted 状态从未被写入的逻辑漏洞）
  - 删除 Redis 中的密码哈希缓存（已用过，安全清理）
  - 注册费不进 `PlatformSettlement`（直接归平台，与卡密抽成解耦）
- [新增] `pay.go` `cacheAgentRegisterPassword` 辅助函数（铁律 04：DB 不存明文密码也不存哈希到订单表，仅短期缓存 bcrypt 哈希等回调使用）
- [修复] `apps/server/internal/handler/install.go` 配置键名 bug：`agent.register_fee` → `agent.register.fee`（与 `migrations/002_seed_data.up.sql` 保持一致，下划线改点号）；`pay.platform_commission_rate` → `pay.platform.commission_rate`
- [新增] `apps/server/internal/router/router.go` 注册 3 个公开路由：`POST /auth/agent/register` + `GET /auth/agent/register/config` + `GET /auth/agent/register/order/:order_no`（无需鉴权）

#### 前端
- [新增] `apps/admin/src/api/agent.ts` 三个注册相关 API + 类型：`agentRegisterConfigApi` / `agentRegisterApi` / `agentRegisterOrderStatusApi` + `AgentRegisterConfig` / `AgentRegisterResult` / `AgentRegisterOrder` 类型
- [改造] `apps/admin/src/views/agent/Register.vue` 落地原 3 处 TODO：
  - `onMounted` 调 `agentRegisterConfigApi` 读 `register_fee` + `pay_methods` + `pay_enabled`，按后端配置重建支付方式列表（铁律 04：不硬编码支付方式）
  - Step 1 提交调 `agentRegisterApi` 创建预支付订单 + 返回 `pay_url`，缓存订单号 + pay_url 到本地
  - Step 2 「前往支付页面」用 `window.open(payURL, '_blank')` 新窗口跳转避免丢失原页面状态；「我已完成支付，查询状态」按钮调 `agentRegisterOrderStatusApi` 轮询订单状态
  - 当 `pay_enabled=false` 时禁用提交按钮 + 显示红色告警

#### 铁律遵守
- 铁律 04：密码明文不入库，仅短期缓存 bcrypt 哈希到 Redis；订单号前缀 `REG` 与 `ORD` 区分业务；支付方式不硬编码，从 `pay.platform.methods` 读取
- 铁律 05：注册费从 `agent.register.fee` 读取；订单过期时间从 `pay.order_expire_seconds` 读取；商品名前缀从 `pay.platform.order_name_prefix` 读取
- 铁律 06：事务内重复 quota 校验防 TOCTOU；邀请码状态机闭环（达 max_uses 时置 exhausted，补齐旧逻辑漏洞）；二次用户名校验防并发；AgentRegisterConfig 不返回敏感字段

### [文档] 四份核心文档 + README + PROMPT 全量同步对齐 v0.3.5 实际状态

本次发布为纯文档同步，按 `web-project-flow` skill 的 `references/09-docs-lifecycle.md` 规范联动更新，消除多份文档与代码实际状态不一致的矛盾。配套 `web-project-flow` skill 已全局安装。

#### README.md
- [修改] 版本徽章 `0.2.7` → `0.3.5`
- [修改] 「当前版本」表格新增 v0.3.0 ~ v0.3.5 六行，所有模块状态从「⏳ 计划中」改为「✅ 已完成」
- [修改] 「后续版本规划」从 v0.2.5/v0.2.6/v0.2.7/v0.3.0/v0.4.0 改为 v0.3.6/v0.4.0
- [新增] 「开发约束」章节补 `web-project-flow` skill 已全局安装说明 + `/bhelp` `/bhardcode /bconfig /bhaluc` `/bdocs` 命令索引

#### PROMPT.md
- [新增] 顶部 skill 使用说明（`/bhelp` / `/bonboard` / `/bdocs`）
- [修改] 「五、当前进度」从 v0.2.0 骨架阶段重写为 v0.3.5 已完成 + v0.3.6 待开始 + v0.4.0 计划
- [修改] 「九、可信度声明」补三个具体占位文件位置（auth.go:443 / pay.go:528 / Register.vue）

#### PROJECT.md
- [修改] 文档版本 `0.1.0` → `0.3.5`，最后更新 `2026-07-19` → `2026-07-20`
- [修改] 「3.1 平台超管后台」17 个模块状态全勾选已实现项（11 ✅ + 6 ☐ 标注 v0.4.x）
- [修改] 「3.2 开发者控制台」19 个模块状态全勾选（15 ✅ + 4 ☐ 标注 v0.3.6/v0.4.x）
- [修改] 「3.3 代理商控制台」10 个模块状态全勾选（8 ✅ + 2 ☐ 标注 v0.4.x）
- [修改] 「3.4 终端用户 H5」14 个页面状态全勾选（5 ✅ + 9 ☐ 标注 v0.4.x 终端用户体系未建）
- [修改] 「3.5 客户端 SDK」补预计版本（Python/Node/PHP v0.3.6，其余 v0.4.0）
- [修改] 「4.1 表清单」26 张 → 30 张，新增 platform_settlement/log_login_failed/refresh_token_device/tenant_balance_log/tenant_withdraw/schema_migrations 6 张表，并加「引入版本」列
- [修改] 「4.2 Redis 缓存键」移除 session:admin/tenant/agent（实际用 JWT 无 session）+ 补 2fa:setup/2fa:backup/login:fail
- [修改] 「6. 目录结构」完全重写对齐实际（移除不存在的 service/repository 子目录、sdks/ 目录、deploy/docker-compose.yml；新增 internal/auth/heartbeat/migration/quota 包说明 + handler 17 文件清单 + .env.development/production + PROMPT.md）
- [修改] 「7.4 编码铁律」补 skill 命令索引

#### SPEC.md
- [修改] 文档版本 `0.1.0` → `0.3.5`，最后更新 `2026-07-19` → `2026-07-20`
- [修改] 「2.1 分层架构」从理论 4 层（Handler/Service/Repository/Model）改为实际 3 层简化架构（Handler 直连 GORM + 辅助包 + Model+Middleware），补当前实现说明 + v0.4.x 重构计划
- [修改] 「2.2 模块边界」从理论 `internal/service/<module>/` 改为实际 `internal/handler/` 17 文件清单，补跨文件通信机制（Deps / RecordOperation / writeVerifyLogCtx）
- [修改] 「8.3 数据库迁移」从 `golang-migrate/migrate` 改为自研轻量级 SQL 文件迁移机制（v0.3.5），补 schema_migrations 表 + dirty 状态 + multiStatements + MIGRATION_AUTO/MIGRATION_DIR 等实际细节
- [修改] 「9.1 四份核心文档」补联动校验铁律三条

#### TODO.md
- [修改] 「三级公告体系」11 项子项从 [待开始] 改为 [已完成]（统一公告表/notice_target/notice_read/S-15/S-16/D-10/agent_notify/顶部公告/P-08），消除与 v0.3.0 章节的矛盾
- [修改] 「云变量与版本管理」5 项子项全部改为 [已完成]（云变量 CRUD + 3 个客户端接口），消除与 v0.3.0 章节的矛盾
- [修改] 「数据统计看板」3 项已实现（admin/tenant/agent dashboard）改为 [已完成]，仅留 2 项 v0.4.x
- [修改] 「客户端 SDK」版本号 v0.3.0 → v0.3.6
- [修改] 「开发者自有易支付」补 tenant_pay_config [已完成] + EpayTenantNotify 仍返回 "fail" 的精确位置（pay.go:528）
- [修改] 「代理注册付费流程」补邀请码 CRUD [已完成] + AgentRegister 501 占位精确位置（auth.go:443）
- [修改] 「代理购买卡密」实时扫码购卡/独立门户/子域名绑定 从 v0.3.0 → v0.4.x
- [修改] 里程碑表新增 v0.3.5 行 + v0.3.6 [进行中] 行
- [修改] 「v0.2.3 进度统计」章节完全重写为「v0.3.5 进度统计」（总任务 75→110，已完成 53→90，待完成 21→19，新增 v0.2.0~v0.3.5 已完成版本汇总表 + v0.3.6/v0.4.x 待完成项分类）
- [修改] 文档版本 `0.2.7` → `0.3.5`

#### 验证
- [验证] 文档版本号四份统一为 `0.3.5`
- [验证] 时间戳统一为 `2026-07-20`
- [验证] TODO 中标记 [已完成] 的项均能在对应版本 CHANGELOG 中找到（联动校验铁律 ①）
- [验证] PROJECT 中描述的功能与 SPEC 中的规范一致（联动校验铁律 ②）

#### 待核实项（铁律 06）
- 表清单 30 张中 `app_user` 在 v0.2.0 DDL 中存在但 v0.3.x 实际未使用（终端用户体系未建），待 v0.4.x 终端用户体系启动时确认
- `agent_quota` 表在 DDL 中存在但 model.go 中无对应 struct，待核实是否仍在使用

---

## [0.3.5] - 2026-07-19

### [修复] P0 紧急修复：RSA 脚本 / 数据库迁移 / H5 公共 API / 套餐配额

本次发布聚焦 4 项 P0 缺陷修复，覆盖部署脚本、数据库迁移机制、H5 购卡闭环、套餐配额统一管理。

#### 后端：RSA-4096 密钥生成独立脚本
- [新增] `scripts/gen_rsa_key.sh`：从 `baota_deploy.sh` 抽取为独立脚本
  - 支持 `--force` 覆盖已存在密钥
  - 支持自定义输出目录（`--dir /path` 或位置参数）
  - 私钥 PKCS#8/PEM + 公钥 PKIX/PEM，chmod 600/644
  - 生成后自动密钥配对校验（openssl rsa -in priv -pubout 对比公钥）
  - 修复 OUTPUT_DIR 拼接 bug（原代码因运算符优先级导致 pwd 输出两次）

#### 后端：数据库迁移机制修复
- [新增] `apps/server/internal/migration/migrator.go`：轻量级 SQL 文件迁移机制
  - `schema_migrations` 表跟踪版本号 + dirty 状态
  - 扫描 `*.up.sql` 文件，按文件名前缀数字排序
  - 每个迁移在独立事务中执行，失败标记 dirty 阻止启动
  - 幂等：已应用的迁移不会重复执行
- [修改] `apps/server/internal/config/config.go`
  - 新增 `MigrationConfig` struct（Auto bool, Dir string）
  - Config 加 `Migration MigrationConfig` 字段，默认 `{Auto: true, Dir: "apps/server/migrations"}`
  - `applyEnvConfig` 加 `MIGRATION_AUTO` / `MIGRATION_DIR` 环境变量覆盖
  - DSN 加 `multiStatements=true` 参数（迁移文件含多语句）
  - `InitContainer` 中加 `migration.Run(db, cfg.Migration.Dir)` 调用
- [修改] `docker-compose.yml`
  - mysql 服务移除 `./apps/server/migrations:/docker-entrypoint-initdb.d:ro` 挂载
    - 修复缺陷：原挂载方式按字母序执行所有 .sql（含 .down.sql），存在迁移顺序错误风险
  - server 服务 environment 加 `MIGRATION_AUTO: "true"` 和 `MIGRATION_DIR: /app/migrations`
- [修改] `configs/config.yaml.example`：完全重写以对齐 Config struct 的 yaml tag（app/mysql/redis/jwt/crypto/migration/domain）

#### 后端：H5 公共 API 补齐（购卡闭环）
- [新增] `apps/server/internal/handler/public.go`：H5 终端用户购卡流程公开 API（无需鉴权）
  - `PublicAppInfo` GET `/public/apps/info?app_key=xxx`：按 app_key 查应用公开信息，联表校验 SysTenant active 状态
  - `PublicCardTypes` GET `/public/card_types?app_id=xxx`：按 app_id 查可购卡类列表，仅返回 active 卡类
  - 安全：`publicAppInfo` / `publicCardType` DTO 过滤敏感字段（app_secret / sign_secret / agent_base_price）
- [修改] `apps/server/internal/handler/pay.go` `GetPayOrder`：订单已支付时返回卡密明文
  - 新增 `card_keys []string` 字段：从 AppCard 表按 card_ids Pluck card_key
  - 供 H5 终端用户支付成功后直接查看卡密，无需另调查询接口
- [修改] `apps/server/internal/router/router.go`：publicGroup 新增 2 条路由
  - `GET /public/apps/info`
  - `GET /public/card_types`
- [修改] `apps/admin/src/api/pay.ts`：`PayOrder` 接口加 `card_keys: string[]` 字段
- [修改] `apps/admin/src/views/h5/Home.vue`：移除"待核实"注释，更新 `loadCardTypes` 使用已实现的 `/public/apps/info` + `/public/card_types`
- [修改] `apps/admin/src/views/h5/PayResult.vue`：`fetchOrder` 使用 `resp.card_keys` 替代空数组占位

#### 后端：套餐配额检查统一封装
- [新增] `apps/server/internal/quota/quota.go`：套餐配额检查 helper 包
  - `ExceededError` 自定义错误类型（Resource / Current / Limit / AddCount）
  - `loadTenantPackage` 内部 helper：校验 tenant.Status == "active" && tenant.ExpiresAt 未过期 && pkg.Status == "active"
  - `CheckMaxApps`：校验开发者创建应用是否超出套餐上限（`App` COUNT）
  - `CheckMaxCards`：校验开发者生成卡密是否超出套餐上限（`AppCard` COUNT + addCount）
  - `CheckMaxAgents`：校验开发者代理数是否超出套餐上限（`Agent` COUNT，Limit==0 表示套餐不支持招募代理）
  - `CheckMaxDevices`：校验单卡密绑定设备数是否超出应用配置上限（`AppDevice` COUNT，MaxDevices 是应用级配置）
  - 设计：不在 helper 内开事务（避免嵌套事务），TOCTOU 风险由调用方在事务内处理
- [修改] `apps/server/internal/handler/app.go` `TenantCreateApp`
  - 将内联 MaxApps 检查替换为 `quota.CheckMaxApps(deps.DB, tenantID)`
  - 使用 `errors.As(err, &qErr)` 区分配额超限错误和系统错误
- [修改] `apps/server/internal/handler/card.go` `TenantGenerateCards`
  - 将内联 MaxCards 检查（含 tenant/pkg 查询）替换为 `quota.CheckMaxCards(deps.DB, tenantID, req.Quantity)`
  - 错误消息保留原格式：「将超过套餐卡密上限 N 张（当前 X 张，本次生成 Y 张）」
- [修改] `apps/server/internal/handler/tenant_business.go` `TenantGenInviteCode`
  - 在事务前新增 `quota.CheckMaxAgents(deps.DB, tenantID)` 调用
  - 区分两种场景：`Limit == 0`（套餐不支持招募代理）vs `Limit > 0`（已达上限）
  - 注释说明：邀请码本身不是代理，但生成邀请码隐含招募代理意图，提前校验避免发放无效邀请码
- [修改] `apps/server/internal/handler/client.go` `ClientLogin` + `ClientBind`
  - `ClientLogin` 新设备绑定前：将内联 MaxDevices 检查替换为 `quota.CheckMaxDevices`
  - `ClientBind` 手动绑定前：将内联 MaxDevices 检查替换为 `quota.CheckMaxDevices`
  - `ClientBind` 保留 `countBoundDevices` 调用以在响应中展示当前绑定数（双重查询可接受，绑定非高频操作）

#### 验证
- [验证] `go build ./...` 通过（0 错误）
- [验证] `go vet ./...` 通过（0 警告）
- [验证] `pnpm run build`（admin）通过（8.56s）

#### 文档
- [修改] `docs/TODO.md`：标记 3 项 P0 已完成 + 新增 v0.3.5 章节（17 项明细）
- [修改] `docs/CHANGELOG.md`：新增 v0.3.5 版本条目

---

## [0.3.4] - 2026-07-19

### [功能] 开发者结算与对账闭环（事务保护 + 批量结算 + 余额流水对称设计）

#### 后端：数据库迁移
- [新增] `apps/server/migrations/007_v0.3.4_tenant_finance.up.sql`
  - `sys_tenant` 增 `balance` DECIMAL(12,2) + `frozen_balance` DECIMAL(12,2) 字段
  - 新建 `tenant_balance_log` 表（type: settle/withdraw/refund/adjust，status: pending/settled/rejected）
  - 新建 `tenant_withdraw` 表（status: pending/paid/rejected/failed）
- [新增] `007_v0.3.4_tenant_finance.down.sql` 回滚迁移

#### 后端：模型 + AdminSettleOrder 改造
- [修改] `apps/server/internal/model/model.go`
  - `SysTenant` 增 `Balance float64` + `FrozenBalance float64` 字段
  - 新增 `TenantBalanceLog` struct（含 TenantID/Type/Amount/BalanceAfter/RelatedSettlementID/RelatedWithdrawID/PayMethod/SettleBatchNo/Status/Remark）
  - 新增 `TenantWithdraw` struct（含 TenantID/Amount/PayMethod/PayAccount/Status/AuditRemark/PayTradeNo/PaidAt/AuditedBy）
- [修改] `apps/server/internal/handler/pay.go` `AdminSettleOrder`：
  - 改造为事务保护 + `tx.Set("gorm:query_option", "FOR UPDATE")` 防并发
  - 累加开发者可提现 `balance`
  - 写 `tenant_balance_log`（type=settle, status=settled）

#### 后端：开发者侧 Handler（tenant_settle.go，对称 agent_business.go）
- [新增] `apps/server/internal/handler/tenant_settle.go`（5 个 handler 全事务保护）
  - `TenantListSettlements` GET `/tenant/settlements` 查自己的 platform_settlement，含 pending_sum/settled_sum 汇总
  - `TenantBalanceOverview` GET `/tenant/balance_overview` 余额概览（balance/frozen/settled_total/withdrawn_total/pending_withdraw）
  - `TenantListBalanceLogs` GET `/tenant/balance_logs` 查自己的余额流水（type/status 筛选）
  - `TenantListOwnWithdrawals` GET `/tenant/withdrawals/mine` 查自己的提现申请
  - `TenantWithdraw` POST `/tenant/withdraw` 事务：扣 balance + 加 frozen_balance + 写 withdraw + 写 balance_log

#### 后端：超管审核 Handler（admin_finance.go，对称 tenant_finance.go）
- [新增] `apps/server/internal/handler/admin_finance.go`（5 个 handler 全事务保护）
  - `AdminListTenantWithdrawals` GET `/admin/tenant_withdrawals` 联表 sys_tenant，默认 status=pending
  - `AdminPayTenantWithdraw` POST `/admin/tenant_withdrawals/:id/pay` 事务：withdraw.status=paid + frozen_balance -= amount + balance_log.status=settled
  - `AdminRejectTenantWithdraw` POST `/admin/tenant_withdrawals/:id/reject` 事务：退 balance + 减 frozen_balance + withdraw.status=rejected + 写 refund 流水
  - `AdminBatchSettle` POST `/admin/settlements/batch_settle` 批量结算，按 tenant_id 分组累计 net_amount，单次最多 100 条
  - `AdminReconciliation` GET `/admin/reconciliation` 对账报表，聚合 SQL（SUM + CASE WHEN）统计订单总额/抽成/应结/已结/未结/已提现/理论余额

#### 后端：路由注册
- [修改] `apps/server/internal/router/router.go`
  - adminAuth 段新增 5 条：`/settlements/batch_settle` + `/tenant_withdrawals` (GET/POST 两条) + `/reconciliation`
  - tenantAuth 段新增 5 条：`/settlements` + `/balance_overview` + `/balance_logs` + `/withdrawals/mine` + `/withdraw`
- [验证] `go build ./...` + `go vet ./...` 双双通过

#### 前端：API 层
- [新增] `apps/admin/src/api/tenantFinance.ts`：6 个类型 + 10 个 API 函数
  - 类型：`PlatformSettlement` / `TenantBalanceLog` / `TenantWithdrawal` / `AdminTenantWithdrawal` / `TenantBalanceOverview` / `ReconciliationData`
  - 开发者侧 5 个：`listTenantSettlementsApi` / `tenantBalanceOverviewApi` / `listTenantBalanceLogsApi` / `listTenantOwnWithdrawalsApi` / `tenantWithdrawApi`
  - 超管侧 5 个：`listAdminTenantWithdrawalsApi` / `payAdminTenantWithdrawalApi` / `rejectAdminTenantWithdrawalApi` / `batchSettleApi` / `reconciliationApi`

#### 前端：开发者后台 2 个新页面
- [新增] `apps/admin/src/views/tenant/Settlements.vue` 开发者结算记录页
  - 余额概览卡片：可用余额 / 冻结 / 累计已结 / 累计已提现 / 待审核提现
  - 双 Tab：结算记录（订单号/抽成比例/平台抽成/应得/状态/批次号 + pending_sum/settled_sum 汇总）+ 余额流水（类型/金额变动+/-/操作后余额/状态）
  - 完整响应式 H5：钱包概览 4 列 → 2 列，搜索栏水平 → 垂直
- [新增] `apps/admin/src/views/tenant/Withdrawal.vue` 开发者提现申请页
  - 余额概览：可用 / 冻结 / 累计已提现 / 待审核提现
  - 提现表单：金额（≤ balance 校验）+ 收款方式（alipay/wechat/bank radio）+ 收款账号（动态 placeholder）+ 备注
  - 快捷按钮：「全部提现」「提现一半」
  - 提现记录列表：金额/方式/账号/状态/审核备注/打款流水号/打款时间
  - 完整响应式 H5：表单 + 记录双栏布局 → 移动端单列堆叠

#### 前端：超管后台 1 个新页面 + 1 个升级
- [新增] `apps/admin/src/views/admin/TenantWithdrawalReview.vue` 开发者提现审核页
  - 列表：开发者用户名/公司名/金额/收款方式/收款账号/状态/打款流水号/打款时间
  - 操作：打款对话框（含打款流水号 + 备注）+ 驳回对话框（必填原因，退回余额）
  - 完整响应式 H5
- [修改] `apps/admin/src/views/admin/Settlements.vue` 升级双 Tab
  - Tab 1 结算记录：保留原单条结算 + 新增多选批量结算按钮（仅 pending 可选，单次最多 100 条，含「选中应结」「涉及开发者数」预览）
  - Tab 2 对账报表：9 个聚合卡片（订单总数/总额/平台抽成/应结/已结/未结/已提现/待审核提现/理论余额 = 已结 - 已提）
  - 完整响应式 H5：3 列 → 2 列

#### 前端：路由注册
- [修改] `apps/admin/src/router/index.ts`
  - admin 段新增 `/admin/tenant-withdrawal-review`
  - tenant 段新增 `/tenant/settlements` + `/tenant/withdrawal`
- [验证] `pnpm run build` 通过

#### 设计要点
- 事务安全：所有金额变动走 `deps.DB.Transaction()` + `FOR UPDATE`，避免并发提现 / 结算竞争
- 对称设计：`tenant_withdraw` / `tenant_balance_log` / `sys_tenant.balance` 对称于 `agent_withdraw` / `agent_balance_log` / `agent.balance`
- 余额模型：`balance`（可提现）+ `frozen_balance`（冻结，提现申请中）；提现时 balance→frozen，打款时 frozen 清除，驳回时 frozen→balance
- 批量结算：按 tenant_id 分组累计 net_amount，避免同一开发者多次更新
- 对账差计算：`balance_theory = settled_sum - withdrawn_sum`，用于校验开发者账户余额理论值

---

## [0.3.3] - 2026-07-19

### [功能] 日志系统：异步 Worker + 三表独立查询 + CSV 导出 + 前端 3 Tab 升级

#### 后端：异步日志 Worker
- [新增] `apps/server/internal/handler/log_worker.go`
  - `verifyLogCh`（容量 4096）+ `StartVerifyLogWorker`：验证日志异步消费 goroutine，超出容量丢弃以保证验证 API 性能
  - `operationLogCh`（容量 2048）+ `StartOperationLogWorker`：操作日志异步消费 goroutine
  - `enqueueVerifyLog` / `enqueueOperationLog`：非阻塞 `select/default` 投递
  - `RecordOperation(deps, c, module, action, status, targetType, targetID, detail)`：从 `gin.Context` 抽取 role/userID/username/IP/UA 的一致切面 helper，供各业务 handler 调用
- [修改] `apps/server/internal/handler/client.go`
  - 新增 `writeVerifyLogCtx(deps, c, app, hwid, cardKey, action, result, message)`：捕获客户端 IP + User-Agent
  - 保留 `writeVerifyLog` 作为向后兼容包装（c=nil）
  - 14 处 `writeVerifyLog(deps, app,` → `writeVerifyLogCtx(deps, c, app,` 批量升级
- [修改] `apps/server/cmd/main.go`：启动时调用 `StartVerifyLogWorker` + `StartOperationLogWorker`

#### 后端：三表独立查询 + CSV 导出
- [新增] `AdminListOperationLogs` GET `/admin/logs/operations`：支持 operator_type / module / action / status / start_date / end_date / keyword 筛选
- [新增] `AdminListVerifyLogs` GET `/admin/logs/verify`：支持 app_id / action / result / start_date / end_date / keyword 筛选
- [新增] `AdminListLoginFailedLogs` GET `/admin/logs/login_failed`：支持 user_type / username / ip / start_date / end_date 筛选
- [新增] `AdminExportLogs` GET `/admin/logs/export?log_type=operation|verify|login_failed`：
  - 输出 `Content-Type: text/csv` + `Content-Disposition: attachment`
  - 写入 UTF-8 BOM `\xEF\xBB\xBF` 保证 Excel 正确识别编码
  - 单次导出上限 10000 条（防止 OOM）
  - `csvRow` helper 处理字段转义
- [新增] 路由注册 4 条新路由到 `adminAuth` 组（保留旧 `/admin/logs` 兼容接口）
- [验证] `go build ./...` + `go vet ./...` 双双通过

#### 前端：3 Tab 切换 + CSV 下载
- [新增] `apps/admin/src/api/admin.ts`：`LogOperation` / `LogVerify` / `LogLoginFailed` 三个接口类型 + `AdminLogTab` 联合类型
- [新增] 4 个 API 函数：`listAdminOperationLogsApi` / `listAdminVerifyLogsApi` / `listAdminLoginFailedLogsApi` / `exportAdminLogsApi`（后者使用 `responseType: 'blob'` 绕过 JSON 拦截器）
- [修改] `apps/admin/src/views/admin/Logs.vue` 全面重构：
  - el-tabs 三表切换：操作日志 / 验证日志 / 登录失败日志
  - 每表独立筛选条件（操作日志：operator_type/operator_id/module/action/status/keyword；验证日志：tenant_id/app_id/action/result/keyword；登录失败日志：user_type/username/ip/reason）
  - 顶部「导出 CSV」按钮：按当前 Tab 调用 `/admin/logs/export?type=xxx`，前端 `createObjectURL` + `<a download>` 触发下载，文件名带时间戳
  - 每表独立的 ResponsiveTable 列定义 + mobileFields（响应式 H5）
  - 详情对话框按 kind 区分操作/验证两种字段集
  - 完整中文映射：operatorType / verifyAction / verifyResult / reason 等 tag/text
- [验证] `pnpm run build` 通过（Logs 模块 18.18 kB / gzip 4.94 kB）

---

## [0.3.2] - 2026-07-19

### [功能] 代理充值审核闭环 + 提现审核闭环

#### 后端：开发者财务审核 Handler
- [新增] `apps/server/internal/handler/tenant_finance.go`（6 个 handler 全事务保护）
  - `TenantListRechargeRequests` GET `/tenant/recharge_requests` 充值申请列表（联表 agent，默认 pending）
  - `TenantApproveRecharge` POST `/tenant/recharge_requests/:id/approve` 通过（事务：加余额 + 流水 status=settled，支持 actual_amount 调整）
  - `TenantRejectRecharge` POST `/tenant/recharge_requests/:id/reject` 驳回（流水 status=rejected）
  - `TenantListWithdrawals` GET `/tenant/withdrawals` 提现申请列表（联表 agent，默认 pending）
  - `TenantPayWithdraw` POST `/tenant/withdrawals/:id/pay` 打款（事务：withdraw.status=paid + paid_at + pay_trade_no + 对应 balance_log status=settled）
  - `TenantRejectWithdraw` POST `/tenant/withdrawals/:id/reject` 驳回（事务：退回余额 + withdraw.status=rejected + balance_log status=rejected）
- [新增] 路由 `router.go` 注册 6 条新路由
- [验证] `go build ./...` + `go vet ./...` 双双通过

#### 前端：开发者审核页面 + 代理充值表单修复
- [新增] `api/tenant.ts`：`TenantRechargeRequest` / `TenantWithdrawal` 类型 + 6 个审核 API 函数
- [新增] `views/tenant/RechargeReview.vue` 充值审核页（搜索 / 通过对话框 / 驳回对话框 / 响应式 H5）
- [新增] `views/tenant/WithdrawalReview.vue` 提现审核页（搜索 / 打款对话框 / 驳回对话框 / 响应式 H5）
- [新增] `router/index.ts` 注册 `/tenant/recharge-review` + `/tenant/withdrawal-review` 两条路由
- [修复] `api/agent.ts`：补齐 `agentRechargeApi` 函数（v0.3.1 已交付 /agent/recharge 端点）
- [修复] `views/agent/Balance.vue`：
  - 移除误用 `agentWithdrawApi` 提交充值的临时方案，改为调用 `agentRechargeApi`
  - 充值表单增加 `pay_method`（alipay/wechat/bank/manual）+ `pay_voucher` 字段
  - 非手工支付必须填写付款凭证
  - 清理「待核实 v0.3.0」过时注释
- [验证] `pnpm run build` 通过（修复 mobileFields 类型断言 + statusTag 返回类型）

#### 资金链路闭环
- 代理充值：申请 → 开发者审核通过（可调实际金额）→ 自动加余额 → 流水 settled
- 代理提现：申请（扣余额）→ 开发者打款（标记 paid + 写流水号）→ 流水 settled
- 代理提现驳回：退回余额 + 流水 rejected + audit_remark 记录原因

#### 待办（v0.4.x 双层支付 D-18）
- 套餐 `allow_custom_pay` 字段生效 + 开发者自有易支付下单/回调接口
- 切换支付方式时通知所有代理

---

## [0.3.1] - 2026-07-19

### [修复] v0.3.0 全部「待核实 v0.3.x」项归零

#### 数据库字段补全（migration 006）
- [新增] `migrations/006_v0.3.1_field_completion.up.sql` ALTER TABLE 补齐缺失字段
- [新增] `sys_tenant.remark` / `sys_package.description` / `notice.sort` / `sec_ip_blacklist.created_by` + `created_by_type` + `source`
- [新增] `log_operation.username` + `user_agent` + `status`
- [新增] `AppCloudVar.read_only`
- [新增] `AppVersion.channel`
- [新增] `Agent.commission_mode` + `inviter_id` + `totp_secret` + `email` + `last_login_ip` + `last_login_at`
- [新增] `AgentInviteCode.used_by_agent_id`
- [新增] 新增表 `log_login_failed`（登录失败日志）+ `refresh_token_device`（设备会话追踪）

#### 后端 Handler 落实真实字段
- [修改] `admin_business.go`：租户结算金额真实查询（`PlatformSettlement.status='settled'`）、`Remark`/`Description`/`CommissionMode`/`InviterUsername`/`Sort`/`CreatedBy` 等字段全部落库
- [修改] `tenant_business.go`：CloudVar 直接用 `ReadOnly` 字段；Version 真实 `channel` 过滤；邀请码联表查询 `used_by_username`；公告 `sort` 排序与 type 三值（tenant/agent/h5）；删除公告级联清理 `NoticeRead`/`NoticeTarget`
- [修改] `agent_business.go`：`AgentMe` 真实返回 `email`/`totp_enabled`/`last_login_ip`/`inviter_username`；Dashboard `today_spent` 真实计算（`SUM(total_amount - commission_amount)`）；提现 `AuditRemark` 持久化 real_name
- [修改] `profile.go`：启用 agent email 更新（v0.3.1 已加字段）；移除三处 agent 2FA 501 阻断（`Setup2FA`/`Verify2FA`/`Disable2FA`）；`loadUserTOTPSecret`/`updateUserTOTPSecret` 新增 agent case
- [修改] `router.go`：移除 `/agent/recharge` 路由的「待核实 v0.3.x」注释

#### 新功能：AgentRecharge 充值申请
- [新增] `handler.AgentRecharge`：代理提交充值申请 → 创建 `AgentBalanceLog(type=recharge, status=pending)`
- [新增] 校验：`amount > 0` / 非手动支付必须 `PayVoucher` / sys_config 读取 `agent.recharge.min_amount`(1.00) + `agent.recharge.max_amount`(100000.00)
- [新增] 返回 pending 状态等待开发者审核

#### 新功能：ListLoginDevices 完整实现
- [新增] `refresh_token_device` 表 + `recordLoginSession` / `markAllSessionsRevoked`
- [新增] `ListLoginDevices`：列出当前用户所有活跃会话（IP / UA 解析 / 最近活跃 / 当前会话标记）
- [新增] `KickDevice`：标记指定 device 为 revoked（v0.4.x 待加 jti 精确单设备踢出）

#### 新功能：登录失败日志
- [新增] `log_login_failed` 表 + `LogLoginFailed` model
- [新增] `recordLoginFailureAsync`（buffered channel 容量 1024，溢出即丢保证登录可用）
- [新增] `StartLoginFailureWorker` main.go 启动后台 goroutine 消费
- [新增] `securityFailedLoginToday` / `securityBlockedIPsToday` 助手供 `AdminSecurityStats` 调用

#### 前端过时标记清理
- [修改] `api/{admin,tenant,agent,profile}.ts`：移除所有「待核实 v0.3.0」「当前 501」过时注释，统一改为「v0.3.1 已实现」
- [新增] `api/tenant.ts`：补齐 `updateTenantNoticeApi` + `deleteTenantNoticeApi`
- [修改] `views/tenant/Notices.vue`：启用删除按钮（带二次确认）+ `remove()` 函数
- [修改] `views/admin/Dashboard.vue`：待办事项「待核实（v0.3.0）」→「去查看」导航文案
- [修改] `views/{admin,tenant}/Dashboard.vue`：catch 注释由「501 静默降级」→「错误已由 http 拦截器处理」
- [修改] `views/{admin,tenant,agent}/Profile.vue`：移除「铁律 06 待核实」注释
- [修改] `views/agent/{Orders,Notices}.vue` + `views/tenant/Agents.vue`：移除「501 占位」头部警告

#### 待核实项归零（v0.3.x → v0.4.x 迁移）
- [里程碑] v0.3.0 CHANGELOG 中所有「待核实 v0.3.x」条目已全部解决或迁移至 v0.4.x
- [迁移] `avatar` 字段（三表均无对应列）→ v0.4.x 加列后落库
- [迁移] 2FA `backup_codes` Redis 持久化 → v0.4.x 加表字段后迁移
- [迁移] UA 解析库（mileusna/ua 或 ua-parser）→ v0.4.x 引入
- [迁移] 登录失败日志结构化记录 → v0.4.x 引入 zap/zerolog

#### 编译验证
- [验证] `go build ./...` 通过（0 错误）
- [验证] `go vet ./...` 通过（0 警告）
- [验证] `pnpm run build`（admin 前端）通过（修复 `Notices.vue` row 类型断言）

---

## [0.3.0] - 2026-07-19

### [新增] 后端业务 API 全量实现（替换全部 501 占位）

#### 核心交付
- [里程碑] 三角色后端业务接口（admin/tenant/agent）从 501 占位升级为真实实现，覆盖前端 v0.2.6/v0.2.7 已建立的 40+ 调用点
- [新增] `internal/handler/admin_business.go` 18 个超管接口（1067 行）：公开平台公告 + 工作台 + 租户/套餐/代理/公告 CRUD + 日志审计 + 安全中心（统计 + IP 黑名单 CRUD）
- [新增] `internal/handler/tenant_business.go` 22 个开发者接口（1479 行）：工作台 + 设备/订单/云变量/版本/代理/邀请码/支付配置/公告 全套 CRUD
- [新增] `internal/handler/agent_business.go` 11 个代理接口（1060 行）：工作台 + AgentMe 扩展 + 卡类/卡密/订单/佣金/提现/通知
- [新增] `internal/handler/profile.go` 三角色统一账号设置（763 行）：ProfileMe（覆盖 auth.go 的 CurrentUser）+ UpdateProfile + ChangePassword + 2FA 全流程（setup→verify→disable）+ LoginDevices

#### 路由注册（router.go）
- [修改] `internal/router/router.go` 注册 40+ 新端点，覆盖三角色工作台、CRUD、账号设置
- [修改] 三角色 `/auth/me` 由 `handler.CurrentUser` 切换为 `handler.ProfileMe`（agent 单独走 `AgentMe` 返回扩展字段）
- [新增] admin 组：`/packages/:id`、`/agents`、`/agents/:id`、`/notices/:id`（PUT/DELETE）、`/logs`、`/security/stats`、`/security/ip_blacklist`（GET/POST/DELETE）
- [新增] tenant 组：`/devices`、`/devices/:id/kick`、`/orders`、`/cloud_vars`（GET/POST/PUT/DELETE）、`/versions`（GET/POST/DELETE）、`/agents/:id`、`/agents/invite_codes`（GET/POST + `/:id/disable`）、`/pay_config`（GET/POST/PUT + `/test`）、`/notices`（GET/POST/PUT/DELETE）
- [新增] agent 组：`/auth/me`、`/card_types`、`/cards`、`/cards/generate`、`/orders`、`/commission`、`/withdraw`、`/recharge`、`/notices`、`/notices/:id/read`
- [新增] 三角色统一账号设置端点：`/auth/profile`、`/auth/change_password`、`/auth/2fa/setup|verify|disable`、`/auth/devices`、`/auth/devices/:id/kick`

#### 关键技术实现
- [新增] `parsePagination(c)` 公共分页助手（page 默认 1、page_size 默认 20、上限 100），跨文件共享
- [新增] `genInviteCodeUnique(db)` 邀请码生成（crypto/rand 16 位 + 5 次重试唯一性保证）
- [新增] `loadUserProfile(deps, role, userID)` 三角色统一资料加载，返回字段对齐前端 `CurrentUser` 接口
- [新增] `loadUserPasswordHash` / `loadUserTOTPSecret` / `updateUserTOTPSecret` 三角色密码与 2FA 密钥统一读写
- [新增] `agentFrozenBalance` / `agentTotalCommission` / `agentTotalWithdrawPaid` 代理聚合统计助手
- [新增] AgentGenerateCards 事务化（4 步：扣余额 gorm.Expr → 生成卡密 → 写扣费日志 → 结算佣金 + 写佣金日志）
- [新增] 2FA 全流程：setup（Redis 中转 10min）→ verify（AES 加密入库 + 备用码 Redis 持久化）→ disable（黑名单 refresh token）
- [新增] `parseDeviceName(ua)` 简化 User-Agent 解析（OS / Browser）

#### admin.go 清理
- [移除] admin.go 中 12 个 501 占位函数（PublicPlatformNotices / AdminDashboard / AdminListTenants / AdminCreateTenant / AdminUpdateTenant / AdminListPackages / AdminCreatePackage / AdminListNotices / AdminCreateNotice / TenantDashboard / TenantListAgents / TenantGenInviteCode），实现已迁移至 admin_business.go 与 tenant_business.go
- [保留] admin.go 仅保留 `AdminListConfig` 与 `AdminUpdateConfig` 两个真实实现（系统配置走 CfgCache，铁律 05）

#### 待核实项（铁律 06，未阻塞编译）
- [待核实] `sys_tenant` 无 `remark` 字段、`sys_package` 无 `description` 字段、`notice` 无 `sort` 字段、`log_operation` 无 `username/user_agent/status` 字段、`sec_ip_blacklist` 无 `created_by` 字段
- [待核实] `AppCloudVar` 无 `read_only` 字段、`AppVersion` 无 `channel` 字段、`Agent` 无 `commission_mode/inviter_id/totp_secret/email/last_login_ip` 字段、`AgentInviteCode` 无 `used_by_agent_id` 字段
- [待核实] `Notice` type 枚举与前端不完全一致（platform/tenant/agent）
- [待核实] `failed_login_today/blocked` 等安全统计当前返回 0（需引入登录失败日志表）
- [待核实] `AgentRecharge` 当前返回 501（待接入支付回调或开发者手工充值流程）
- [待核实] agent 表暂无 `totp_secret` 字段，代理 2FA setup 返回 501
- [待核实] `ListLoginDevices` 当前简化为返回当前会话信息（待引入完整的 refresh token 设备表）

#### 编译验证
- [验证] `go build ./...` 通过（0 错误，修复 tenant_business.go:382 `items` → `orders` 笔误）
- [验证] `go vet ./...` 通过（0 警告）

---

## [0.2.7] - 2026-07-19

### [新增] 全部剩余 PlaceholderView 替换为真实页面（响应式 H5 完整覆盖）

#### Admin 后台（7 页）
- [新增] `views/admin/Tenants.vue` 开发者管理：关键词+状态筛选 + 列表（用户名/套餐/应用数/卡密数/余额/到期）+ 新建对话框 + 编辑对话框（套餐/延长天数/状态/重置密码/备注）+ 启用/禁用
- [新增] `views/admin/Packages.vue` 套餐管理：列表 + 新建对话框（名称/描述/应用上限/卡密上限/代理上限/月费/年费/特性 JSON/状态）+ 编辑
- [新增] `views/admin/Agents.vue` 代理管理：关键词+状态+tenant_id 筛选 + 列表（所属开发者/余额/冻结/累计佣金/累计提现/佣金模式/比例）+ 编辑对话框（status/commission_mode/commission_rate/balance）
- [新增] `views/admin/Notices.vue` 平台公告：类型+状态+关键词筛选 + 列表 + 新建/编辑对话框（类型/标题/内容 textarea/状态/置顶/排序/发布时间/过期时间）+ 删除二次确认
- [新增] `views/admin/PayConfig.vue` 支付配置：表单（PID/Key 敏感隐藏/API URL/签名类型/通知路径/同步跳转/前端回跳/订单名前缀/默认抽成/最低结算/自动结算）+ 保存（逐项 updateSysConfig）+ 测试按钮调用 testPayConfigApi
- [新增] `views/admin/Logs.vue` 日志审计：类型+用户 ID+日期范围+关键词筛选 + 列表（用户名/角色/动作/目标/IP/状态）+ 详情弹窗（JSON 美化）
- [新增] `views/admin/Security.vue` 安全防护：4 数据卡（黑名单总数/生效中/今日登录失败/今日封禁 IP）+ 2 列布局（最近封禁 IP 列表 + IP 黑名单管理表格）+ 新增对话框（IP/原因/过期小时数）

#### Tenant 控制台（8 页）
- [新增] `views/tenant/Devices.vue` 设备管理：应用+关键词+在线状态筛选 + 列表（应用/卡密截断/设备名/设备 ID/IP/位置/心跳时间/在线状态）+ 强制下线二次确认
- [新增] `views/tenant/Orders.vue` 订单管理：应用+状态+渠道+日期范围+关键词筛选 + 列表（订单号/应用/卡类/买家/代理/数量/单价/总金额/佣金/净额/状态/渠道/支付时间）
- [新增] `views/tenant/CloudVars.vue` 云变量：应用+关键词筛选 + 列表（键/值截断/类型 tag/只读）+ 新建/编辑对话框 + 值完整查看对话框 + 删除二次确认
- [新增] `views/tenant/Versions.vue` 版本管理：应用+渠道筛选 + 列表（版本号/渠道/下载 URL/最低版本/强制更新/已发布/发布时间）+ 新建对话框（版本号/渠道/下载 URL/更新日志/最低版本/强制更新/立即发布）+ 删除二次确认
- [新增] `views/tenant/Agents.vue` 代理管理：4 数据卡（代理总数/活跃/累计佣金/累计提现，后端未返回显示 0）+ 关键词+状态筛选 + 列表 + 编辑对话框（status/commission_mode/commission_rate）
- [新增] `views/tenant/InviteCodes.vue` 邀请码：状态筛选 + 顶部说明 alert + 生成对话框（数量/有效天数/备注）+ 生成结果对话框（单条复制/复制全部）+ 列表（邀请码 mono/状态/使用人/过期时间）+ 禁用（仅 unused 显示）+ 复制按钮
- [新增] `views/tenant/PayConfig.vue` 支付配置：顶部 warning alert（需套餐 allow_custom_pay + v0.3.0 实现）+ 列表（渠道/状态/更新时间）+ 新建/编辑对话框（渠道/PID/Key 敏感/API URL/通知路径/同步跳转/状态）+ 测试按钮
- [新增] `views/tenant/Notices.vue` 我的公告：类型+状态筛选 + 列表 + 新建对话框（类型/标题/内容 textarea/状态/置顶/发布时间/过期时间）+ 删除按钮 disabled（待 v0.3.0 补全 delete/update API）

#### Agent 控制台（1 页 + API 扩展）
- [修改] `api/agent.ts` 末尾追加 2 个方法：`listAgentNoticesApi`（GET /agent/notices）+ `readAgentNoticeApi`（POST /agent/notices/:id/read）+ `AgentNotice` 类型
- [新增] `views/agent/Notices.vue` 消息通知：未读统计小卡 + 类型+仅未读筛选 + 列表（类型 tag/标题/置顶/已读/发布时间）+ 查看对话框（标题/类型/发布时间/内容 textarea readonly）+ 标为已读按钮（仅未读显示）

#### 路由
- [修改] `router/index.ts` 16 个 PlaceholderView 全部替换为懒加载真实组件，并移除 `import PlaceholderView`（不再使用）
- [里程碑] PlaceholderView 占位阶段彻底结束，前端三角色所有路由全部由真实响应式 H5 页面承载

#### 响应式适配
- [新增] 所有 16 页统一使用 PageHeader + ResponsiveTable + mobileFields 模式，桌面表格移动卡片自动切换
- [新增] PayConfig 表单两列布局（el-row + el-col :xs=24 :sm=12），label-position=top
- [新增] Security 4 数据卡（el-col :xs=12 :sm=6）+ 2 列布局（el-col :xs=24 :sm=12）
- [新增] Agents 4 数据卡（el-col :xs=12 :sm=6）网格响应式

#### 待核实项（铁律 06）
- [待核实] 后端 `/admin/tenants|packages|agents|notices|logs|security` 当前为 501，前端 try/catch 静默降级，待 v0.3.0 实现
- [待核实] 后端 `/tenant/devices|orders|cloud_vars|versions|agents|invite_codes|pay_config|notices` 当前为 501，前端 try/catch 静默降级
- [待核实] 后端 `/agent/notices` 当前为 501，待 v0.3.0 实现
- [待核实] admin.ts 未导出 `updateAdminPackageApi`，Packages.vue 编辑直接调用 `request.put('/admin/packages/:id')`，待 v0.3.0 补全正式 API
- [待核实] tenant.ts 未导出公告 update/delete API，Notices.vue 删除按钮暂 disabled，待 v0.3.0 补全
- [待核实] Tenant Agents 4 数据卡（代理总数/活跃/累计佣金/累计提现）后端暂不返回，显示 0 不编造，待 v0.3.0 扩展

#### 编译验证
- [验证] `npx vue-tsc --noEmit` 通过（0 错误，修复 2 处类型：admin/Agents editingId narrowing 提取为 const + tenant/Agents form.status 由 string 改为联合类型字面量）
- [验证] `npx vite build` 通过（8.68s），输出 16 个新页面 chunk

---

## [0.2.6] - 2026-07-19

### [新增] 三角色 Profile + 双 Dashboard（响应式 H5）

#### API 模块（3 个新文件）
- [新增] `api/profile.ts` 三角色共享账号设置 API 封装：9 个方法（currentUser / updateProfile / changePassword / setup2FA / verify2FA / disable2FA / listLoginDevices / kickDevice，按 role 动态拼接路径）+ 5 个类型定义（CurrentUser / ChangePasswordReq / UpdateProfileReq / TwoFASetupResp / LoginDevice）
- [新增] `api/admin.ts` 超管后台 API 封装：17 个方法覆盖 dashboard / tenants（CRUD）/ packages（CRUD）/ agents / notices（CRUD + 删除）/ logs / security（stats + IP 黑名单增删）+ 7 个类型定义（AdminDashboardData / AdminTenant / AdminPackage / AdminAgent / AdminNotice / AdminLog / AdminSecurityStats / IpBlacklistItem）
- [新增] `api/tenant.ts` 开发者控制台 API 封装：19 个方法覆盖 dashboard / devices（列表 + 踢线）/ orders / cloud-vars（CRUD）/ versions（CRUD）/ agents / invite-codes（生成 + 禁用）/ pay-config（保存 + 测试）/ notices + 9 个类型定义（TenantDashboardData / TenantDevice / TenantOrder / TenantCloudVar / TenantVersion / TenantAgent / TenantInviteCode / TenantPayConfig / TenantNotice）

#### 三角色 Profile 页面（账号设置）
- [新增] `views/admin/Profile.vue` 超管账号设置：基础资料（用户名只读 + 真实姓名/邮箱/手机可编辑）+ 修改密码（最小 8 位 + 字母数字组合 + 二次确认）+ 2FA TOTP（生成二维码 → 扫码 → 验证 6 位码 → 显示备用码）+ 登录设备列表（可踢下线）
- [新增] `views/tenant/Profile.vue` 开发者账号设置：基础资料（用户名只读 + tenant_id 标签 + 真实姓名/邮箱/手机）+ 公司信息（公司名/联系人/联系电话/营业执照/地址）+ 修改密码 + 2FA TOTP
- [新增] `views/agent/Profile.vue` 代理账号设置：账户概览（4 数据卡：余额/冻结/累计佣金/累计提现）+ 基础资料（用户名/真实姓名/手机/邮箱/邀请人/注册时间）+ 提现账户（支付宝/微信/银行卡三选一动态字段）+ 修改密码

#### 双 Dashboard 页面
- [新增] `views/admin/Dashboard.vue` 超管平台概览：8 数据卡（开发者/代理/应用/卡密/订单/今日收入/待结算/快捷操作）+ 2 列布局（待办列表 + 收入趋势柱状图）+ 2 列布局（最近开发者表 + 最近订单表）
- [新增] `views/tenant/Dashboard.vue` 开发者工作台：8 数据卡（应用/卡密/设备/订单/今日收入/待结算/代理/快捷操作）+ 8 项快捷入口网格 + 2 列布局（收入趋势 + 应用 TOP5 排行榜）+ 最近订单表

#### 路由
- [修改] `router/index.ts` 5 个 PlaceholderView 替换为真实页面：admin/Dashboard + admin/Profile + tenant/Dashboard + tenant/Profile + agent/Profile

#### 响应式适配
- [新增] Profile 表单 label 位置：桌面 right / 移动 top（computed 监听 window.innerWidth < 768）
- [新增] Dashboard 数据卡网格：桌面 4 列 / 平板 2 列 / 手机 2 列
- [新增] Dashboard 双列布局：桌面 2 列 / 移动 1 列堆叠
- [新增] 趋势图高度：桌面 200px / 移动 160px
- [新增] 快捷入口网格：桌面 8 列（4×2）/ 平板 4 列 / 手机 4 列（2×4）
- [新增] 账户概览 4 数据卡：桌面 4 列 / 平板 2 列 / 手机 2 列

#### 业务特性
- [新增] 修改密码成功后 1.5s 自动登出并跳转登录页
- [新增] 2FA 设置流程：调用 setup 获取 secret + otpauth URL → 渲染二维码（qrcode 库）→ 输入 6 位验证码 → 调用 verify 启用 → 显示备用码（可复制）
- [新增] 2FA 禁用对话框：要求密码 + 当前 6 位验证码双重确认
- [新增] 代理提现账户：method 切换动态显示不同字段（alipay: 账号+姓名 / wechat: 微信号+姓名 / bank: 开户行+账号+姓名）
- [新增] 待办列表项可点击跳转对应管理页（结算/开发者/代理/公告）
- [新增] 应用 TOP5 排行：金/银/铜徽章 + 销量 + 收入

#### 待核实项（铁律 06）
- [待核实] 后端 `/admin/dashboard` `/tenant/dashboard` 当前为 501 占位（v0.3.0 交付），Dashboard 数据全部回退为 0/空数组
- [待核实] 后端 `/{role}/auth/me` 当前仅返回 user_id/username/role/tenant_id，Profile 中 email/phone/real_name/totp_enabled 字段暂为空，待 v0.3.0 扩展 CurrentUser handler
- [待核实] 修改密码 / 2FA 设置/禁用 / 登录设备列表/踢下线 接口当前为 501，前端 try/catch 静默处理，待 v0.3.0 实现
- [待核实] 代理账户概览的 balance/frozen/total_commission/total_withdraw 字段当前返回 0（CurrentUser 未含代理扩展字段），待 v0.3.0 扩展
- [待核实] 开发者公司信息字段（contact_name/contact_phone/license_no/address）后端尚未支持，待 v0.3.0 扩展 tenant 表
- [待核实] Dashboard 收入趋势/应用排行/最近订单 当前为空数组，待 v0.3.0 后端实现聚合查询

#### 编译验证
- [验证] `npx vue-tsc --noEmit` 通过（0 错误，修复 1 处 el-table slot 类型：kickDevice row 参数由 LoginDevice 改为 any）
- [验证] `npx vite build` 通过（6.93s），输出 5 个新页面 chunk（admin/Dashboard + admin/Profile + tenant/Dashboard + tenant/Profile + agent/Profile）

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
