# 规范文档 (SPEC)

## 1. 代码规范

### 1.1 命名规范

#### Go 后端
| 类型 | 规范 | 示例 |
|---|---|---|
| 包名 | 全小写，单数 | `package auth` |
| 文件名 | 全小写，下划线分隔 | `user_service.go` |
| 结构体 | 驼峰，首字母大写（导出）/小写（私有） | `type UserService struct` |
| 接口 | 驼峰，加 `er` 后缀 | `type Reader interface` |
| 常量 | 全大写，下划线分隔 | `const MAX_RETRY = 3` |
| 方法 | 驼峰 | `func (s *UserService) Create()` |

#### Vue3 前端
| 类型 | 规范 | 示例 |
|---|---|---|
| 组件名 | PascalCase，多单词 | `UserProfile.vue` |
| 文件名 | kebab-case（视图）/ PascalCase（组件） | `user-profile.vue` / `Pagination.vue` |
| 变量/方法 | camelCase | `const userInfo = ref()` |
| 常量 | 全大写，下划线分隔 | `const API_BASE_URL` |
| Props | camelCase | `defineProps<{ userId: number }>()` |
| 事件 | kebab-case | `emit('user-updated')` |
| CSS 类 | kebab-case，BEM 命名 | `.user-card__title--active` |

### 1.2 格式规范

#### Go
- 缩进：Tab
- 行宽：120 字符
- 用 `gofmt` 格式化
- 用 `goimports` 管理导入
- 导入分组：标准库 / 第三方 / 本项目

```go
// 正确示例
import (
    "context"
    "fmt"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "github.com/your-org/keyauth-saas/apps/server/internal/model"
)
```

#### TypeScript/Vue
- 缩进：2 空格
- 行宽：100 字符
- 用 ESLint + Prettier 统一格式
- 使用单引号 `'`
- 语句末尾不加分号
- 使用 `const`/`let`，禁用 `var`

```typescript
// 正确示例
import { ref, computed } from 'vue'
import type { UserInfo } from '@/types'
import { getUserInfo } from '@/api/user'

const userInfo = ref<UserInfo | null>(null)
const isLoading = computed(() => !userInfo.value)
```

### 1.3 注释规范

#### Go
- 导出函数必须有注释，以函数名开头
- 包必须有包注释（在 `doc.go` 中）
- 复杂逻辑必须有行内注释

```go
// CreateUser 创建新用户
// 参数：
//   - ctx: 上下文
//   - req: 创建请求
// 返回：用户ID 或 错误
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserReq) (uint64, error) {
    // 校验邮箱唯一性
    if exists, err := s.repo.ExistsByEmail(ctx, req.Email); err != nil {
        return 0, err
    } else if exists {
        return 0, ErrEmailExists
    }
    // ...
}
```

#### Vue/TypeScript
- 复杂组件顶部必须有功能说明注释
- Props/Emits 必须有注释
- TODO/FIXME 必须带 issue 号

```vue
<!--
  用户卡密列表组件
  功能：展示当前用户的卡密列表，支持筛选、解绑、查看详情
  作者：@yourname
-->
<script setup lang="ts">
/** 当前用户ID */
const props = defineProps<{
  userId: number
}>()

// TODO(#123): 添加批量解绑功能
</script>
```

---

## 2. 架构规范

### 2.1 分层架构（后端）

实际采用 **3 层简化架构**（Handler 直连 GORM，无独立 service/repository 层），通过 `handler.Deps` 注入共享依赖：

```
┌─────────────────────────────────────┐
│ Handler 层（HTTP 处理器，17 文件）   │
│ - 路由匹配、参数校验、响应封装       │
│ - 业务逻辑 + GORM 操作（简化版）    │
└──────────────┬──────────────────────┘
               │ 调用
┌──────────────▼──────────────────────┐
│ 辅助包（quota / heartbeat / auth）  │
│ - 跨 handler 共享的业务能力          │
└──────────────┬──────────────────────┘
               │ 调用
┌──────────────▼──────────────────────┐
│ Model 层 + Middleware 层            │
│ - 30 个 GORM struct（纯数据结构）   │
│ - auth/tenant/signature/ratelimit   │
└─────────────────────────────────────┘
```

**当前实现说明**：
- 项目当前未拆分独立 service/repository 层，业务逻辑直接写在 handler 内（事务用 `deps.DB.Transaction()`）
- 共享辅助能力封装在 `internal/quota` / `internal/heartbeat` / `internal/auth` 子包
- 后续若 handler 过胖，可在 `internal/service/<module>/` 下抽出业务层（v0.4.x 重构计划）

**禁止行为**：
- ❌ Handler 跨租户读写（必须经 `middleware.TenantScope` 注入 tenant_id）
- ❌ 金额变动非事务（必须 `deps.DB.Transaction()` + `FOR UPDATE`）
- ❌ Model 包含方法（仅纯数据结构）
- ❌ 业务代码硬编码配置（必须走 `cfgCache.GetString("key", "默认值")`，铁律 05）

### 2.2 模块边界

按 handler 文件粒度划分，每个文件对应一个业务域：

```
internal/handler/
├── auth.go             # 三角色登录 + RefreshToken + AgentRegister（v0.3.6 待实现）
├── session.go          # 登出 + 当前用户
├── profile.go          # 三角色统一账号设置（ProfileMe + UpdateProfile + 2FA + LoginDevices）
├── public.go           # H5 公共 API（PublicAppInfo + PublicCardTypes，v0.3.5）
├── app.go              # 应用 CRUD + 密钥轮换
├── card.go             # 卡类 + 卡密生成/封禁/解封/删除 + CSV 导入导出（v0.3.6）+ 封禁联动设备下线（v0.3.6）
├── client.go           # 客户端验证 API（9 个端点）
├── pay.go              # 平台总支付 + EpayTenantNotify（v0.3.6 待实现）
├── admin.go            # 超管：sys_config CRUD + TestPayConfig
├── admin_business.go   # 超管业务：dashboard + 租户/套餐/代理/公告 CRUD + 日志 + 安全
├── admin_finance.go    # 超管财务：开发者提现审核 + 批量结算 + 对账报表
├── tenant_business.go  # 开发者业务：dashboard + 应用/卡密/云变量/版本/代理/邀请码/公告/订单/设备
├── tenant_finance.go   # 开发者财务：代理充值/提现审核（6 个 handler）
├── tenant_settle.go    # 开发者结算：结算记录 + 余额概览 + 流水 + 提现申请
├── agent_business.go   # 代理业务：dashboard + me + 卡类/卡密/订单/佣金/提现/通知
├── log_worker.go       # 异步日志 worker（验证 4096 + 操作 2048）
└── deps.go             # 依赖注入容器
```

**跨文件通信**：
- 通过 `handler.Deps` 共享 DB/Redis/Crypto/Config/CfgCache
- 通过 `RecordOperation(deps, c, ...)` 切面 helper 写操作日志
- 通过 `writeVerifyLogCtx(deps, c, ...)` 切面 helper 异步写验证日志

### 2.3 设计模式

| 模式 | 应用场景 |
|---|---|
| 依赖注入 | Service/Repository 通过构造函数注入 |
| 工厂模式 | 卡密生成器、支付通道选择器 |
| 策略模式 | 佣金计算（percentage / diff）、支付通道（平台总 / 自有易支付） |
| 观察者模式 | 公告推送、支付方式变更通知代理 |
| 中间件模式 | Gin 中间件链 |
| 仓储模式 | 数据访问层 |

### 2.4 前端架构（Vue3）

```
src/
├── api/              # API 请求封装（按模块）
├── components/       # 通用组件
├── composables/      # 组合式函数
├── layouts/          # 布局组件（Admin/Tenant/Agent）
├── router/           # 路由配置
├── stores/           # Pinia 状态管理
├── styles/           # 全局样式
├── types/            # TypeScript 类型定义
├── utils/            # 工具函数
└── views/            # 页面视图
    ├── admin/        # 超管页面
    ├── tenant/       # 开发者页面
    └── agent/        # 代理页面
```

**状态管理原则**：
- 全局状态用 Pinia（用户信息、权限、通知）
- 局部状态用 `ref`/`reactive`
- 跨组件共享但非全局用 `composables`

---

## 3. 接口规范

### 3.1 API 设计原则

- RESTful 风格
- 资源用名词复数：`/api/v1/apps`、`/api/v1/cards`
- 动作用动词：`/api/v1/cards/{id}/ban`
- 版本前缀：`/api/v1/`
- 客户端验证 API 独立前缀：`/api/v1/client/`

### 3.2 请求格式

#### 请求头（管理后台）
```http
GET /api/v1/apps HTTP/1.1
Authorization: Bearer eyJhbGciOiJIUzI1NiJ9...
Content-Type: application/json
X-Tenant-Id: 1001
```

#### 请求头（客户端验证 API）
```http
POST /api/v1/client/login HTTP/1.1
Content-Type: application/json
X-App-Key: ak_5f8d7e6c5b4a3210
X-Timestamp: 1721374800
X-Nonce: a1b2c3d4e5f6
X-Signature: 9f8e7d6c5b4a39281706f5e4d3c2b1a0
```

签名原文格式：
```
METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
```

例如：
```
POST
/api/v1/client/login
1721374800
a1b2c3d4e5f6
{"card_key":"K2X9-AB7C-MN4P-QR8S","hwid":"abc123"}
```

签名算法：`HMAC-SHA256(sign_secret, 原文)` → 64 位小写 hex

### 3.3 响应格式

#### 统一响应结构
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "name": "应用A"
  },
  "request_id": "req-uuid-xxx",
  "timestamp": 1721374800
}
```

#### 字段说明
| 字段 | 类型 | 说明 |
|---|---|---|
| code | int | 0=成功，非 0=错误码 |
| message | string | 提示信息 |
| data | any | 业务数据 |
| request_id | string | 请求追踪 ID |
| timestamp | int | 服务器时间戳 |

#### 客户端验证 API 响应（带签名）
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "xxx",
    "expires_at": 1721461200,
    "card": {
      "type": "monthly",
      "expires_at": "2026-08-19 10:30:00",
      "remaining_seconds": 2678400
    },
    "features": {
      "pro": true,
      "max_devices": 1
    }
  },
  "signature": "RSA-4096 签名(base64)"
}
```

### 3.4 分页规范

请求参数：
```
GET /api/v1/cards?page=1&page_size=20&keyword=K2X9&status=active
```

响应结构：
```json
{
  "code": 0,
  "data": {
    "list": [...],
    "total": 156,
    "page": 1,
    "page_size": 20,
    "total_pages": 8
  }
}
```

### 3.5 错误码规范

#### 通用错误码（1xxx）
| 错误码 | 含义 |
|---|---|
| 0 | 成功 |
| 1001 | 参数错误 |
| 1002 | 未授权 |
| 1003 | 禁止访问 |
| 1004 | 资源不存在 |
| 1005 | 请求过于频繁 |
| 1006 | 服务器内部错误 |
| 1007 | 服务暂不可用 |

#### 认证错误码（2xxx）
| 错误码 | 含义 |
|---|---|
| 2001 | 用户名或密码错误 |
| 2002 | Token 无效 |
| 2003 | Token 已过期 |
| 2004 | 2FA 验证码错误 |
| 2005 | 账号已被封禁 |

#### 卡密错误码（3xxx）
| 错误码 | 含义 |
|---|---|
| 3001 | 卡密不存在 |
| 3002 | 卡密已使用 |
| 3003 | 卡密已过期 |
| 3004 | 卡密已被封禁 |
| 3005 | 设备数超限 |
| 3006 | 设备指纹不匹配 |
| 3007 | 解绑次数耗尽 |

#### 支付错误码（4xxx）
| 错误码 | 含义 |
|---|---|
| 4001 | 支付配置未启用 |
| 4002 | 订单已支付 |
| 4003 | 订单已过期 |
| 4004 | 支付回调验签失败 |
| 4005 | 套餐未开通自定义支付 |

#### 代理错误码（5xxx）
| 错误码 | 含义 |
|---|---|
| 5001 | 邀请码无效 |
| 5002 | 邀请码已过期 |
| 5003 | 邀请码已用尽 |
| 5004 | 余额不足 |
| 5005 | 提现金额超限 |

### 3.6 HTTP 状态码使用

| 状态码 | 使用场景 |
|---|---|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 429 | 限流 |
| 500 | 服务器错误 |

### 3.7 接口文档

- 使用 OpenAPI 3.0 规范
- 自动生成：后端用 `swaggo/swag`，前端用 `openapi-typescript` 生成类型
- 文档地址：`https://yourdomain.com/docs`

---

## 4. 提交规范

### 4.1 Commit Message 格式

使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>(<scope>): <subject>

<body>

<footer>
```

#### type 取值
| type | 说明 |
|---|---|
| feat | 新功能 |
| fix | 修复 bug |
| docs | 文档变更 |
| style | 代码格式（不影响功能） |
| refactor | 重构 |
| perf | 性能优化 |
| test | 测试 |
| chore | 构建/工具变更 |
| ci | CI 配置 |
| revert | 回滚 |

#### scope 取值
按模块：`auth` / `tenant` / `app` / `card` / `device` / `verify` / `pay` / `agent` / `notice` / `stats` / `admin` / `h5` / `sdk` / `deploy`

#### 示例
```
feat(card): 新增卡密批量生成功能

- 支持自定义前缀、分组、数量
- 使用 SecureRandom 生成
- SHA-512 校验位防伪
- 批量 INSERT 优化性能

Closes #123
```

```
fix(pay): 修复易支付回调验签失败问题

回调用 sign 字段比对时未做常量时间比较，存在时序攻击风险

Closes #456
```

### 4.2 分支策略

```
main          ─────●────────────●────────────●─────────
                    \          /              /
develop    ────●─────●────●────●────●────●────●────
              /         /              \
feature/xxx  ●─────────●                ●
                                      card-batch-gen
```

#### 分支命名
- `feature/<module>-<feature>`：`feature/card-batch-gen`
- `fix/<module>-<issue>`：`fix/pay-sign-timing`
- `release/<version>`：`release/0.2.0`
- `hotfix/<module>-<issue>`：`hotfix/auth-token-leak`

### 4.3 PR 规范

- PR 标题遵循 Commit Message 格式
- PR 描述必须包含：
  - 变更说明
  - 测试方式
  - 截图（前端变更）
  - 关联 Issue
- 至少 1 人 Code Review 通过
- CI 通过（lint + test + build）
- 禁止直接 push 到 `main`/`develop`

### 4.4 版本标签

发布版本时打 tag：
```
git tag -a v0.2.0 -m "Release 0.2.0 - MVP"
git push origin v0.2.0
```

---

## 5. 测试规范

### 5.1 测试覆盖率要求

| 模块 | 覆盖率要求 |
|---|---|
| 核心业务（auth/card/verify/pay） | ≥ 80% |
| 一般业务（tenant/app/agent） | ≥ 60% |
| 工具包（pkg/*） | ≥ 90% |
| Handler 层 | ≥ 50% |
| 前端组件 | ≥ 40% |

### 5.2 测试分类

#### 后端
- **单元测试**：`*_test.go`，与被测文件同目录
- **集成测试**：`internal/test/integration/`，使用真实 MySQL/Redis（Docker）
- **API 测试**：`internal/test/api/`，使用 `httptest`
- **基准测试**：`*_bench_test.go`

```go
// 单元测试示例
func TestCardService_Generate(t *testing.T) {
    tests := []struct {
        name    string
        req     *GenerateCardReq
        wantErr error
    }{
        {
            name:    "正常生成",
            req:     &GenerateCardReq{Count: 10, Type: "monthly"},
            wantErr: nil,
        },
        {
            name:    "数量超限",
            req:     &GenerateCardReq{Count: 100000, Type: "monthly"},
            wantErr: ErrCountExceed,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := cardService.Generate(tt.req)
            if err != tt.wantErr {
                t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
            }
            _ = got
        })
    }
}
```

#### 前端
- **单元测试**：Vitest
- **组件测试**：@vue/test-utils
- **E2E 测试**：Playwright

### 5.3 CI 流水线

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]

jobs:
  backend-test:
    runs-on: ubuntu-latest
    services:
      mysql: ...
      redis: ...
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test -race -coverprofile=coverage.out ./...
      - run: go tool cover -func=coverage.out
  
  frontend-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v2
      - run: pnpm install
      - run: pnpm lint
      - run: pnpm test
      - run: pnpm build
  
  docker-build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker build -t keyauth-api -f deploy/Dockerfile.api .
```

---

## 6. 安全规范

### 6.1 输入校验

- 所有外部输入必须校验（用 `validator` 包）
- 字符串长度、格式、范围校验
- 拒绝非法字符（SQL 注入、XSS）

```go
type CreateAppReq struct {
    Name        string `json:"name" validate:"required,min=2,max=64"`
    Description string `json:"description" validate:"max=500"`
    MaxDevices  int    `json:"max_devices" validate:"required,min=1,max=10"`
}
```

### 6.2 权限控制

#### 三级权限模型
| 角色 | 权限范围 |
|---|---|
| 平台超管 | 全平台 |
| 开发者（租户） | 自己租户内 |
| 代理 | 自己代理账号内 |

#### 强制校验
- **租户隔离中间件**：所有租户相关查询自动注入 `tenant_id`
- **资源归属校验**：操作前校验资源是否属于当前租户
- **代理归属校验**：操作前校验代理是否属于当前租户

```go
// 租户隔离中间件示例
func TenantIsolation() gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.GetUint64("tenant_id")
        c.Set("db", db.WithContext(c).Where("tenant_id = ?", tenantID))
        c.Next()
    }
}

// 资源归属校验
func (s *CardService) GetCard(ctx context.Context, cardID uint64) (*Card, error) {
    tenantID := ctx.Value("tenant_id").(uint64)
    card, err := s.repo.GetByID(ctx, cardID)
    if err != nil {
        return nil, err
    }
    if card.TenantID != tenantID {
        return nil, ErrForbidden  // 越权访问
    }
    return card, nil
}
```

### 6.3 敏感信息处理

#### 加密存储
| 字段 | 加密算法 |
|---|---|
| 密码 | bcrypt (cost=12) |
| AppSecret / SignSecret | AES-256-GCM |
| 易支付商户密钥 | AES-256-GCM |
| JWT 签名密钥 | 环境变量 |
| 2FA TOTP Secret | AES-256-GCM |

#### 输出脱敏
```go
// 手机号脱敏：138****0000
func MaskPhone(phone string) string {
    if len(phone) < 7 { return phone }
    return phone[:3] + "****" + phone[len(phone)-4:]
}

// 卡密脱敏：K2X9-****-****-****（仅展示前4位）
func MaskCardKey(key string) string {
    parts := strings.Split(key, "-")
    if len(parts) < 2 { return key }
    masked := []string{parts[0]}
    for i := 1; i < len(parts); i++ {
        masked = append(masked, "****")
    }
    return strings.Join(masked, "-")
}
```

### 6.4 防御措施

| 威胁 | 防御 |
|---|---|
| SQL 注入 | GORM 参数化查询，禁用字符串拼接 |
| XSS | 输出转义，CSP 头 |
| CSRF | 双重提交 Cookie + SameSite=Lax |
| 暴力破解 | IP 限流 + 卡密错误 5 次封 IP 1h |
| 重放攻击 | Nonce + Timestamp |
| 中间人攻击 | 强制 HTTPS + HMAC + RSA 响应签名 |
| 时序攻击 | 常量时间比较 `hmac.Equal()` |
| 路径遍历 | 文件路径校验 `filepath.Clean()` |
| DDoS | Cloudflare WAF + Nginx 限流 |

### 6.5 安全头配置（Nginx）

```nginx
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
add_header X-Frame-Options "DENY" always;
add_header X-Content-Type-Options "nosniff" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https:; font-src 'self' https:;" always;
```

### 6.6 密钥管理

- 密钥不进代码仓库，全部走环境变量或 `sys_config`（加密存储）
- 密钥定期轮换（建议 90 天）
- AppSecret/SignSecret 支持后台轮换（保留旧密钥 7 天用于平滑过渡）
- RSA 私钥文件权限 `600`

---

## 7. 性能规范

### 7.1 性能指标

| 接口 | P99 响应时间 | QPS |
|---|---|---|
| 客户端验证 API | < 50ms | ≥ 1000 |
| 心跳 API | < 30ms | ≥ 5000 |
| 卡密生成（10000 条） | < 5s | - |
| 管理后台 API | < 200ms | ≥ 100 |

### 7.2 优化措施

- **缓存**：Redis 三级缓存（卡密验证、设备状态、配置）
- **索引**：所有查询字段必须有索引
- **连接池**：MySQL（max=100）、Redis（max=50）
- **批量操作**：卡密生成用批量 INSERT
- **分表分区**：日志表按月分区
- **异步处理**：日志写入、通知发送用消息队列
- **CDN**：静态资源走 CDN

### 7.3 v0.3.6 新增接口规范

#### 卡密 CSV 导出
- 路由：`GET /api/v1/tenant/cards/export`
- 鉴权：tenant 角色 + Authorization Header
- 查询参数：`app_id` / `status` / `batch_no` / `keyword`（与列表 API 一致）
- 响应：`Content-Type: text/csv; charset=utf-8` + UTF-8 BOM + `Content-Disposition: attachment`
- 条数上限：从 `sys_config.card.export.max_rows` 读取，默认 10000，最大 100000（兜底防拖垮服务）
- 字段顺序：ID,AppID,CardTypeID,CardKey,Checksum,Status,BatchNo,Prefix,GroupTag,DurationSeconds,UsedCount,MaxUses,ActivatedAt,ExpiresAt,LastVerifyAt,CreatedBy,CreatorType,OrderID,BannedAt,BannedReason,CreatedAt

#### 卡密 CSV 导入
- 路由：`POST /api/v1/tenant/cards/import`
- 鉴权：tenant 角色
- 请求体：JSON（前端解析 CSV 后传明文数组）
  ```json
  {
    "app_id": 1,
    "card_type_id": 1,
    "prefix": "VIP",
    "group_tag": "",
    "duration_seconds": 0,
    "max_uses": 0,
    "cards": ["XXX-YYYY-ZZZZ", "..."]
  }
  ```
- 条数上限：从 `sys_config.card.import.max_rows` 读取，默认 5000，最大 50000
- 行为：
  - 未传 `duration_seconds` / `max_uses` / `prefix` 时取卡类默认值
  - 套餐配额校验（quota.CheckMaxCards）
  - 卡密明文去重 + 空值过滤
  - 事务批量入库，重复 hash（SHA-512）跳过并记失败明细
  - 批次号前缀 `I`（Import）+ 日期 + 用户 ID 后 6 位
- 响应：`{ batch_no, success_count, failed_count, empty_count, dup_count, failed[] }`
- 操作日志：自动写入 `log_operation`（module=card, action=import_csv）

#### 封禁卡密联动设备下线
- 触发：`POST /api/v1/tenant/cards/:id/ban` 成功后
- 行为：
  1. 查询 `app_device` 中 `card_id = ? AND tenant_id = ? AND status = 'active'` 的所有设备
  2. 循环调 `heartbeat.Remove(ctx, rdb, appID, deviceID)` 清 Redis ZSET + Hash
  3. DB 批量更新 `app_device.status = 'banned'` + `last_heartbeat_at = NULL`
- 容错：Redis 清理失败不阻塞封禁主流程（卡密已 banned，下次 verify 会因 card.status 拒绝）

#### 安装向导（首次部署用）

**铁律 06 重点**：通过 `sys_admin.password_hash` 是否含占位串 `PLACEHOLDER_BCRYPT_HASH` 判定 installed 状态，禁止用 `count(*) > 0` 判定（seed 已插入 1 行占位 admin）。

##### GET /api/v1/install/status
- 鉴权：无（公开接口，仅首次部署阶段可访问）
- 路由：`v1.GET("/install/status")`
- 行为：
  1. 调 `checkInstalled(db)`：查 `sys_admin` id=1，若不存在返回 `installed=false`；若 `password_hash` 含 `PLACEHOLDER_BCRYPT_HASH` 返回 `installed=false`，否则 `installed=true`
  2. 已安装时返回当前超管用户名 + 平台域名；未安装时返回占位字段
- 响应（未安装）：
  ```json
  {
    "code": 0,
    "data": {
      "installed": false,
      "admin_name": "",
      "domain": "",
      "server_time": "2026-07-20T10:00:00Z"
    }
  }
  ```
- 响应（已安装）：
  ```json
  {
    "code": 0,
    "data": {
      "installed": true,
      "admin_name": "admin",
      "domain": "https://example.com",
      "server_time": "2026-07-20T10:00:00Z"
    }
  }
  ```

##### POST /api/v1/install
- 鉴权：无（仅首次部署可用，handler 内二次校验 installed 状态）
- 路由：`v1.POST("/install")`
- 请求体：
  ```json
  {
    "admin_username": "admin",
    "admin_password": "StrongPwd@2026",
    "admin_email": "admin@example.com",
    "admin_phone": "13800138000",
    "platform_domain": "https://example.com",
    "platform_name": "KeyAuth SaaS",
    "platform_notify_email": "notify@example.com",
    "agent_register_fee": 100.00,
    "platform_commission_rate": 5.00
  }
  ```
- 参数校验：
  - `admin_username`：3-64 字符，字母数字下划线
  - `admin_password`：8-64 字符，至少含字母+数字
  - `admin_email`：标准邮箱格式
  - `agent_register_fee`：≥ 0
  - `platform_commission_rate`：0-100
- 行为（事务）：
  1. 二次校验 `checkInstalled(db)`，已安装则返回 `400 已安装，禁止重复初始化`
  2. `crypto.HashPassword(password)` 计算 bcrypt cost=12 哈希
  3. 事务更新 `sys_admin` id=1：`username` + `password_hash` + `email` + `phone`
  4. 事务 upsert 6 个 `sys_config` 项：
     - `platform.domain` = 请求 `platform_domain`
     - `platform.name` = 请求 `platform_name`
     - `platform.notify_email` = 请求 `platform_notify_email`
     - `agent.register_fee` = 请求 `agent_register_fee`
     - `pay.platform_commission_rate` = 请求 `platform_commission_rate`
     - `platform.installed_at` = 当前 RFC3339 时间戳
  5. 事务提交后调 `deps.CfgCache.InvalidateAll(ctx)` 刷新 Redis 配置缓存
  6. `RecordOperation` 写入操作日志（detail 不含密码明文）
- 响应：
  ```json
  {
    "code": 0,
    "data": {
      "installed": true,
      "admin_name": "admin",
      "installed_at": "2026-07-20T10:00:00Z",
      "message": "安装完成，请使用新账号登录"
    }
  }
  ```
- 前端流程：4 步向导（环境检测 → 超管账号 → 平台配置 → 完成），路由 `/install`，`meta.public = true` 不走鉴权

#### 代理注册付费流程（方案 B：先支付后建 Agent）

**设计原则**：避免引入 `pending_payment` 状态破坏 `AgentLogin` 现有 `status != "active"` 不变量。代理行仅在支付回调事务内创建且 `Status="active"`，可直接登录。

**关键设计**：
- 订单号前缀 `REG`（代理注册）与 `ORD`（卡密购买）区分，`EpayNotify` 通过 `dispatchPaidOrder` 按前缀分发
- 密码 bcrypt 哈希短期缓存到 Redis（`agent_register:pwd:{order_no}`，TTL=`pay.order_expire_seconds` 默认 1800s），DB 不存明文也不存哈希
- 注册费不进 `PlatformSettlement`（直接归平台，与卡密抽成解耦）

##### GET /api/v1/public/auth/agent/register/config
- 鉴权：无（公开接口，供注册页未登录时读取）
- 行为：从 `sys_config` 读取 `agent.register.fee` + `pay.platform.enabled` + `pay.platform.methods` + `pay.order_expire_seconds`
- 响应：
  ```json
  {
    "code": 0,
    "data": {
      "register_fee": 99.00,
      "pay_enabled": true,
      "pay_methods": ["alipay", "wxpay", "qqpay"],
      "order_expire_seconds": 1800
    }
  }
  ```
- 安全：不返回敏感字段（`gateway_url` / `pid` / `key_encrypted`）

##### POST /api/v1/public/auth/agent/register
- 鉴权：无（公开接口）
- 请求体：
  ```json
  {
    "invite_code": "ABCD1234EFGH5678",
    "username": "agent001",
    "password": "StrongPwd@2026",
    "phone": "13800138000",
    "pay_type": "alipay"
  }
  ```
- 行为：
  1. 校验平台支付总开关 `pay.platform.enabled`
  2. 校验邀请码：`status=active` + `used_count < max_uses` + `expires_at > now`
  3. 校验用户名在所属租户内唯一
  4. `quota.CheckMaxAgents` 校验套餐代理数上限（第一道防线）
  5. 读 `agent.register.fee` 注册费（默认 99.00）
  6. bcrypt 哈希密码（cost=12），缓存到 Redis（`agent_register:pwd:{order_no}`）
  7. INSERT `AgentRegistrationOrder{OrderNo: REG+snowflake, PayStatus: pending, AgentID: nil}`
  8. `epay.BuildSubmitURL` 构造支付 URL
- 响应：
  ```json
  {
    "code": 0,
    "data": {
      "order_no": "REG1767225600000123",
      "pay_url": "https://pay.example.com/submit.php?pid=...&sign=...",
      "amount": 99.00,
      "message": "请在新页面完成支付，支付成功后代理账号将自动创建"
    }
  }
  ```

##### GET /api/v1/public/auth/agent/register/order/:order_no
- 鉴权：无（公开接口）
- 行为：查 `AgentRegistrationOrder` 表，返回订单状态 + 已支付时附 `agent_id`
- 响应：
  ```json
  {
    "code": 0,
    "data": {
      "order_no": "REG1767225600000123",
      "pay_status": "paid",
      "amount": 99.00,
      "username": "agent001",
      "created_at": "2026-07-20T10:00:00Z",
      "paid_at": "2026-07-20T10:01:30Z",
      "agent_id": 42
    }
  }
  ```

##### 支付回调处理（EpayNotify 内部分发）
- 触发：`POST /api/v1/pay/notify/epay`，验签通过 + Redis 防重入后调 `dispatchPaidOrder(notify)`
- 路由：按订单号前缀分发
  - `ORD*` → `processPaidOrder`（现有卡密购买流程，保持不变）
  - `REG*` → `processAgentRegisterPaid`（v0.3.6 新增）
- `processAgentRegisterPaid` 事务内：
  1. 校验订单状态/金额（防伪造）
  2. 幂等保护（已 paid 直接返回）
  3. 事务内重复 `quota` 校验防 TOCTOU（套餐上限 + 用户名重复）
  4. INSERT `Agent{Status: "active", CommissionRate: 邀请码.DefaultCommissionRate, CommissionMode: "percentage"}`
  5. 回填 `AgentRegistrationOrder.AgentID` + `PayStatus=paid` + `PaidAt` + `PayTradeNo`
  6. 邀请码 `used_count++`，达 `max_uses` 时 `status=exhausted` + 写 `used_by_agent_id`（补齐旧逻辑漏洞）
  7. 删除 Redis 中的密码哈希缓存（已用过，安全清理）
- 前端流程：3 步向导（填写邀请码 → 支付注册费 → 完成注册），路由 `/agent/register`（meta.public=true），新窗口跳转支付页面，用户支付后点「查询状态」按钮轮询订单

#### 双层支付模式切换（v0.3.6）

**设计原则**：通过 `SysPackage.AllowCustomPay` + `TenantPayConfig.Enabled` 双开关实现"平台总支付（默认）/ 开发者自有易支付（按套餐开通）"双层支付模式。订单号前缀区分业务通道，`dispatchPaidOrder` 集中分发。

**订单号前缀定义**：

| 前缀 | 业务通道 | 处理函数 | 抽成 | 资金流向 |
|---|---|---|---|---|
| `ORD` | 平台总支付卡密购买 | `processPaidOrder` | 平台按 `PlatformCommissionRate` 抽成，写 `PlatformSettlement` | 资金进平台易支付账户，开发者结算后从 `sys_tenant.balance` 提现 |
| `TOP` | 开发者自有易支付卡密购买 | `processTenantOwnPaidOrder` | 不抽成，平台通过套餐 `CustomPayFee` 月费模式收费 | 资金直接进开发者易支付账户，订单总额累加到 `sys_tenant.balance` |
| `REG` | 代理注册付费 | `processAgentRegisterPaid` | 不抽成，注册费归平台 | 资金进平台易支付账户 |

**双层切换逻辑**（`CreatePayOrder` 内）：
1. 查开发者套餐 `SysPackage.AllowCustomPay`
2. 查开发者 `TenantPayConfig(tenant_id, channel=epay, enabled=true)`
3. 双条件命中 → 走自有支付：订单号前缀 `TOP`，回调 URL 携带 `tenant_id`（`resolveTenantNotifyURL`）
4. 否则回退平台总支付：订单号前缀 `ORD`，需 `pay.platform.enabled=true`

##### POST /api/v1/pay/notify/tenant/:tenant_id
- 鉴权：无（公开回调，靠签名校验）
- 路径参数：`tenant_id` —— 开发者租户 ID
- 行为：
  1. 从 URL 取 `tenant_id`，调 `loadTenantPayConfig` 加载该租户 `TenantPayConfig(channel=epay, enabled=true)` + AES 解密 `key_encrypted`
  2. 收集回调参数（GET + POST 合并）+ `epay.VerifyNotify` 验签（用该租户密钥）
  3. `epay.ParseNotify` + `IsSuccess` 校验 `trade_status=TRADE_SUCCESS`
  4. Redis 防重入（key=`pay:notify:tenant:{tid}:lock:{order_no}`，TTL=60s，按 tenant_id 命名空间隔离）
  5. 调 `dispatchPaidOrder(notify)` 按订单号前缀分发
- 响应：`"success"` / `"fail"`（与平台回调一致）
- 安全：
  - 金额校验（订单 DB 中的 `total_amount` 与回调 `money` 字符串严格匹配，防伪造）
  - 幂等保护（已 paid 直接返回 success）
  - FOR UPDATE 锁开发者行防并发余额更新

##### processTenantOwnPaidOrder 事务流程
1. 查 `AppOrder` by `order_no`，校验金额 + 状态（pending → paid）
2. 查 `AppCardType`（自动发卡参数来源）
3. 事务内：
   - 更新订单 `pay_status=paid` + `pay_trade_no` + `paid_at`
   - 自动发卡 N 张（`batch_no` 前缀 `T` 区分），回填 `card_ids`
   - FOR UPDATE 锁 `sys_tenant`，`balance += total_amount`
   - 写 `TenantBalanceLog{type=settle, amount=total_amount, pay_method=tenant_epay, status=settled}`
4. **不写 `PlatformSettlement`**（资金已直接到开发者易支付账户，平台不抽成）

### 7.4 客户端 SDK 接入规范（v0.3.6）

三语言 SDK（Python / Node.js / PHP）已发布于 `sdks/` 目录，统一封装 9 个验证 API + HMAC-SHA512/256 签名。

#### 通用约定

| 项 | 值 |
|---|---|
| 客户端 API 前缀 | `/api/v1/client` |
| 签名算法 | HMAC-SHA512/256（与后端 `crypto.HMACSHA256` 的 `sha512.New512_256` 变体对齐） |
| 签名原文 | `METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY` |
| 请求头 | `X-App-Key` / `X-Timestamp` / `X-Nonce` / `X-Signature` |
| 签名输出 | 64 位小写十六进制字符串 |
| 回退策略 | 运行时不支持 `sha512/256` 时回退标准 `sha256`（**待核实**：与后端 sha512.New512_256 是否完全等价） |

#### 9 个 API 端点

| 方法 | 路径 | 必填参数 |
|---|---|---|
| `login` | `POST /api/v1/client/login` | `card_key` / `hwid` |
| `verify` | `POST /api/v1/client/verify` | `card_key` / `hwid` |
| `heartbeat` | `POST /api/v1/client/heartbeat` | `card_key` / `hwid` |
| `bind` | `POST /api/v1/client/bind` | `card_key` / `hwid` |
| `unbind` | `POST /api/v1/client/unbind` | `card_key` / `hwid` |
| `get_var` | `POST /api/v1/client/get_var` | `card_key` / `var_key` |
| `notice` | `POST /api/v1/client/notice` | — |
| `version` | `POST /api/v1/client/version` | — |
| `logout` | `POST /api/v1/client/logout` | `card_key` / `hwid` |

#### 各语言实现要点

##### Python SDK（`sdks/python/`，`keyauth-py` v0.3.6）
- 构造函数：`KeyAuthClient(api_base, app_key, sign_secret, timeout=10)`
- 签名函数：`_sha512_256_hex(key, msg)` 优先 `hashlib.new("sha512_256")`，不支持时回退 `hashlib.sha256`
- 依赖：`requests>=2.20`（唯一第三方依赖）
- Python 版本：`>=3.7`

##### Node.js SDK（`sdks/nodejs/`，`keyauth-node` v0.3.6）
- 构造函数：`new KeyAuthClient({ apiBase, appKey, signSecret, timeout })`
- 签名函数：`hmacSha512_256Hex(secret, msg)` 用 `crypto.createHmac('sha512/256', secret)`，不支持时回退 `sha256`
- 依赖：**无第三方依赖**（仅用 Node 内置 `https` / `crypto`）
- Node 版本：`>=14.0.0`
- TypeScript：提供 `index.d.ts` 完整类型定义

##### PHP SDK（`sdks/php/`，`keyauth/keyauth-php` v0.3.6）
- 构造函数：`new KeyAuthClient($apiBase, $appKey, $signSecret, $timeout=10)`
- 签名方法：`hmacSha512256($secret, $msg)` 用 `hash_hmac('sha512/256', $msg, $secret)`，不支持时回退 `hash_hmac('sha256', ...)`
- 依赖：**无第三方依赖**（仅依赖 `ext-curl` / `ext-json` / `ext-hash` PHP 标配扩展）
- PHP 版本：`>=7.2.0`（推荐 7.4+ 或 8.x）
- 自动加载：PSR-4 `KeyAuth\\` 命名空间
- 类型安全：`declare(strict_types=1)`

#### 错误处理

三语言 SDK 统一通过 `KeyAuthError` 异常传递错误，含三个字段：
- `message` —— 错误消息
- `code` / `errorCode` —— 业务错误码（如 2001/2002/2003）
- `httpStatus` / `http_status` —— HTTP 状态码（如 401/403/500）

#### 铁律合规

- **铁律 04**：SDK 不硬编码任何密钥/域名/AppKey，全部由开发者构造函数传入；README 推荐从环境变量读取
- **铁律 05**：SDK 内部无可调业务参数，路径前缀为常量
- **铁律 06**：签名算法回退分支已标注「待核实」；PHP SDK 通过 `php -l` 语法校验；运行时集成测试待 v0.4.x

### 7.4.1 多级代理体系规范（v0.4.0）

#### 数据模型

- `agent.parent_id BIGINT NOT NULL DEFAULT 0`：上级代理 ID（0 = 一级代理，由开发者邀请码注册）
- `agent.level TINYINT NOT NULL DEFAULT 1`：代理层级（1/2/3，上限由 `agent.commission.max_level` 控制）
- `agent_invite_code.creator_type VARCHAR(16) NOT NULL DEFAULT 'tenant'`：邀请码创建者类型（`tenant`=开发者 → 注册为一级 / `agent`=代理 → 注册为 creator.level+1）
- `agent_invite_code.creator_agent_id BIGINT NOT NULL DEFAULT 0`：creator_type='agent' 时填创建者代理 ID

#### 配置项（sys_config，可后台调整）

| Key | 默认值 | 含义 |
|---|---|---|
| `agent.commission.cross_level_2_rate` | `50.00` | 二级代理产生佣金时，父级（一级）分润比例（百分比） |
| `agent.commission.cross_level_3_rate` | `20.00` | 三级代理产生佣金时，祖父级（一级）分润比例（百分比） |
| `agent.commission.max_level` | `3` | 最大代理层级（1/2/3） |
| `agent.invite_code.agent_can_create` | `1` | 是否允许代理创建下级邀请码（0/1） |

#### 跨级佣金算法（`multilevel.DistributeCrossCommission`）

调用时机：`AgentGenerateCards` 计算出当前代理佣金 `commission` 后，在事务内调用。

```
若 agent.Level == 1 或 ParentID == 0 → 无跨级佣金（返回 nil）
向上遍历 parent_id 链，最多 2 层（depth=0/1）：
  depth=0 (parent=直接父级):
    - agent.Level=2 → rate = cross_level_2_rate
    - agent.Level=3 → rate = cross_level_2_rate  （父级 level=2 获此比例）
  depth=1 (parent=祖父级):
    - agent.Level=3 → rate = cross_level_3_rate  （祖父级 level=1 获此比例）
  parent.status != 'active' → break（停止向上）
  parent 已被删除 → break（停止向上）
  事务内：
    1. UPDATE agent SET balance = balance + (commission * rate / 100) WHERE id = parent.id
    2. 重新读取 parent.Balance 作为 BalanceAfter
    3. INSERT agent_balance_log (type='cross_commission', amount, balance_after, related_card_ids, status='settled')
  current = parent （继续向上）
```

#### 三端代理树查询 API

| 路由 | 角色 | 行为 |
|---|---|---|
| `GET /api/v1/admin/agents/:id/tree` | 平台超管 | 跨租户查询任意代理下级树（maxDepth = max_level - 1） |
| `GET /api/v1/tenant/agents/:id/tree` | 开发者 | 校验 agent 归属当前 tenant_id，构建下级树 |
| `GET /api/v1/agent/tree` | 代理 | 查询自己为根的下级树（maxDepth = max_level - 1） |
| `GET /api/v1/agent/subordinates` | 代理 | 查询直接下级（单层，parent_id = agentID AND tenant_id 匹配） |

#### 代理邀请码管理 API（agent 端）

| 路由 | 行为 |
|---|---|
| `POST /api/v1/agent/invite_codes` | 创建下级邀请码（`CanCreateSubordinate` 校验 + `quota.CheckMaxAgents` 配额校验 + `CreatorType='agent'` + `CreatorAgentID=agentID`） |
| `GET /api/v1/invite_codes` | 列出自己的下级邀请码（`creator_type='agent' AND creator_agent_id=agentID`） |
| `POST /api/v1/agent/invite_codes/:id/disable` | 禁用自己的邀请码（归属校验） |

#### 兼容性

- v0.3.x 老代理升级后 `parent_id=0` + `level=1`，行为等同一级代理（无跨级佣金）
- v0.3.x 老邀请码升级后 `creator_type='tenant'` + `creator_agent_id=0`，新代理仍注册为一级
- 跨级佣金流水类型 `cross_commission` 与既有 `commission` / `recharge` / `withdraw` 类型独立，互不干扰

### 7.4.2 灰度发布体系规范（v0.4.0）

#### 数据模型

- `app_version.release_strategy VARCHAR(32) NOT NULL DEFAULT 'full'`：发布策略（`full` / `grayscale` / `canary`）
- `app_version.grayscale_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00`：命中比例 0~100
- `app_version.grayscale_platforms VARCHAR(200)`：逗号分隔平台白名单（空=不限）
- `app_version.grayscale_regions VARCHAR(500)`：逗号分隔地区白名单（空=不限）
- `app_version.grayscale_channels VARCHAR(200)`：逗号分隔渠道白名单（空=默认 `stable`）
- 复合索引 `idx_app_status_strategy`（app_id, status, release_strategy）加速客户端灰度匹配

#### 配置项（sys_config，可后台调整）

| Key | 默认值 | 含义 |
|---|---|---|
| `app.version.grayscale.enabled` | `1` | 灰度全局开关（0=关闭后所有 grayscale/canary 策略回退到 full 行为） |
| `app.version.grayscale.default_rate` | `10.00` | 新建灰度版本未指定 rate 时的默认比例 |
| `app.version.grayscale.hash_salt` | `keyauth-grayscale-v040` | Hash 桶算法盐值，更换可全量重排灰度命中（用于紧急清退灰度） |

#### 匹配算法（`grayscale.Match`）

调用时机：`ClientVersion` 查询出所有 active 版本后，按 id DESC 遍历调用 `Match`，首个命中即返回。

```
7 步过滤链：
1. version == nil → 未命中（Reason="version_not_found"）
2. release_strategy == "full" → 命中（Reason="full_strategy"）
3. strategy in ("grayscale","canary") AND enabled=false → 命中（Reason="grayscale_disabled_fallback"）
4. grayscale_platforms 非空 AND client.Platform 不在 ParseList 列表（小写）→ 未命中（Reason="platform_filtered"）
5. ParseList(grayscale_channels) 非空时默认值="stable"；client.Channel 不在列表 → 未命中（Reason="channel_filtered"）
6. grayscale_regions 非空 AND client.Region 不在列表 → 未命中（Reason="region_filtered"）
7. 比例判定：
   - rate <= 0 → 未命中（Reason="rate_zero"）
   - rate >= 100 → 命中（Reason="rate_full"）
   - 0 < rate < 100 → HashBucket(salt, appID, clientID) < rate → 命中（Reason="bucket_hit"）；否则未命中（Reason="bucket_miss"）
```

#### Hash 桶算法（`grayscale.HashBucket`）

```
input: salt (string) + appID (uint64) + clientID (string)
hash := SHA-256(salt + ":" + strconv.FormatUint(appID, 10) + ":" + clientID)
取 hash 前 4 字节 little-endian uint32
return uint32 % 100   // 0~99 稳定桶号
```

特性：
- **稳定性**：相同 (salt, appID, clientID) 永远返回相同桶号
- **均匀性**：SHA-256 输出在 0~99 范围内近似均匀分布
- **可重排**：更换 salt 即可全量重排所有客户端的桶号（紧急清退灰度场景）
- **可隔离**：appID 参与哈希保证不同应用桶号互不影响

#### 客户端 API（`POST /api/v1/client/version`）

请求扩展字段（v0.3.x 客户端可省略，回退到 client_ip 作为 ClientID）：

| 字段 | 用途 |
|---|---|
| `hwid` | 硬件指纹（首选 ClientID） |
| `device_id` | 设备 ID（次选 ClientID） |
| `platform` | 客户端平台（如 `windows` / `linux` / `macos` / `android` / `ios`） |
| `channel` | 发布渠道（默认 `stable`，可传 `beta` / `internal` 等） |
| `region` | 地区码（如 `cn` / `us` / `eu`） |

响应扩展字段（仅 grayscale/canary 策略命中时返回）：

| 字段 | 含义 |
|---|---|
| `release_strategy` | 命中的发布策略（grayscale/canary） |
| `grayscale_hit` | 是否灰度命中（true） |
| `grayscale_bucket` | 客户端所在桶号（0~99） |
| `grayscale_rate` | 命中版本的灰度比例 |

#### 管理端 API

| 路由 | 角色 | 行为 |
|---|---|---|
| `GET /api/v1/admin/versions` | 平台超管 | 跨租户查询版本列表（JOIN sys_tenant + app，支持 tenant_id/app_id/channel/release_strategy 筛选） |
| `GET /api/v1/admin/versions/:id` | 平台超管 | 跨租户查询单版本详情 |
| `POST /api/v1/tenant/versions` | 开发者 | 创建版本（grayscale/canary 策略 + rate=0 时自动取 `DefaultRate`） |
| `PUT /api/v1/tenant/versions/:id` | 开发者 | 编辑版本灰度规则（归属校验 + 指针字段可选更新 + 切换到灰度策略 + rate=0 时取 `DefaultRate`） |

#### 兼容性

- v0.3.x 老版本升级后 `release_strategy='full'` + `grayscale_rate=0`，行为等同原「最新 active 版本一刀切」
- v0.3.x 客户端不传 `hwid`/`device_id` 时，`ClientVersion` 回退到 client_ip 作为 ClientID 桶号
- 全局开关 `app.version.grayscale.enabled=0` 时所有 grayscale/canary 版本回退 full 命中（紧急关停灰度）
- 更换 `hash_salt` 可全量重排灰度命中（紧急清退/放量场景）

### 7.4.3 在线更新体系规范（v0.4.0）

#### 数据模型

- `system_update_log` 表（migration 011）：
  - `trigger_source VARCHAR(32)`：触发源（`webhook` / `manual` / `rollback`）
  - `trigger_by BIGINT`：触发者 admin id（webhook 时为 0）
  - `trigger_ip VARCHAR(45)`：触发者 IP
  - `commit_before` / `commit_after VARCHAR(64)`：更新前后 commit hash
  - `branch VARCHAR(64)`：目标分支
  - `status VARCHAR(32)`：状态（`pending` / `running` / `success` / `failed` / `rolled_back`）
  - `steps_json TEXT`：步骤 JSON 数组 `[{step,status,duration_ms,output,error}]`
  - `log_text MEDIUMTEXT`：人类可读完整日志
  - `error_message VARCHAR(512)`：失败原因摘要
  - `duration_ms INT`：总耗时
  - `rolled_back_from BIGINT`：若为回滚，原失败更新 id（0=非回滚）
- 3 个索引：`idx_update_log_status` / `idx_update_log_created` / `idx_update_log_trigger`

#### 配置项（sys_config 8 项，可后台调整）

| Key | 默认值 | 含义 |
|---|---|---|
| `update.webhook.secret` | `` | GitHub Webhook HMAC-SHA256 密钥（X-Hub-Signature-256 头），空=不校验仅本地开发 |
| `update.webhook.branch` | `main` | 监听分支（仅 push 到此分支触发更新） |
| `update.webhook.auto_update` | `0` | 1=自动触发更新；0=仅记录通知，需管理员手动触发 |
| `update.deploy.script_path` | `scripts/deploy_update.sh` | 部署脚本相对项目根目录的路径 |
| `update.healthcheck.url` | `http://localhost:8080/health` | 更新后健康检查 URL（2xx/3xx 视为成功） |
| `update.healthcheck.timeout` | `30` | 健康检查超时秒数 |
| `update.rollback.enabled` | `1` | 失败自动回滚开关 |
| `update.lock.timeout` | `600` | 更新锁超时秒数（防死锁自动释放） |

#### Webhook 签名校验（`update.VerifyWebhookSignature`）

GitHub 算法：`HMAC-SHA256(secret, body)` → hex 编码 → 前缀 `sha256=`

校验规则：
- 空 secret 时跳过校验（仅本地开发；生产必须配置非空 secret）
- 空 signature 时拒绝
- signature 必须以 `sha256=` 前缀开头
- 用 `hmac.Equal` 防时序攻击（非常量时间比较）

#### 更新流程（`Manager.ExecuteUpdate`）

```
6 步流程：
1. 加锁（进程内 mutex + Redis SET NX EX 双重锁）
   - 已锁 → 返回 "update locked" 错误
   - Redis 锁超时由 update.lock.timeout 控制（默认 600s）
2. 创建 pending 审计日志（trigger_source / trigger_by / trigger_ip / commit_before / branch）
3. git fetch origin <branch> + git reset --hard origin/<branch>
   - 命令显式组合：exec.Command("git", "fetch", "origin", branch)
   - 禁止 shell 拼接用户输入
4. 跑部署脚本：bash <script_path>
   - 路径从 sys_config(update.deploy.script_path) 读取
   - 默认 scripts/deploy_update.sh
5. 健康检查：HTTP GET <healthcheck.url>
   - 2xx/3xx 视为成功（CheckRedirect 禁用跟随以捕获原始 3xx）
   - 超时由 update.healthcheck.timeout 控制
6. 失败处理：若 update.rollback.enabled=1 → 调用 maybeRollback
   - git reset --hard <commit_before>
   - 重跑部署脚本
   - 健康检查
   - 写入 rolled_back 状态 + 独立回滚审计日志（rolled_back_from 关联原失败日志 id）
```

#### 部署脚本（`scripts/deploy_update.sh`）

通过 `DEPLOY_MODE` 环境变量适配不同部署环境：

| DEPLOY_MODE | 重启方式 |
|---|---|
| `systemd` | `sudo systemctl restart keyauth-server` |
| `docker` | `docker-compose restart keyauth-server` |
| `pm2` | `pm2 restart keyauth-server` |
| `none` | 不重启（假设外部监管进程自动拉起新二进制） |

脚本严格 `set -euo pipefail` + 显式 `cd` 项目根；失败时退出码非 0，触发 `Manager` 回滚流程。

#### API 接口

| 路由 | 鉴权 | 行为 |
|---|---|---|
| `POST /api/v1/public/update/webhook` | HMAC 签名 | GitHub Webhook 接收：签名校验 + push event 解析 + 分支匹配 + 自动/手动触发 |
| `GET /api/v1/admin/update/status` | admin JWT | 当前 commit + 锁状态 + 自动开关 + 分支 + 最近审计日志 + 成功/失败统计 |
| `POST /api/v1/admin/update/trigger` | admin JWT | 手动触发更新（异步执行，立即返回） |
| `GET /api/v1/admin/update/history` | admin JWT | 分页查询审计日志（status / trigger_source 筛选） |
| `POST /api/v1/admin/update/rollback` | admin JWT | 手动回滚到指定失败日志的 commit_before |
| `GET /api/v1/admin/update/logs/:id` | admin JWT | 单条审计日志详情（含完整 log_text） |

#### 安全机制

- Webhook 端点无鉴权但强制 HMAC-SHA256 签名校验
- 管理后台 5 个接口仅 `admin` 角色可访问（JWTAuth 中间件）
- 所有更新操作写 `system_update_log` 审计日志
- shell 命令显式组合参数（`exec.Command` 不走 shell），禁止 eval/exec 任意用户输入
- 部署脚本路径从 sys_config 读取，仅 root/admin 可后台修改

#### 可靠性保障

- 双重锁（进程内 mutex + Redis SET NX EX）防并发触发
- 锁超时 600s 自动释放（防死锁）
- 失败自动回滚到 `commit_before`（`git reset --hard` + 重跑脚本 + 健康检查）
- 健康检查通过后才标记 success
- 完整步骤日志（`steps_json` JSON 数组 + `log_text` 人类可读文本）

### 7.5 测试规范（v0.3.6）

#### 测试栈

| 用途 | 库 | 版本 |
|---|---|---|
| 断言 / require | `github.com/stretchr/testify` | v1.11.1 |
| 内存 Redis | `github.com/alicebob/miniredis/v2` | v2.38.0 |
| SQLite 内存库 | `gorm.io/driver/sqlite` + `github.com/mattn/go-sqlite3` | v1.6.0 + v1.14.22 |

#### 测试覆盖（13 个包，0 失败）

| 包 | 测试文件 | 覆盖范围 |
|---|---|---|
| `pkg/crypto` | `crypto_test.go` | AES / HMAC（含 sha512/256 vs sha256 区分） / bcrypt / SHA-512 / MD5 / 易支付签名 / 卡密生成 / HWID |
| `pkg/crypto` | `sign_alignment_test.go` | 跨语言签名对齐（Python / Node.js / PHP / Go / Java / C++ / C# vs 后端 `HMACSHA256`，易语言 Windows-only 永久 skip）+ v0.4.0 5 个新 SDK 目录结构元数据校验 |
| `pkg/snowflake` | `snowflake_test.go` | `NewNode` 边界 / `NextID` 并发安全 / `OrderNo` 三通道前缀 / `twepoch` 常量 |
| `pkg/epay` | `epay_test.go` | `BuildSubmitURL` / `ParseNotify` / `VerifyNotify` / 端到端闭环 |
| `pkg/ua` | `ua_test.go` | UA 解析：Chrome/Firefox/Safari/Edge/curl/Bot/空字符串 + OS 版本号提取 + 设备类型 + 优先级匹配（20 个测试） |
| `internal/quota` | `quota_test.go` | `CheckMaxApps/Cards/Agents/Devices` 全场景 + `ExceededError` 类型匹配 |
| `internal/heartbeat` | `heartbeat_test.go` | `Record/IsOnline/Remove/CountOnline/ListOnline/GetLastHeartbeatAt` 全场景 + 端到端闭环 |
| `internal/middleware` | `middleware_test.go` | JWT 鉴权（含 v0.4.0 JTI 注入上下文测试） / TenantScope 租户隔离 / SignatureAuth HMAC 签名闭环 / RateLimitByIP 滑动窗口 / IPBlacklist / RecordCardFailure 自动封禁 / Response 格式（22 个测试） |
| `internal/auth` | `jwt_test.go` | v0.4.0 JTI 精准单点踢出：GenerateTokenPair 写入 JTI / BlacklistRefreshTokenByJTI 隔离性 / 同一用户不同设备互不影响 / IsRefreshTokenBlacklisted 兼容旧 token 回退 user 维度 / TTL 过期 / JTI 黑名单端到端闭环（登录两设备 → 踢一设备 → 另一设备不受影响 → 改密强制全部重登）/ ExtractBearer 5 子用例（18 个测试） |
| `internal/logger` | `logger_test.go` | v0.4.0 slog 结构化日志：parseLevel 4 级别 + 大小写 + 默认值 / JSON 格式 level/msg/字段断言 / level 过滤（warn 时不输出 debug/info）/ text 格式 msg 含空格自动加引号 / L() 非 nil / 空 Options 不 panic（6 个测试） |
| `internal/handler` | `profile_2fa_test.go` | v0.4.0 2FA 备用码 DB 持久化：loadUserBackupCodes DB 读取 + Redis 回退 + 用户不存在 + tenant/agent role 分支 + 不支持角色 / updateUserBackupCodes 清空 / consumeBackupCode 消费成功 + 消费最后一个 + 输入不匹配 + 空输入 + 无备用码 + 从 Redis 回退消费 / twoFABackupKey + twoFASetupKey 格式（13 个测试） |
| `internal/multilevel` | `multilevel_test.go` | v0.4.0 多级代理：DistributeCrossCommission（level 1/2/3 + 父级禁用跳过 + 零/负佣金 + nil agent + 自定义比例，7 个）/ CanCreateSubordinate（level vs max_level 矩阵 + agent_can_create flag + 禁用 + nil，6 个）/ ComputeSubordinateLevel（tenant 邀请码 → 1 / agent 邀请码 → 2 / level 3 超限 / 创建者不存在 / nil，5 个）/ BuildAgentTree（三级树 + maxDepth=0 + 不存在 + 租户隔离，4 个）/ ListSubordinates（单层 + 无子级 + 租户隔离，3 个）/ 边界（parent 链断裂，2 个）（27 个测试） |
| `internal/grayscale` | `grayscale_test.go` | v0.4.0 灰度发布：Match full 策略（3）+ 全局开关（1）+ 平台过滤（4）+ 渠道过滤（3）+ 地区过滤（3）+ 比例（4）+ HashBucket 稳定性/范围/salt/appID（4）+ ParseList 空/单/多/空格/大小写/仅逗号（6）+ DefaultRate/IsEnabled（4）+ 边界匿名 clientID/canary/多过滤全过/多过滤失败（4）（33 个测试） |
| `internal/update` | `update_test.go` | v0.4.0 在线更新：VerifyWebhookSignature（7：有效/错误 secret/空 secret 跳过/空签名拒绝/错误前缀/篡改 body/空 body）+ ParsePushEvent（4：有效/非法 JSON/空 ref/缺失 ref）+ BranchMatches（5：短形式/完整形式/不匹配/空分支/tag ref）+ AcquireLock/ReleaseLock（5：首次成功/二次失败/释放后重新获取/Redis key SET/DEL/多 Manager 共享 lockKey 互斥）+ HealthCheck（6：2xx/3xx 禁用重定向/5xx/4xx/连接拒绝/超时尊重）+ 状态机常量（4：TriggerSource/Status/StepStatus/8 个 ConfigKey 互不冲突 + 全部 update. 前缀）+ IsAutoUpdateEnabled/IsLocked（4：默认 false/true/未锁/已锁）+ 边界（6：大 body 10KB/额外字段忽略/分支名特殊字符/不同 lockKey/多次校验一致性/JSON round-trip）+ 并发压力（1：10 goroutine 抢锁无 panic 无死锁）（37 个测试） |

#### 测试原则

1. **不依赖外部服务**：MySQL → SQLite 内存库；Redis → miniredis；HTTP → `httptest.NewRecorder` + `gin.TestMode` 不启真实端口
2. **铁律 06（防幻觉）合规**：所有断言基于已知固定输入，无随机/不确定性；Node.js 沙箱环境不支持 `sha512/256` 时 `t.Skipf` 标注「环境限制」，不掩盖
3. **跨语言签名对齐测试**：脚本位于 `sdks/tests/sign.{py,js,php}`，CLI 接收 `<secret> <msg>` 输出 hex；Go 测试通过 `exec.Command` 调用对比；运行时缺失或环境限制自动跳过
4. **gorm default 值陷阱**：测试 `MaxApps=0` / `MaxCards=0`（不限）场景时，必须 Create 后用 `Updates(map[string]interface{})` 强制覆盖 gorm `default:` 标签
5. **miniredis FastForward 限制**：`mr.FastForward` 不影响 Go `time.Now()`，需用 `rdb.ZAdd` 直接覆写 score 模拟心跳超时
6. **miniredis Close 后 Addr() panic**：测试 Redis 故障 fail-open 场景时，不能用 `mr.Close()` 后调用 `mr.Addr()`，需直接构造指向不可达地址（如 `127.0.0.1:1`）的 `redis.Client`
7. **ConfigReader mock**：中间件测试用 `mockConfigReader`（内存 map）实现 `ConfigReader` 接口，避免依赖 sys_config 表
8. **CryptoManager 注入**：`SetCryptoManager` 在测试 setup 时注入测试 AES 密钥（32 字节），`t.Cleanup` 恢复 nil
9. **UA 解析纯函数测试**：`pkg/ua` 无外部依赖，纯函数测试基于固定 UA 字符串断言；iOS/macOS UA 用 `_` 分隔版本号，`cleanVersion` 必须允许 `_` 通过再由 `parseOS` 转换为 `.`；浏览器匹配顺序 Edge → curl → Bot → Firefox → Chrome → Safari（避免 Edge UA 含 Chrome/ 被误识别为 Chrome）
10. **JWT jti 黑名单测试（v0.4.0）**：`internal/auth/jwt_test.go` 使用 miniredis 验证 `BlacklistRefreshTokenByJTI` 隔离性（不同 jti 互不影响）+ `IsRefreshTokenBlacklisted` 兼容旧 token（无 jti 时回退 user 维度）+ TTL 过期（`mr.FastForward` 推进 Redis 时间，不影响 Go `time.Now()`）；端到端测试覆盖「登录两设备 → 踢一设备 → 另一设备不受影响 → 修改密码强制全部重登」核心业务语义。中间件 `TestJWTAuth_JTI注入上下文` 用 `httptest.NewRecorder` 验证 `c.Set("jti", claims.ID)` 注入正确
11. **2FA 备用码 DB 持久化测试（v0.4.0）**：`internal/handler/profile_2fa_test.go` 用 SQLite 内存库 + miniredis + 真实 AES-256 crypto.Manager 测试 `loadUserBackupCodes` / `updateUserBackupCodes` / `consumeBackupCode`；关键场景：DB 读取 + Redis 回退（v0.3.x 老用户兼容）+ 消费成功后 DB 回写 + Redis 自动清理 + 输入不匹配不修改 DB + 消费最后一个时 DB 写入空字符串 + 3 角色（admin/tenant/agent）分支覆盖
12. **结构化日志测试（v0.4.0）**：`internal/logger/logger_test.go` 用 `bytes.Buffer` 临时替换全局 logger 输出验证 JSON / text 格式；`atomic.Value` 保证并发安全切换；level 过滤测试验证 `level=warn` 时 debug/info 不输出；`TestInit_DefaultFallback` 验证空 Options 不 panic（保证 Init 容错性）
13. **全语言 SDK 签名对齐测试（v0.4.0）**：`pkg/crypto/sign_alignment_test.go` 从 3 语言扩展到 7 语言（新增 Go / Java / C++ / C# + 易语言 Windows-only Skip）；解释器模式（Python/Node/PHP/Go）+ 编译型模式（C++ 用 g++ 编译到 t.TempDir() + 运行）+ Java 单文件源码模式（JDK 11+）+ C# dotnet 临时项目模式；运行时缺失自动 `t.Skip` 不强制依赖；`javaSupportsSHA512_256` 检测 JDK 版本，仅 JDK 17+ 强断言签名匹配（JDK < 17 回退 HmacSHA256 时仅 t.Logf 提示，不掩盖差异）；`TestSignAlignment_NewLanguages` 校验 5 个新 SDK 的目录结构完整性（不依赖运行时，CI 友好）
14. **多级代理测试（v0.4.0）**：`internal/multilevel/multilevel_test.go` 用 SQLite in-memory（4 表 AutoMigrate：agent/agent_invite_code/agent_balance_log/sys_config）+ miniredis + 真实 `ConfigCache`（预置 sys_config 4 项 + overrides 覆盖）；关键场景：DistributeCrossCommission 沿 parent_id 链向上分润（level 1 无父级 / level 2 父级 50% / level 3 父级 + 祖父级 / 父级禁用跳过 / 零/负佣金 / nil agent / 自定义比例）；CanCreateSubordinate（level vs max_level 矩阵 + agent_can_create flag + 禁用 + nil）；ComputeSubordinateLevel（tenant 邀请码 → 1 / agent 邀请码 → 2 / level 3 创建者超限 / 创建者不存在 / nil）；BuildAgentTree（三级树 + maxDepth=0 + 不存在 + 租户隔离）；ListSubordinates（单层 + 无子级 + 租户隔离）；边界场景（parent 链断裂：parent_id 指向已删除代理时停止向上）；测试修复了一个真实算法 bug（基于 `current.Level` 误判比例 → 改为基于 `agent.Level` + depth 判断）
15. **灰度发布测试（v0.4.0）**：`internal/grayscale/grayscale_test.go` 用 SQLite in-memory（app_version + sys_config AutoMigrate）+ miniredis + 真实 `ConfigCache`（预置 sys_config 3 项 + overrides 覆盖）；关键场景：Match 7 步过滤链全路径覆盖（full 策略 + 全局开关回退 + 平台/渠道/地区过滤 + 比例 0/100/部分桶命中/未命中）；HashBucket 稳定性（同输入同输出 + 范围 0-99 + 不同 salt 不同桶 + 不同 appID 不同桶）；ParseList 边界（空 / 单 / 多 / 含空格 / 混合大小写 / 仅逗号返回空非 nil）；DefaultRate / IsEnabled 配置读取 + fallback；边界场景（匿名 clientID 走 client_ip + canary 策略 + 多过滤全过 + 多过滤一过滤失败）；测试栈无网络/无文件 IO，纯函数 + 内存 DB
16. **在线更新测试（v0.4.0）**：`internal/update/update_test.go` 用 SQLite in-memory（system_update_log + sys_config AutoMigrate）+ miniredis + 真实 `ConfigCache`（预置 8 项 sys_config + overrides 覆盖）+ `httptest.Server` 模拟健康检查端点；关键场景：VerifyWebhookSignature 7 路径（有效签名 + 错误 secret + 空 secret 跳过 + 空签名拒绝 + 错误前缀拒绝 + 篡改 body 拒绝 + 空 body 边界）；ParsePushEvent（有效 + 非法 JSON + 空 ref + 缺失 ref）；BranchMatches（短/完整形式 + 不匹配 + 空分支 + tag ref 不匹配）；AcquireLock/ReleaseLock（首次成功 + 二次失败 + 释放后重新获取 + Redis key SET/DEL 验证 + 多 Manager 共享 lockKey 互斥）；HealthCheck（2xx 成功 + 3xx 成功禁用重定向 + 5xx 失败 + 4xx 失败 + 连接拒绝 + 超时尊重 1s<2s）；状态机常量（TriggerSource/Status/StepStatus/8 个 ConfigKey 互不冲突 + 全部 `update.` 前缀）；IsAutoUpdateEnabled/IsLocked（默认 false/true/未锁/已锁）；边界（大 body 10KB + 额外字段忽略 + 分支名特殊字符 + 不同 lockKey 互不影响 + 多次校验一致性 + PushEvent JSON round-trip）；并发压力（10 goroutine 抢同一锁无 panic 无死锁）；测试修复了一个真实 bug（Go HTTP client 默认跟随重定向导致 3xx 测试失败 → 改用 CheckRedirect 返回 http.ErrUseLastResponse 捕获原始状态码）

#### 运行命令

```bash
cd apps/server
go test ./...                    # 全部测试
go test -v ./pkg/crypto/ -run TestSignAlignment  # 仅签名对齐
go vet ./...                     # 静态检查
go build ./...                   # 编译验证
```

### 7.6 慢查询监控

- 慢查询阈值：200ms
- 慢查询日志：单独文件，便于分析
- EXPLAIN 分析：每周检查 TOP 10 慢查询

---

## 8. 部署规范

### 8.1 环境要求

- OS：Linux (Ubuntu 22.04+ / CentOS 8+)
- Docker：24.0+
- Docker Compose：2.20+
- MySQL：8.0+
- Redis：7.0+
- Nginx：1.22+

### 8.2 Docker 镜像规范

- 基于 `alpine` 或 `distroless` 精简镜像
- 多阶段构建
- 非 root 用户运行
- 健康检查 `HEALTHCHECK`
- 镜像标签：`keyauth-api:v0.2.0`、`keyauth-api:latest`

```dockerfile
# 多阶段构建示例
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /keyauth-api ./cmd

FROM alpine:3.19
RUN adduser -D -u 1000 keyauth
WORKDIR /app
COPY --from=builder /keyauth-api .
USER keyauth
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -q -O- http://localhost:8080/health || exit 1
ENTRYPOINT ["./keyauth-api"]
```

### 8.3 数据库迁移

- 自研轻量级 SQL 文件迁移机制（`internal/migration/migrator.go`，v0.3.5）
- `schema_migrations` 表跟踪版本号 + dirty 状态
- 扫描 `*.up.sql` 文件，按文件名前缀数字排序（001 ~ 007）
- 每个迁移在独立事务中执行，失败标记 dirty 阻止启动
- 幂等：已应用的迁移不会重复执行
- DSN 加 `multiStatements=true` 参数（迁移文件含多语句）
- 启动时 `InitContainer` 调用 `migration.Run(db, cfg.Migration.Dir)` 自动执行
- 配置：`MIGRATION_AUTO` / `MIGRATION_DIR` 环境变量覆盖
- 禁止在生产环境手动改表

---

## 9. 文档规范

### 9.1 四份核心文档

详见 `web-project-flow` skill 的 `references/09-docs-lifecycle.md`（已全局安装，可用 `/bdocs` 触发）

| 文档 | 用途 | 更新时机 |
|---|---|---|
| CHANGELOG.md | 更新日志 | 每次发布 |
| PROJECT.md | 项目文档 | 架构变更 |
| SPEC.md | 规范文档 | 规则变更 |
| TODO.md | 待完成文档 | 任务调整 |

**联动校验铁律**：
- TODO 中标记完成的条目必须出现在对应版本的 CHANGELOG 中
- PROJECT 中描述的功能必须与 SPEC 中的规范一致
- 已移除功能必须同时从 PROJECT 中删除并在 CHANGELOG 中记录

### 9.2 API 文档

- OpenAPI 3.0 规范
- 自动生成
- 示例齐全
- 错误码完整

### 9.3 SDK 文档

每个语言 SDK 必须包含：
- 快速开始
- API 参考
- 示例代码
- 常见问题
- 更新日志

---

**文档版本**：0.3.6  
**最后更新**：2026-07-20  
**维护者**：KeyAuth SaaS Team
