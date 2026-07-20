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

### 7.5 慢查询监控

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
