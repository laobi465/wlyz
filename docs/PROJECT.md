# 项目文档 (PROJECT)

## 1. 项目概述

### 1.1 项目名称
**KeyAuth SaaS** —— 面向开发者的多租户卡密验证平台

### 1.2 项目目标
为开发者提供自托管 SaaS 卡密验证基础设施，一次部署即可服务多个软件应用，支持多租户隔离、一机一卡密、在线验证、心跳保活、代理分销、自动发卡等完整能力。

### 1.3 项目背景
参考卡密通、T3 网络验证、布丁卡密三家之长，结合 License-SaaS 开源项目的技术栈选型，打造完全开源自托管、数据自主可控的卡密验证 SaaS 平台。

### 1.4 适用范围
- 独立开发者为自己开发的软件加授权
- 软件工作室商业化运营软件
- 中小型 SaaS 团队多应用统一授权管理
- 卡密分销代理商体系运营

### 1.5 对标产品

| 产品 | 借鉴点 |
|---|---|
| [卡密通 keyt.cn](https://www.keyt.cn/) | 极简三步接入、应用隔离、心跳保活、数据看板 |
| [T3 网络验证 t3yanzheng.com](https://t3yanzheng.com/) | 双向加密传输、独立接口链接、多级代理体系 |
| [布丁卡密 wmxwz.cn](https://wmxwz.cn/) | 硬件指纹双模式、七层纵深防御、SHA-512 校验位防伪 |
| [License-SaaS 开源项目](https://bbs.ijingyi.com/forum.php?mod=viewthread&tid=14876646) | Go + Vue3 技术栈、单端口部署、HMAC-SHA256 签名协议、云变量机制 |

### 1.6 差异化优势
- 完全开源自托管，数据自主可控
- 多应用 + 多租户架构
- 一机一卡密强绑定（CPU+主板+MAC+磁盘多重哈希）
- 双层支付模式（平台总支付 / 开发者自有易支付）
- 多级代理分销体系
- 心跳保活 + 离线宽限期
- 云变量远程下发
- 多语言 SDK（Python / Node.js / Java / C# / Go / PHP / C++ / 易语言）
- Docker 一键部署（同时支持宝塔 Docker + 原生 Docker Compose）

---

## 2. 架构总览

### 2.1 系统架构图

```
                    ┌─────────────────────────────────┐
                    │   Cloudflare CDN + WAF + SSL    │
                    └────────────┬────────────────────┘
                                 │
                    ┌────────────▼────────────────────┐
                    │   Nginx (反向代理 + 限流)        │
                    └────────────┬────────────────────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                        │
┌───────▼────────┐    ┌──────────▼──────────┐   ┌─────────▼─────────┐
│  Vue3 管理后台  │    │  Go Gin API 服务    │   │  H5 移动端         │
│  (超管/开发者/  │    │  (RESTful + HMAC)   │   │  (终端用户)        │
│   代理自适应)   │    │                     │   │                    │
└────────────────┘    └──────────┬──────────┘   └────────────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
       ┌──────▼──────┐    ┌──────▼──────┐   ┌──────▼──────┐
       │  MySQL 8.0  │    │  Redis 7    │   │  MinIO/OSS  │
       │  (主从可选) │    │  (缓存/限流)│   │  (对象存储) │
       └─────────────┘    └─────────────┘   └─────────────┘
```

### 2.2 技术栈

| 层级 | 选型 |
|---|---|
| 前端 | Vue3 + TypeScript + Element Plus + Vite + Pinia |
| 后端 | Go 1.22 + Gin + GORM |
| 数据库 | MySQL 8.0 + Redis 7 |
| 部署 | Docker Compose + 宝塔面板 Docker |
| 反代 | Nginx + Let's Encrypt |
| 监控 | Prometheus + Grafana（可选） |
| 日志 | Loki + Promtail（可选） |

### 2.3 核心模块依赖关系

```
认证模块 ──► 租户模块 ──► 应用模块 ──► 卡密模块
                │              │            │
                ▼              ▼            ▼
            套餐模块      设备模块      订单模块
                │              │            │
                ▼              ▼            ▼
            支付模块      验证日志      代理模块
                │                          │
                ▼                          ▼
            通知模块                  佣金结算
```

### 2.4 数据流（核心验证流程）

```
客户端启动
  │
  ▼
SDK 调用 /api/v1/login（卡密 + 机器码 + 签名）
  │
  ▼
Gin 接收请求 → 中间件验签 → 限流检查 → 租户路由
  │
  ▼
查 Redis 缓存 → 命中返回 → 未命中查 MySQL
  │
  ▼
校验卡密状态 → 校验设备绑定 → 写入/更新设备表
  │
  ▼
写入验证日志 → 返回带 RSA 签名的响应
  │
  ▼
SDK 校验签名 → 通过则解锁功能
  │
  ▼
后台定时心跳 /api/v1/heartbeat（保活）
```

---

## 3. 功能清单

### 3.1 平台超管后台（17 个模块）

| 编号 | 模块 | 已实现 | 说明 |
|---|---|---|---|
| S-01 | 平台看板 | ☐ | 全局统计、趋势图 |
| S-02 | 租户管理 | ☐ | 开发者 CRUD、审核、套餐分配 |
| S-03 | 套餐管理 | ☐ | 套餐 CRUD、应用数/卡密数/代理数上限、支付权限、抽成比例 |
| S-04 | 应用审核 | ☐ | 应用上架审核、违规下架 |
| S-05 | 代理全局视图 | ☐ | 跨租户代理统计 |
| S-06 | 平台总支付配置 | ☐ | 易支付网关/商户号/密钥、抽成比例、结算周期 |
| S-07 | 系统配置 | ☐ | sys_config 全局参数、代理注册配置 |
| S-08 | 通知模板 | ☐ | 短信/邮件/站内信模板 |
| S-09 | 安全防护 | ☐ | 全局 IP 黑名单、CC 防护、敏感词 |
| S-10 | 操作日志 | ☐ | 全平台操作审计 |
| S-11 | 系统监控 | ☐ | 服务器状态、QPS、慢查询 |
| S-12 | 数据备份 | ☐ | 数据库备份、恢复、导出 |
| S-13 | 更新管理 | ☐ | 在线更新、版本回滚 |
| S-14 | 管理员管理 | ☐ | 超管账号、角色权限、2FA |
| S-15 | 平台总公告管理 | ☐ | 公告 CRUD、置顶、强制弹窗 |
| S-16 | 开发者公告管理 | ☐ | 公告 CRUD、置顶 |
| S-17 | 代理注册管理 | ☐ | 注册订单、收入统计、退款 |

### 3.2 开发者控制台（19 个模块）

| 编号 | 模块 | 已实现 | 说明 |
|---|---|---|---|
| D-01 | 工作台 | ☐ | 数据看板 + 公告显示区（平台公告 + 开发者公告） |
| D-02 | 应用管理 | ☐ | 应用 CRUD、密钥轮换 |
| D-03 | 卡密管理 | ☐ | 批量生成、导入导出、封禁 |
| D-04 | 卡类套餐 | ☐ | 时长卡/次数卡/永久卡/试用卡/功能解锁卡 |
| D-05 | 设备管理 | ☐ | 在线状态、强制下线、解绑、封禁 |
| D-06 | 用户管理 | ☐ | 终端用户列表、封禁 |
| D-07 | 订单管理 | ☐ | 订单列表、退款、导出 |
| D-08 | 代理管理 | ☐ | 代理 CRUD、邀请码生成、授权范围 |
| D-09 | 云变量 | ☐ | 变量 CRUD、按应用分组 |
| D-10 | 公告管理 | ☐ | 应用公告、平台公告 |
| D-11 | 版本管理 | ☐ | 版本号、最低版本、下载地址、强制更新 |
| D-12 | 验证日志 | ☐ | 验证/激活/心跳记录 |
| D-13 | 操作日志 | ☐ | 后台操作记录 |
| D-14 | 财务统计 | ☐ | 收入统计、代理业绩、佣金结算打款 |
| D-15 | 安全设置 | ☐ | IP 黑名单、频率限制 |
| D-16 | SDK 下载 | ☐ | 各语言 SDK 包 + 文档 |
| D-17 | 开发者设置 | ☐ | 资料、密码、2FA、API Token |
| D-18 | 支付配置 | ☐ | 双层模式切换（平台总支付 / 自有易支付） |
| D-19 | 代理充值审核 | ☐ | 审核代理充值申请 |

### 3.3 代理商控制台（10 个模块）

| 编号 | 模块 | 已实现 | 说明 |
|---|---|---|---|
| P-01 | 工作台 | ☐ | 销售数据、库存、佣金概览 + 平台公告 |
| P-02 | 卡密库存 | ☐ | 可售套餐、生成卡密（扣余额） |
| P-03 | 卡密管理 | ☐ | 自己生成的卡密列表 |
| P-04 | 销售订单 | ☐ | 自售订单 |
| P-05 | 佣金结算 | ☐ | 佣金明细、提现申请、打款记录 |
| P-06 | 独立门户 | ☐ | 代理专属购卡页（可绑子域名） |
| P-07 | 代理设置 | ☐ | 资料、密码、收款方式 |
| P-08 | 公告中心 | ☐ | 平台总公告 + 所属开发者应用公告 |
| P-09 | 余额充值 | ☐ | 向开发者充值余额 |
| P-10 | 实时购卡 | ☐ | 扫码购卡（备用方式） |

### 3.4 终端用户 H5（14 个页面）

| 编号 | 页面 | 已实现 |
|---|---|---|
| U-01 | 首页 | ☐ |
| U-02 | 应用详情页 | ☐ |
| U-03 | 购卡结算页 | ☐ |
| U-04 | 支付结果页 | ☐ |
| U-05 | 我的卡密 | ☐ |
| U-06 | 卡密详情 | ☐ |
| U-07 | 查卡页 | ☐ |
| U-08 | 在线激活页 | ☐ |
| U-09 | 用户登录/注册 | ☐ |
| U-10 | 用户中心 | ☐ |
| U-11 | 订单列表 | ☐ |
| U-12 | 公告详情 | ☐ |
| U-13 | 帮助中心 | ☐ |
| U-14 | 联系客服 | ☐ |

### 3.5 客户端 SDK

| 语言 | 包名 | 已实现 |
|---|---|---|
| Python | `keyauth-py` | ☐ |
| Node.js | `keyauth-node` | ☐ |
| Java | `keyauth-java` | ☐ |
| C# | `keyauth-csharp` | ☐ |
| Go | `keyauth-go` | ☐ |
| PHP | `keyauth-php` | ☐ |
| C/C++ | `keyauth-cpp` | ☐ |
| 易语言 | 模块源码 | ☐ |

---

## 4. 数据库设计

### 4.1 表清单（共 26 张表）

| 分类 | 表名 | 说明 |
|---|---|---|
| 平台 | `sys_admin` | 超管账号 |
| 平台 | `sys_config` | 系统配置（铁律 05） |
| 平台 | `sys_tenant` | 租户（开发者） |
| 平台 | `sys_tenant_quota` | 租户套餐配额 |
| 平台 | `sys_package` | 平台套餐定义 |
| 平台 | `tenant_pay_config` | 租户自有易支付配置 |
| 应用 | `app` | 开发者应用 |
| 应用 | `app_card_type` | 卡类套餐 |
| 应用 | `app_card` | 卡密 |
| 应用 | `app_device` | 设备绑定 |
| 应用 | `app_user` | 终端用户 |
| 应用 | `app_order` | 订单 |
| 应用 | `app_cloud_var` | 云变量 |
| 应用 | `app_notice` | 应用公告（合并到 notice） |
| 应用 | `app_version` | 应用版本 |
| 代理 | `agent` | 代理商账号 |
| 代理 | `agent_quota` | 代理可售范围 |
| 代理 | `agent_balance_log` | 代理余额流水 |
| 代理 | `agent_commission` | 佣金结算记录 |
| 代理 | `agent_withdraw` | 代理提现记录 |
| 代理 | `agent_invite_code` | 代理邀请码 |
| 代理 | `agent_registration_order` | 代理注册订单 |
| 公告 | `notice` | 统一公告表 |
| 公告 | `notice_target` | 公告精准投递 |
| 公告 | `notice_read` | 公告已读记录 |
| 安全 | `sec_ip_blacklist` | IP 黑名单 |
| 安全 | `sec_request_log` | 请求日志 |
| 日志 | `log_verify` | 验证日志（按月分区） |
| 日志 | `log_operation` | 后台操作日志 |

### 4.2 Redis 缓存键设计

| Key 模式 | TTL | 用途 |
|---|---|---|
| `config:{key}` | 1h | sys_config 缓存 |
| `pay:config:tenant:{tenant_id}` | 10min | 租户支付配置缓存 |
| `card:verify:{card_key_hash}` | 60s | 卡密验证结果缓存 |
| `device:online:{device_id}` | 180s | 设备在线状态 |
| `heartbeat:{card_id}` | 180s | 心跳计数 |
| `rate:verify:{ip}` | 60s | IP 限流计数 |
| `rate:login:{card_key}` | 60s | 卡密登录防爆破 |
| `session:tenant:{token}` | 24h | 租户会话 |
| `session:agent:{token}` | 24h | 代理会话 |
| `session:admin:{token}` | 24h | 超管会话 |
| `lock:card:{card_id}` | 5s | 卡密操作分布式锁 |
| `nonce:{nonce}` | 5min | Nonce 防重放 |
| `pay:notify:lock:{order_no}` | 10s | 回调防重入锁 |

---

## 5. 使用指南

### 5.1 安装

#### 方式 1：Docker Compose 一键部署
```bash
git clone https://github.com/your-org/keyauth-saas.git
cd keyauth-saas
cp .env.example .env
# 编辑 .env 修改数据库密码、域名等
docker-compose up -d
```

#### 方式 2：宝塔面板 Docker 部署
1. 宝塔面板 → Docker → 应用商店 → 自定义 → 上传 compose 文件
2. 配置环境变量
3. 启动容器
4. 宝塔配置反向代理 + SSL 证书

### 5.2 配置

环境变量（写入 `.env`）：
```
# 数据库
MYSQL_HOST=mysql
MYSQL_PORT=3306
MYSQL_DATABASE=keyauth
MYSQL_USER=keyauth
MYSQL_PASSWORD=your-password
MYSQL_ROOT_PASSWORD=your-root-password

# Redis
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=

# 应用
APP_PORT=8080
APP_MODE=release
APP_JWT_SECRET=your-jwt-secret
APP_AES_KEY=your-aes-32-byte-key
APP_RSA_PRIVATE_KEY_PATH=/app/keys/private.pem
APP_RSA_PUBLIC_KEY_PATH=/app/keys/public.pem

# 域名
DOMAIN=yourdomain.com
```

### 5.3 运行

```bash
# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f api

# 停止
docker-compose down

# 升级
git pull
docker-compose up -d --build
```

### 5.4 初始化

1. 访问 `https://yourdomain.com/install`
2. 设置超管账号密码
3. 配置平台总支付（易支付网关/商户号/密钥）
4. 创建套餐（免费版/专业版/企业版）
5. 配置代理注册费用

### 5.5 示例（开发者接入流程）

```go
// Go SDK 示例
package main

import (
    "github.com/your-org/keyauth-go"
)

func main() {
    client := keyauth.New("your-app-key", "your-sign-secret", "https://yourdomain.com")
    
    // 1. 用户登录（首次自动绑定设备）
    result, err := client.Login("K2X9-AB7C-MN4P-QR8S", getHWID())
    if err != nil {
        panic(err)
    }
    
    // 2. 校验响应签名
    if !client.VerifyResponse(result) {
        panic("响应签名校验失败")
    }
    
    // 3. 启动心跳保活
    client.StartHeartbeat(result.Token, 60, func() {
        // 心跳失败回调
        os.Exit(1)
    })
    
    // 4. 解锁功能
    if result.Features["pro"] {
        unlockProFeatures()
    }
}
```

---

## 6. 目录结构说明

```
keyauth-saas/
├── apps/
│   ├── server/                    # Go 后端
│   │   ├── cmd/
│   │   │   └── main.go           # 程序入口
│   │   ├── internal/
│   │   │   ├── config/           # 配置加载
│   │   │   ├── middleware/       # 中间件
│   │   │   │   ├── auth.go       # JWT 认证
│   │   │   │   ├── tenant.go     # 租户隔离
│   │   │   │   ├── signature.go  # HMAC 验签
│   │   │   │   ├── ratelimit.go  # 限流
│   │   │   │   └── logger.go     # 日志
│   │   │   ├── model/            # 数据模型
│   │   │   ├── repository/       # 数据访问层
│   │   │   ├── service/          # 业务逻辑层
│   │   │   │   ├── auth/
│   │   │   │   ├── tenant/
│   │   │   │   ├── app/
│   │   │   │   ├── card/
│   │   │   │   ├── device/
│   │   │   │   ├── verify/       # 客户端验证 API
│   │   │   │   ├── pay/          # 支付（平台总支付 + 自有易支付）
│   │   │   │   ├── agent/        # 代理体系
│   │   │   │   ├── notice/       # 公告
│   │   │   │   └── stats/        # 统计
│   │   │   ├── handler/          # HTTP 处理器
│   │   │   └── router/           # 路由
│   │   ├── pkg/
│   │   │   ├── crypto/           # 加密工具（AES/RSA/HMAC/bcrypt）
│   │   │   ├── jwt/              # JWT 工具
│   │   │   ├── snowflake/        # 雪花算法
│   │   │   └── securerandom/     # 安全随机数
│   │   ├── migrations/           # 数据库迁移
│   │   └── go.mod
│   │
│   ├── admin/                     # Vue3 管理后台（超管/开发者/代理）
│   │   ├── src/
│   │   │   ├── api/
│   │   │   ├── components/
│   │   │   ├── layouts/
│   │   │   │   ├── AdminLayout.vue     # 超管布局
│   │   │   │   ├── TenantLayout.vue    # 开发者布局
│   │   │   │   └── AgentLayout.vue     # 代理布局
│   │   │   ├── views/
│   │   │   │   ├── admin/              # 超管页面（S-01 ~ S-17）
│   │   │   │   ├── tenant/             # 开发者页面（D-01 ~ D-19）
│   │   │   │   └── agent/              # 代理页面（P-01 ~ P-10）
│   │   │   ├── router/
│   │   │   ├── stores/
│   │   │   └── utils/
│   │   └── package.json
│   │
│   └── h5/                        # 终端用户 H5（移动端）
│       ├── src/
│       │   ├── api/
│       │   ├── pages/             # U-01 ~ U-14
│       │   └── stores/
│       └── package.json
│
├── sdks/                          # 客户端 SDK
│   ├── python/
│   ├── nodejs/
│   ├── java/
│   ├── csharp/
│   ├── go/
│   ├── php/
│   ├── cpp/
│   └── e-lang/
│
├── deploy/                        # 部署相关
│   ├── docker-compose.yml
│   ├── Dockerfile.api
│   ├── Dockerfile.admin
│   ├── Dockerfile.h5
│   ├── nginx.conf
│   ├── baota-install.sh          # 宝塔安装脚本
│   └── README.md
│
├── docs/                          # 文档
│   ├── CHANGELOG.md              # 更新日志
│   ├── PROJECT.md                # 项目文档（本文件）
│   ├── SPEC.md                   # 规范文档
│   ├── TODO.md                   # 待完成文档
│   ├── api/                      # API 文档（OpenAPI 3.0）
│   ├── sdk/                      # SDK 对接文档
│   └── database/                 # 数据库设计文档
│
├── scripts/                       # 脚本
│   ├── backup.sh                 # 备份脚本
│   ├── restore.sh                # 恢复脚本
│   └── migrate.sh                # 迁移脚本
│
├── .env.example
├── .gitignore
├── LICENSE
├── README.md
└── go.mod
```

---

## 7. 贡献指南

### 7.1 开发环境搭建
```bash
# 1. 克隆仓库
git clone https://github.com/your-org/keyauth-saas.git
cd keyauth-saas

# 2. 启动依赖服务
docker-compose up -d mysql redis

# 3. 后端开发
cd apps/server
go mod tidy
go run cmd/main.go

# 4. 前端开发
cd apps/admin
pnpm install
pnpm dev
```

### 7.2 提交规范
详见 [SPEC.md](./SPEC.md) 第 4 节「提交规范」。

### 7.3 分支策略
- `main`：生产分支，受保护，只接受 PR
- `develop`：开发主分支
- `feature/*`：功能分支
- `fix/*`：修复分支
- `release/*`：发布分支

### 7.4 编码铁律（HARD）
所有代码必须遵守：
- [references/04 禁硬编码假数据](../../web-project-flow/references/04-no-hardcode-fake-data.md)
- [references/05 配置后台化 sys_config](../../web-project-flow/references/05-config-to-backend.md)
- [references/06 防 AI 幻觉](../../web-project-flow/references/06-anti-hallucination.md)

违反铁律的代码必须重写。

---

## 8. 联系与支持

- 项目仓库：https://github.com/your-org/keyauth-saas
- 问题反馈：https://github.com/your-org/keyauth-saas/issues
- 文档中心：https://docs.yourdomain.com

---

**文档版本**：0.1.0  
**最后更新**：2026-07-19  
**维护者**：KeyAuth SaaS Team
