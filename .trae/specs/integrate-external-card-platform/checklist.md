# Checklist

## 数据库层
- [ ] migration 020 up.sql 包含 3 张表 DDL（external_card_platform_binding / external_card_product_mapping / external_card_push_log）
- [ ] 3 张表均包含必要的索引（user_type+user_id 复合索引 / binding_id 索引 / mapping_id 索引 / status 索引）
- [ ] external_card_platform_binding 含 unique 约束（user_type + user_id + nickname）
- [ ] external_card_product_mapping 含 unique 约束（binding_id + external_product_id）
- [ ] migration 020 up.sql 包含 8 项 external.platform.* sys_config seed
- [ ] migration 020 down.sql 完整回滚（先删表后删 sys_config）
- [ ] model.go 新增 3 个 GORM struct 字段与 DDL 完全对齐

## 平台适配器层
- [ ] PlatformAdapter 接口定义 ListProducts 和 PushCards 两个方法
- [ ] GenericHTTPAdapter 实现 PlatformAdapter 接口
- [ ] GenericHTTPAdapter 支持 Authorization Bearer header 和 query token 两种鉴权方式
- [ ] GenericHTTPAdapter 支持配置拉取商品路径和上架卡密路径
- [ ] GenericHTTPAdapter HTTP 调用带超时（从 sys_config 读取）
- [ ] GenericHTTPAdapter HTTP 调用支持重试（从 sys_config 读取次数和间隔）
- [ ] GetAdapter 工厂函数支持 platform_type=generic_http，未知类型返回错误
- [ ] verify_ssl=0 时跳过 SSL 证书校验

## Manager 业务层
- [ ] CreateBinding 使用 AES-256-GCM 加密 api_token，DB 不存明文
- [ ] CreateBinding 校验 api_endpoint 以 https:// 开头
- [ ] CreateBinding 校验同用户下 nickname 唯一
- [ ] ListBindings 返回脱敏 api_token（前 4 后 4 字符 + 中间星号）
- [ ] 所有 Binding/Mapping/PushLog 查询均带 user_type + user_id 隔离
- [ ] CreateMapping 校验 binding 归属当前用户
- [ ] CreateMapping 校验 card_type 归属当前用户（tenant 校验 tenant_id，agent 校验可售卡类）
- [ ] CreateMapping 校验同 binding_id 下 external_product_id 唯一
- [ ] ListExternalProducts 解密 token + 调用 adapter + 失败返回详细 error_message
- [ ] PushCards 事务内生成卡密 + 写 push_log pending
- [ ] PushCards 事务外调用 adapter.PushCards + 更新 push_log status
- [ ] PushCards 失败时卡密保留在 app_card 表（status=unused）不回滚
- [ ] PushCards 校验 count <= external.platform.max_push_count
- [ ] agent 端 PushCards 复用 AgentGenerateCards 余额扣款 + 佣金计算逻辑
- [ ] agent 端 PushCards 余额不足返回 5004 错误
- [ ] RetryPushLog 仅 status=failed 可重试
- [ ] RetryPushLog 重试时 retry_count++ 且校验 max_retry 上限
- [ ] 禁用状态的 binding 所有操作返回 1003 错误

## handler 层
- [ ] admin 端 3 个 handler 全部只读（ListBindings / ListPushLogs / DisableBinding）
- [ ] tenant 端 11 个 handler 全部带 tenant_id 隔离
- [ ] agent 端 11 个 handler 全部带 agent_id + tenant_id 隔离
- [ ] getUserScope 辅助函数统一处理三端 user_type + user_id + tenant_id
- [ ] 所有响应使用 middleware.Success / middleware.Fail 统一格式
- [ ] 写操作（CreateBinding / UpdateBinding / DeleteBinding / CreateMapping / UpdateMapping / DeleteMapping / PushCards / RetryPushLog / DisableBinding）调用 RecordOperation 记录审计日志

## router 层
- [ ] adminAuth 组注册 3 条新路由
- [ ] tenantAuth 组注册 11 条新路由
- [ ] agentAuth 组注册 11 条新路由
- [ ] Deps 注入 ExternalPlatformMgr（nil 时 handler 返回 1007 服务暂不可用）
- [ ] cmd/main.go 初始化 ExternalPlatformMgr 传入 DB + ConfigCache + AES Key

## 单元测试
- [ ] externalplatform 包测试覆盖 adapter 工厂 + GenericHTTPAdapter mock + Manager CRUD + PushCards 事务 + RetryPushLog
- [ ] handler 测试覆盖三端权限隔离 + 跨用户访问拒绝 + 数量超限 + 余额不足 + 重试已成功推送拒绝
- [ ] 测试使用 SQLite 内存库 + miniredis + httptest mock 外部平台 API
- [ ] 测试不依赖外部真实服务，所有外部 API 调用走 mock

## 铁律合规
- [ ] 铁律 04：所有错误码、字段名、配置键常量化（无硬编码字符串）
- [ ] 铁律 05：8 项配置走 sys_config + ConfigCache API（GetBool/GetInt/GetString 带 ctx + fallback）
- [ ] 铁律 06：外部平台 API 协议未明确处标注「待核实」+ 所有测试基于固定输入断言无随机性
- [ ] api_token 加密存储 + 返回脱敏 + 不打日志
- [ ] 跨用户访问严格隔离 + 跨租户访问严格隔离

## 文档同步
- [ ] CHANGELOG.md 新增第十七项迁移完整记录
- [ ] TODO.md 标记对接任务已完成
- [ ] PROJECT.md 三角色模块清单各新增 1 项 + migration 总数 19→20 + 表总数 +3
- [ ] SPEC.md 新增 7.8 章节（外部发卡网平台对接规范）
- [ ] README.md 功能表格新增「外部平台对接」行 + v0.4.0 进度行
- [ ] PROMPT.md v0.4.0 进度行 + 第十七项迁移详细记录

## 验证
- [ ] `go build ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] `go test ./...` 全 PASS 无回归（17 个测试包 + 新增 2 个测试包）
- [ ] git commit + push 到 GitHub 成功
- [ ] GitHub 仓库描述更新
