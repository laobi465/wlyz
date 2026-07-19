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

严格遵循 **4 层架构**，禁止跨层调用：

```
┌─────────────────────────────────────┐
│ Handler 层（HTTP 处理器）            │
│ - 路由匹配、参数校验、响应封装       │
│ - 不含业务逻辑                       │
└──────────────┬──────────────────────┘
               │ 调用
┌──────────────▼──────────────────────┐
│ Service 层（业务逻辑）               │
│ - 业务规则、事务管理                 │
│ - 调用 Repository                    │
└──────────────┬──────────────────────┘
               │ 调用
┌──────────────▼──────────────────────┐
│ Repository 层（数据访问）            │
│ - CRUD、查询                         │
│ - 不含业务逻辑                       │
└──────────────┬──────────────────────┘
               │ 调用
┌──────────────▼──────────────────────┐
│ Model 层（数据模型）                 │
│ - 结构体定义、字段映射               │
└─────────────────────────────────────┘
```

**禁止行为**：
- ❌ Handler 直接操作数据库
- ❌ Repository 包含业务逻辑
- ❌ Service 直接返回 HTTP 响应
- ❌ Model 包含方法（仅纯数据结构）

### 2.2 模块边界

每个业务模块独立目录，互不直接依赖：

```
internal/service/
├── auth/           # 认证模块（不依赖其他业务模块）
├── tenant/         # 租户模块（依赖 auth）
├── app/            # 应用模块（依赖 tenant）
├── card/           # 卡密模块（依赖 app）
├── device/         # 设备模块（依赖 card）
├── verify/         # 验证模块（依赖 card, device）
├── pay/            # 支付模块（依赖 tenant, order）
├── agent/          # 代理模块（依赖 tenant, card）
├── notice/         # 公告模块（独立）
└── stats/          # 统计模块（依赖所有模块，只读）
```

**跨模块通信**：
- 通过接口依赖注入
- 通过事件总线（异步场景）
- 禁止直接 import 其他模块的内部实现

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

### 7.3 慢查询监控

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

- 使用 `golang-migrate/migrate`
- 迁移文件命名：`{version}_{description}.up.sql` / `.down.sql`
- 启动时自动迁移
- 禁止在生产环境手动改表

---

## 9. 文档规范

### 9.1 四份核心文档

详见 [references/09-docs-lifecycle.md](../../web-project-flow/references/09-docs-lifecycle.md)

| 文档 | 用途 | 更新时机 |
|---|---|---|
| CHANGELOG.md | 更新日志 | 每次发布 |
| PROJECT.md | 项目文档 | 架构变更 |
| SPEC.md | 规范文档 | 规则变更 |
| TODO.md | 待完成文档 | 任务调整 |

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

**文档版本**：0.1.0  
**最后更新**：2026-07-19  
**维护者**：KeyAuth SaaS Team
