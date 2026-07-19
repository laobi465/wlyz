# KeyAuth SaaS

> 面向开发者的多租户卡密验证 SaaS 平台

[![Version](https://img.shields.io/badge/version-0.2.3-blue.svg)](docs/CHANGELOG.md)
[![License](https://img.shields.io/badge/license-Proprietary-red.svg)](#许可证)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4+-42b883.svg)](https://vuejs.org)

## 项目简介

KeyAuth SaaS 是一个多租户卡密验证 SaaS 平台，借鉴布丁卡密的安全设计理念，为软件开发者提供：

- **在线验证 + 一机一卡**：HMAC-SHA256 签名 + RSA-4096 响应签名 + 硬件指纹绑定
- **多层支付体系**：平台总支付（默认）+ 开发者自定义易支付（按套餐开通）
- **三级公告系统**：平台公告 / 开发者公告 / 应用公告 同时显示
- **代理分销体系**：开发者邀请码 + 注册费 + 佣金分成（按比例/按差价）
- **后台可视化配置**：所有可变参数走 `sys_config` 表，无需重启即时生效

## 当前版本（v0.2.3）

| 模块 | 状态 | 说明 |
|---|---|---|
| 项目骨架 + 26 张表 DDL + 种子数据 | ✅ 已完成 | Go + Vue3 + Docker + 宝塔部署 |
| JWT 双 Token + TOTP 2FA + 登录锁定 | ✅ 已完成 v0.2.1 | 三角色统一鉴权，refresh 轮换 + 黑名单 |
| 应用管理（CRUD + 密钥生成/轮换） | ✅ 已完成 v0.2.2 | AppKey/AppSecret/SignSecret + AES 加密入库 |
| 卡密管理（5 类型 + 批量生成 + 状态机） | ✅ 已完成 v0.2.2 | duration/count/permanent/trial/feature |
| 客户端验证 API（9 个端点全部实现） | ✅ 已完成 v0.2.2 | login/verify/heartbeat/bind/unbind/get_var/notice/version/logout |
| 心跳保活（Redis Sorted Set） | ✅ 已完成 v0.2.2 | 百万级在线设备支持 |
| 平台总支付（彩虹易支付） | ✅ 已完成 v0.2.3 | 下单/异步回调/同步跳转/自动发卡/Redis 防重入/订单超时关闭/平台抽成结算 |
| 前端业务页面（三角色 + H5） | ⏳ 计划中 v0.2.4 | 当前为骨架占位 |

详细变更见 [CHANGELOG.md](docs/CHANGELOG.md)，任务进度见 [TODO.md](docs/TODO.md)。

## 技术栈

| 层级 | 技术选型 |
|---|---|
| 后端 | Go 1.22 + Gin + GORM |
| 前端 | Vue 3.4 + TypeScript + Element Plus + Vite + Pinia |
| 数据库 | MySQL 8.0 + Redis 7 |
| 部署 | Docker + Docker Compose + 宝塔面板 |
| 加密 | AES-256-GCM + RSA-4096 + bcrypt(cost=12) + HMAC-SHA256 |

## 目录结构

```
keyauth-saas/
├── apps/
│   ├── server/                    # Go 后端
│   │   ├── cmd/main.go            # 入口
│   │   ├── internal/
│   │   │   ├── config/            # 配置加载 + sys_config 缓存
│   │   │   ├── handler/           # HTTP 处理器
│   │   │   ├── middleware/         # 中间件（auth/signature/tenant/ratelimit）
│   │   │   ├── model/             # GORM 模型
│   │   │   └── router/            # 路由注册
│   │   ├── migrations/            # 数据库迁移 SQL
│   │   │   ├── 001_init_schema.up.sql   # 26 张表 DDL
│   │   │   ├── 001_init_schema.down.sql
│   │   │   ├── 002_seed_data.up.sql     # 默认配置/套餐/超管
│   │   │   └── 002_seed_data.down.sql
│   │   └── pkg/
│   │       ├── crypto/            # AES/RSA/HMAC/bcrypt/卡密生成
│   │       └── snowflake/         # 订单号生成
│   └── admin/                     # Vue3 前端
│       ├── src/
│       │   ├── api/               # HTTP 封装
│       │   ├── components/        # 公告横幅等通用组件
│       │   ├── layouts/           # 三套布局：AdminLayout/TenantLayout/AgentLayout
│       │   ├── router/            # 路由 + 角色守卫
│       │   ├── stores/            # Pinia stores（auth/sysConfig）
│       │   ├── styles/            # 全局 SCSS 变量
│       │   └── views/             # 登录页 + 404 + 代理注册页
│       └── vite.config.ts
├── configs/                       # 运行时配置
│   └── config.yaml.example
├── deploy/
│   └── nginx/                     # admin.conf / gateway.conf
├── docs/                          # 项目文档
│   ├── CHANGELOG.md
│   ├── PROJECT.md
│   ├── SPEC.md
│   └── TODO.md
├── keys/                          # RSA 密钥对挂载点
├── scripts/
│   ├── baota_deploy.sh            # 宝塔面板一键部署
│   └── reset_admin_password.sh    # 重置超管密码
├── Dockerfile                     # 后端镜像
├── Dockerfile.admin               # 前端镜像
├── docker-compose.yml             # 完整编排
├── .env.example                   # 环境变量样例
└── README.md
```

## 快速开始

### 1. 准备环境

- Linux 服务器（推荐 Ubuntu 22.04 / CentOS 7+）
- 已安装宝塔面板（推荐）或 Docker + Docker Compose
- 域名 + SSL 证书（生产环境）

### 2. 拉取代码

```bash
git clone <repo-url> keyauth-saas
cd keyauth-saas
```

### 3. 配置环境变量

```bash
cp .env.example .env
vim .env
```

**必须修改的字段**（铁律 04：禁止硬编码密钥）：

```bash
MYSQL_ROOT_PASSWORD=<强密码>
MYSQL_PASSWORD=<强密码>
REDIS_PASSWORD=<强密码>

# AES-256 密钥：32 字节 base64
openssl rand -base64 32
AES_KEY=<上面的输出>

# JWT 密钥
openssl rand -hex 32
JWT_SECRET=<上面的输出>
```

### 4. 生成 RSA 密钥对

```bash
mkdir -p keys
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out keys/rsa_private.pem
openssl rsa -in keys/rsa_private.pem -pubout -out keys/rsa_public.pem
chmod 600 keys/rsa_private.pem
```

### 5. 复制后端配置

```bash
cp configs/config.yaml.example configs/config.yaml
# 按需修改（一般默认即可，敏感字段从环境变量传入）
```

### 6. 启动服务

**方式一：宝塔面板一键部署**（推荐）

```bash
bash scripts/baota_deploy.sh
```

**方式二：Docker Compose 直接启动**

```bash
docker compose up -d
```

**方式三：生产环境（含 nginx 网关 + HTTPS）**

```bash
docker compose --profile production up -d
```

### 7. 首次部署后必做

```bash
# 重置超管密码（种子数据中的哈希为占位）
bash scripts/reset_admin_password.sh

# 在宝塔面板「安全」中关闭 MySQL/Redis 的公网端口
# 登录后台 http://your-ip:8081 配置平台易支付参数
```

## 默认账号

| 角色 | 用户名 | 密码 | 说明 |
|---|---|---|---|
| 平台超管 | admin | （占位） | 部署后必须执行 `reset_admin_password.sh` 重置 |

## 核心文档

- [CHANGELOG.md](docs/CHANGELOG.md) —— 版本变更记录
- [PROJECT.md](docs/PROJECT.md) —— 项目架构与模块清单
- [SPEC.md](docs/SPEC.md) —— 代码 / API / 安全 / 部署规范
- [TODO.md](docs/TODO.md) —— 任务清单与里程碑

## 安全说明

- 所有敏感字段（AES_KEY / JWT_SECRET / RSA 私钥 / DB 密码）通过环境变量传入，不进代码
- 客户端 API 强制 HMAC-SHA256 签名 + nonce 防重放
- 服务端响应可选 RSA-4096 签名（fail-closed）
- 密码 bcrypt cost=12，敏感字段 AES-256-GCM 加密
- IP 自动封禁机制（失败次数达阈值自动拉黑）
- 心跳通过 Redis Sorted Set 维护，支持百万级在线设备

## 后续版本规划

详见 [TODO.md](docs/TODO.md)：

- **v0.2.4**：前端业务页面（三角色控制台 + 终端用户 H5）
- **v0.3.0**：开发者自有易支付 + 代理注册付费 + 三级公告 + SDK（Python/Node/PHP）
- **v0.4.0**：多级代理 + 全语言 SDK + 在线更新 + 数据备份恢复

## 开发约束（铁律）

本项目严格遵守 `web-project-flow` skill 的三份铁律：

1. **禁硬编码**：密钥 / token / 域名 / 价格 / 接口地址 全部抽离到环境变量或 sys_config
2. **配置后台化**：所有可调参数走 `sys_config` 表 + Redis 缓存 + 后台可视化编辑
3. **防幻觉**：不确定处标注「待核实」，代码不确定处标注「需验证」

## 许可证

Proprietary —— 未经授权禁止商用
