# KeyAuth SaaS

> 面向开发者的多租户卡密验证 SaaS 平台

[![Version](https://img.shields.io/badge/version-0.9.0-blue.svg)](docs/CHANGELOG.md)
[![License](https://img.shields.io/badge/license-Proprietary-red.svg)](#许可证)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4+-42b883.svg)](https://vuejs.org)
[![Deploy](https://img.shields.io/badge/deploy-one--click-success.svg)](#一键部署)
[![Security](https://img.shields.io/badge/security-audit%20passed-brightgreen.svg)](#安全审计)
[![MySQL](https://img.shields.io/badge/MySQL-8.0.36-4479A1.svg)](https://www.mysql.com/)

## 项目简介

KeyAuth SaaS 是一个多租户卡密验证 SaaS 平台，借鉴布丁卡密的安全设计理念，为软件开发者提供：

- **在线验证 + 一机一卡**：HMAC-SHA256 签名 + RSA-4096 响应签名 + 硬件指纹绑定
- **多层支付体系**：平台总支付（默认）+ 开发者自定义易支付（按套餐开通）+ 海外支付（USDT/PayPal/Stripe）
- **三级公告系统**：平台公告 / 开发者公告 / 应用公告 同时显示
- **代理分销体系**：开发者邀请码 + 注册费 + 多级佣金分成（按比例/按差价/跨级分润）
- **后台可视化配置**：所有可变参数走 `sys_config` 表，无需重启即时生效
- **响应式 H5 全栈**：管理后台 / 开发者控制台 / 代理中心 / 官网 / 终端用户 H5 全部响应式适配
- **高级分析体系**：用户行为画像 + 卡密使用画像 + 风险用户识别 + 24h 异常模式检测 + 自动封禁候选
- **全语言 SDK**：Python / Node.js / PHP / Go / Java / C# / C++ / 易语言 8 语言客户端 SDK

## 一键部署

> 全新服务器？只需 SSH 连接后执行下面这一行命令，脚本会自动完成：检测系统 → 安装宝塔（若未装）→ 安装 Docker（若未装）→ 拉取源码 → 生成密钥 → 构建启动 → 输出部署信息到 `/root/keyauth_deploy_info.txt`。

### 远程一行命令（推荐，适合全新服务器，无需事先上传源码）

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh)"
```

### 先下载后执行（适合需审查脚本或网络不稳定）

```bash
curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh -o one_click_deploy.sh
sudo bash one_click_deploy.sh
```

### 已 clone 项目内执行

```bash
sudo bash scripts/one_click_deploy.sh
```

**脚本执行流程（约 8-15 分钟，一行命令完成全部）**：

| 步骤 | 操作 | 说明 |
|---|---|---|
| 1 | 检测操作系统 | 自动识别 CentOS/Ubuntu/Debian 系 |
| 2 | 安装基础工具 | curl / wget / openssl / git |
| 3 | 检测/安装宝塔面板 | 未装则拉取宝塔官方脚本自动安装 |
| 4 | 检测/安装 Docker | 优先用宝塔脚本，回退 Docker 官方脚本（阿里云镜像） |
| 5 | 拉取/更新项目源码 | git clone 到 `/www/wwwroot/keyauth` |
| 6 | 生成 RSA-4096 密钥对 | 响应签名用，现场生成不进仓库 |
| 7 | 生成 `.env` 配置 | 自动填充强随机密钥（MySQL/Redis/AES/JWT） |
| 8 | `docker compose up -d --build` | 构建 mysql/redis/server/admin |
| 9 | **初始化超管账号 admin/admin123** | 检测占位 hash → 自动生成 bcrypt 写入 → 验证 |
| 10 | **生成部署信息 txt** | 所有信息写入 `/root/keyauth_deploy_info.txt`（chmod 600） |

**部署完成后自动生成的 txt 文件包含**：
- 宝塔面板入口（地址/账号/密码/端口）
- KeyAuth 访问地址（前端后台 + API）
- **超管账号 admin/admin123**（首次部署自动初始化，登录后请立即改密）
- 所有密钥（MySQL/AES/JWT）
- 服务状态、运维命令、备份建议
- **第七章：宝塔反代 + 免费 SSL 完整教程**（6 步 + 常见问题）

**部署完成后必做**：
1. `cat /root/keyauth_deploy_info.txt` 查看完整部署信息（含反代教程）
2. 用 **admin/admin123** 通过 `http://your-ip:8081/admin/login` 登录管理员后台 → 立即修改默认密码（用户端登录页 `/login` 仅显示开发者/代理入口）
3. 域名 A 记录解析到本服务器 IP
4. 宝塔面板「网站」添加站点 → 反向代理到 `http://127.0.0.1:8081`
5. 宝塔面板「SSL」申请 Let's Encrypt 免费证书 + 强制 HTTPS
6. 宝塔面板「安全」关闭 8081/8080/3306/6379 公网端口
7. 登录后台「系统配置 > 支付」配置易支付参数

> 脚本严格遵守三铁律：禁硬编码（密钥现场生成）/ 配置走 sys_config / 反幻觉（脚本含详细日志和错误处理）。完整说明见 [scripts/one_click_deploy.sh](scripts/one_click_deploy.sh)。

## 手动部署

适合已有环境、需精细控制每一步的用户。

### 1. 准备环境

- Linux 服务器（推荐 Ubuntu 22.04 / CentOS 7+）
- 已安装宝塔面板（推荐）或 Docker + Docker Compose
- 域名 + SSL 证书（生产环境）

### 2. 拉取代码

```bash
git clone https://github.com/laobi465/wlyz.git keyauth-saas
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

# AES-256 密钥：必须正好 32 字节字符串（不是 base64 解码后 32 字节）
openssl rand -hex 16
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

## 已部署用户升级

```bash
cd /www/wwwroot/keyauth
git pull origin main
docker compose up -d --build admin server
# 浏览器：清除 localStorage 后重新登录（Console 执行 localStorage.clear() 然后刷新）
```

版本更新详情见 [CHANGELOG.md](docs/CHANGELOG.md)。

## 默认账号

| 角色 | 用户名 | 密码 | 登录入口 | 说明 |
|---|---|---|---|---|
| 平台超管 | admin | admin123 | `/admin/login` | 首次部署自动初始化，登录后请立即改密 |
| 开发者 | - | - | `/login` | 用户端登录页，自助注册 |
| 代理 | - | - | `/login` | 用户端登录页，需开发者邀请码注册 |

## 技术栈

| 层级 | 技术选型 |
|---|---|
| 后端 | Go 1.22 + Gin + GORM |
| 前端 | Vue 3.4 + TypeScript + Element Plus + Vite + Pinia |
| 数据库 | MySQL 8.0.36 + Redis 7 |
| 部署 | Docker + Docker Compose + 宝塔面板 |
| 加密 | AES-256-GCM + RSA-4096 + bcrypt(cost=12) + HMAC-SHA256 |
| SDK | Python / Node.js / PHP / Go / Java / C# / C++ / 易语言 |

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
│   │   └── pkg/
│   │       ├── crypto/            # AES/RSA/HMAC/bcrypt/卡密生成
│   │       └── snowflake/         # 订单号生成
│   └── admin/                     # Vue3 前端
│       ├── src/
│       │   ├── api/               # HTTP 封装
│       │   ├── components/        # 通用组件
│       │   ├── layouts/           # 响应式布局
│       │   ├── router/            # 路由 + 角色守卫
│       │   ├── stores/            # Pinia stores
│       │   ├── styles/            # 全局 SCSS 变量
│       │   └── views/             # 页面
│       └── vite.config.ts
├── configs/                       # 运行时配置
├── deploy/
│   └── nginx/                     # admin.conf / gateway.conf
├── docs/                          # 项目文档
├── keys/                          # RSA 密钥对挂载点
├── scripts/
│   ├── one_click_deploy.sh        # SSH 一键自动化部署（推荐）
│   ├── baota_deploy.sh            # 宝塔面板手动部署
│   ├── clean_dirty_migration.sh   # dirty 迁移修复专用脚本
│   └── reset_admin_password.sh    # 重置超管密码
├── sdks/                          # 8 语言客户端 SDK
├── Dockerfile                     # 后端镜像
├── Dockerfile.admin               # 前端镜像
├── docker-compose.yml             # 完整编排
├── .env.example                   # 环境变量样例
└── README.md
```

## 核心文档

- [CHANGELOG.md](docs/CHANGELOG.md) —— 版本变更记录
- [PROJECT.md](docs/PROJECT.md) —— 项目架构与模块清单
- [SPEC.md](docs/SPEC.md) —— 代码 / API / 安全 / 部署规范
- [TODO.md](docs/TODO.md) —— 任务清单与里程碑

## 安全审计

本项目已完成全项目安全审计，共发现并修复 **82 个 bug**（4 类优先级全覆盖）：

| 优先级 | 类别 | 数量 | 修复内容 |
|---|---|---|---|
| **P0** | 高危 | 13 | 部署链路 5 bug + 认证绕过 + SQL 注入 + 权限提升 + 敏感信息泄露 + 并发竞态 |
| **P1** | 普通 | 21 | Migration 兼容性 + 加密名实一致 + JWT Subject + Nonce 顺序 + IP 黑名单 + CF IP 伪造 + v-html XSS + Cookie Secure + 充值 FOR UPDATE + 提现流水精确匹配 + TOTP skew |
| **P2** | 联调 | 15 | 字段映射 + 枚举对齐 + 分页参数 + 云变量字段 + 收入趋势 + 邀请码 + 支付配置 + 版本 channel + 公告 type/status + 佣金 type/status |
| **P3** | 优化 | 34 | 30 处错误信息泄露 + 4 处 N+1 查询 + 4 处 HTTP 客户端超时 |

**安全亮点**：
- JWT 强制校验 `Subject == "access"`，refresh token 不能访问业务接口
- 充值/提现/支付回调所有状态转换用 `Where(status=pending)+RowsAffected` 检查，DB 级幂等
- GORM `clause.Locking{Strength:"UPDATE"}` 行锁保护并发余额操作
- v-html 接入 DOMPurify 防 XSS，Cookie 启用 Secure + SameSite
- 密码 bcrypt cost=12，敏感字段 AES-256-GCM 加密
- 心跳通过 Redis Sorted Set 维护，支持百万级在线设备

详细修复记录见 [CHANGELOG.md](docs/CHANGELOG.md)。

## 安全说明

- 所有敏感字段（AES_KEY / JWT_SECRET / RSA 私钥 / DB 密码）通过环境变量传入，不进代码
- 客户端 API 强制 HMAC-SHA256 签名 + nonce 防重放（签名校验后再写 nonce）
- 服务端响应可选 RSA-4096 签名（fail-closed）
- IP 自动封禁机制（失败次数达阈值自动拉黑）+ Redis 故障回退 MySQL
- 公开端点统一限流（默认 10 次/分钟，可后台调整）

## 开发约束（铁律）

本项目严格遵守三份铁律：

1. **禁硬编码**：密钥 / token / 域名 / 价格 / 接口地址 全部抽离到环境变量或 sys_config
2. **配置后台化**：所有可调参数走 `sys_config` 表 + Redis 缓存 + 后台可视化编辑
3. **防幻觉**：不确定处标注「待核实」，代码不确定处标注「需验证」

## 许可证

Proprietary —— 未经授权禁止商用
