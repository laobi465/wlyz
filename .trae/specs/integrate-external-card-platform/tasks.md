# Tasks

- [ ] Task 1: migration 020 + model struct 扩展
  - [ ] SubTask 1.1: 创建 `migrations/020_v0.4.0_external_card_platform.up.sql`：3 张表 DDL（external_card_platform_binding + external_card_product_mapping + external_card_push_log）+ 索引 + 8 项 sys_config seed
  - [ ] SubTask 1.2: 创建 `migrations/020_v0.4.0_external_card_platform.down.sql`：回滚脚本
  - [ ] SubTask 1.3: 修改 `internal/model/model.go` 新增 3 个 GORM struct（ExternalCardPlatformBinding / ExternalCardProductMapping / ExternalCardPushLog），字段与 DDL 对齐

- [ ] Task 2: 平台适配器抽象 + 通用 HTTP 实现
  - [ ] SubTask 2.1: 新建 `internal/externalplatform/adapter.go`：定义 `PlatformAdapter` 接口（`ListProducts` / `PushCards` 两个方法）+ `ProductInfo` / `PushRequest` / `PushResult` 类型
  - [ ] SubTask 2.2: 实现 `GenericHTTPAdapter`：通用 HTTP JSON 适配器，支持配置拉取商品路径 + 上架卡密路径 + 鉴权方式（Authorization Bearer header / query token）+ 字段映射（product_id/product_name/price/stock + card_keys/external_product_id/external_order_id）
  - [ ] SubTask 2.3: 实现 `GetAdapter(platformType string)` 工厂函数，预留扩展点

- [ ] Task 3: Manager 业务层
  - [ ] SubTask 3.1: 新建 `internal/externalplatform/manager.go`：`Manager` 结构体持有 DB + CfgCache + AES Key
  - [ ] SubTask 3.2: 实现 `Manager.CreateBinding` / `ListBindings` / `GetBinding` / `UpdateBinding` / `DeleteBinding`（user_type + user_id 隔离 + AES 加密 api_token + 脱敏返回）
  - [ ] SubTask 3.3: 实现 `Manager.CreateMapping` / `ListMappings` / `GetMapping` / `UpdateMapping` / `DeleteMapping`（binding 归属校验 + card_type 归属校验 + 同 binding 下 external_product_id 唯一）
  - [ ] SubTask 3.4: 实现 `Manager.ListExternalProducts(bindingID)`：解密 token + 调用 adapter.ListProducts + 返回商品列表
  - [ ] SubTask 3.5: 实现 `Manager.PushCards(ctx, userType, userID, mappingID, count)`：事务内生成卡密（复用 card.go 卡密生成逻辑）+ 写 push_log pending → 事务外调 adapter.PushCards → 更新 push_log status
  - [ ] SubTask 3.6: 实现 `Manager.ListPushLogs` / `RetryPushLog`（仅 failed 可重试，retry_count++ 上限校验）

- [ ] Task 4: handler 层（admin/tenant/agent 三端）
  - [ ] SubTask 4.1: 新建 `internal/handler/external_platform.go`：admin 端 3 个 handler（ListBindings 只读 + ListPushLogs 只读 + DisableBinding）
  - [ ] SubTask 4.2: tenant 端 11 个 handler：Bindings CRUD 4 + ListExternalProducts 1 + Mappings CRUD 5 + PushCards 1 + ListPushLogs 1 + RetryPushLog 1
  - [ ] SubTask 4.3: agent 端 11 个 handler：同 tenant 端，user_type=agent + 余额扣款逻辑复用 AgentGenerateCards
  - [ ] SubTask 4.4: 共用辅助函数 `getUserScope(c)` 返回 (userType, userID, tenantID)，三端统一鉴权

- [ ] Task 5: router 注册 + Deps 注入
  - [ ] SubTask 5.1: 修改 `internal/handler/deps.go` 新增 `ExternalPlatformMgr *externalplatform.Manager` 字段
  - [ ] SubTask 5.2: 修改 `internal/router/router.go` 注册 25 条新路由（admin 3 + tenant 11 + agent 11）+ Deps 注入 ExternalPlatformMgr
  - [ ] SubTask 5.3: 修改 `cmd/main.go` 初始化 ExternalPlatformMgr（传入 DB + ConfigCache + AES Key）

- [ ] Task 6: 单元测试
  - [ ] SubTask 6.1: 新建 `internal/externalplatform/externalplatform_test.go`：adapter 工厂 + GenericHTTPAdapter mock 测试 + Manager CRUD 测试 + PushCards 事务测试 + RetryPushLog 测试
  - [ ] SubTask 6.2: 新建 `internal/handler/external_platform_test.go`：admin/tenant/agent 三端 handler 端到端测试（含权限隔离 + 跨用户访问拒绝 + 数量超限 + 余额不足等边界）

- [ ] Task 7: 文档同步 + 验证
  - [ ] SubTask 7.1: `go build ./...` + `go vet ./...` + `go test ./...` 全验证无回归
  - [ ] SubTask 7.2: 更新 `docs/CHANGELOG.md` 新增第十七项迁移完整记录
  - [ ] SubTask 7.3: 更新 `docs/TODO.md` 标记对接任务已完成
  - [ ] SubTask 7.4: 更新 `docs/PROJECT.md` 三角色模块清单各新增 1 项 + migration 总数 19→20 + 表总数 +3
  - [ ] SubTask 7.5: 更新 `docs/SPEC.md` 新增 7.8 章节（外部发卡网平台对接规范）
  - [ ] SubTask 7.6: 更新 `README.md` 功能表格新增「外部平台对接」行 + v0.4.0 进度行
  - [ ] SubTask 7.7: 更新 `PROMPT.md` v0.4.0 进度行 + 第十七项迁移详细记录
  - [ ] SubTask 7.8: git commit + push 到 GitHub + 更新仓库描述

# Task Dependencies
- Task 2 依赖 Task 1（model struct）
- Task 3 依赖 Task 2（adapter 抽象）
- Task 4 依赖 Task 3（Manager 业务层）
- Task 5 依赖 Task 4（handler 已就绪）
- Task 6 依赖 Task 5（路由注册完成可端到端测试）
- Task 7 依赖 Task 6（测试全 PASS 后同步文档）
- Task 1 与其他无依赖，可优先独立完成
