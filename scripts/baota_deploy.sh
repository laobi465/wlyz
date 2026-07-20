#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— 宝塔面板 Docker 部署脚本
# ============================================================
# 适用：已安装宝塔面板的 Linux 服务器（CentOS 7+ / Ubuntu 18+）
# 用法：bash scripts/baota_deploy.sh
# ============================================================
set -euo pipefail

# ---------- 颜色 ----------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] [WARN]${NC} $*"; }
err()  { echo -e "${RED}[$(date '+%H:%M:%S')] [ERROR]${NC} $*" >&2; }

# ---------- 检查 root ----------
if [[ $EUID -ne 0 ]]; then
    err "请使用 root 用户执行：sudo bash scripts/baota_deploy.sh"
    exit 1
fi

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

log "当前项目目录：$PROJECT_DIR"

# ---------- 检查 .env ----------
if [[ ! -f .env ]]; then
    err "未找到 .env 文件，请先执行：cp .env.example .env 并填写真实密钥"
    exit 1
fi

# 加载环境变量
set -a; source .env; set +a

# 校验关键字段
if [[ "${AES_KEY}" == "CHANGE_ME"* || "${JWT_SECRET}" == "CHANGE_ME"* || "${MYSQL_ROOT_PASSWORD}" == "CHANGE_ME"* ]]; then
    err ".env 中仍存在 CHANGE_ME 占位符，请填写真实值后再执行"
    exit 1
fi

# ---------- 检查 Docker ----------
if ! command -v docker >/dev/null 2>&1; then
    log "未检测到 Docker，正在通过宝塔安装..."
    # 宝塔面板 -> 软件商店 -> Docker 管理器 一键安装
    # 命令行备用方案：
    curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
    systemctl enable docker
    systemctl start docker
fi

if ! docker compose version >/dev/null 2>&1; then
    err "未检测到 docker compose v2，请通过宝塔软件商店安装「Docker 管理器」或手动安装 compose 插件"
    exit 1
fi

log "Docker 与 Compose 检查通过"

# ---------- 生成 RSA 密钥对 ----------
mkdir -p keys
if [[ ! -f keys/rsa_private.pem || ! -f keys/rsa_public.pem ]]; then
    log "生成 RSA-4096 密钥对..."
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out keys/rsa_private.pem 2>/dev/null
    openssl rsa -in keys/rsa_private.pem -pubout -out keys/rsa_public.pem 2>/dev/null
    chmod 600 keys/rsa_private.pem
    chmod 644 keys/rsa_public.pem
    log "RSA 密钥对已生成：keys/rsa_private.pem / keys/rsa_public.pem"
fi

# ---------- 校验 AES 密钥长度 ----------
# 后端 config.go 校验 len(AES_KEY) == 32（字符串本身 32 字节，不是 base64 解码后 32 字节）
# 正确生成方式：openssl rand -hex 16  → 16 字节随机 → hex 编码 32 字符
AES_KEY_LEN=$(echo -n "${AES_KEY}" | wc -c | tr -d ' ')
if [[ "${AES_KEY_LEN}" -ne 32 ]]; then
    warn "AES_KEY 长度为 ${AES_KEY_LEN} 字符，应为 32 字符"
    warn "请使用以下命令重新生成：openssl rand -hex 16"
fi

# ---------- 构建并启动 ----------
log "开始构建镜像并启动服务..."
docker compose build --pull
docker compose up -d

# ---------- 等待服务就绪 ----------
log "等待 MySQL 与 Redis 就绪..."
for i in {1..30}; do
    if docker compose exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot -p"${MYSQL_ROOT_PASSWORD}" >/dev/null 2>&1; then
        log "MySQL 已就绪"
        break
    fi
    sleep 2
done

for i in {1..20}; do
    if docker compose exec -T redis redis-cli -a "${REDIS_PASSWORD}" ping >/dev/null 2>&1; then
        log "Redis 已就绪"
        break
    fi
    sleep 1
done

log "等待后端 server 启动..."
for i in {1..30}; do
    if curl -sf http://127.0.0.1:${SERVER_PORT:-8080}/health >/dev/null 2>&1; then
        log "后端 server 已就绪"
        break
    fi
    sleep 2
done

# ---------- 重置超管密码 ----------
warn "种子数据中超管密码为占位哈希，需要执行以下命令重置："
echo ""
echo -e "  ${BLUE}bash scripts/reset_admin_password.sh${NC}"
echo ""

# ---------- 防火墙提示 ----------
log "部署完成！"
echo ""
echo "==================== 访问信息 ===================="
echo "  后台地址：http://$(hostname -I | awk '{print $1}'):${ADMIN_PORT:-8081}"
echo "  API 地址：http://$(hostname -I | awk '{print $1}'):${SERVER_PORT:-8080}"
echo "  MySQL 端口：${MYSQL_PORT:-3306}（建议仅内网访问）"
echo "  Redis 端口：${REDIS_PORT:-6379}（建议仅内网访问）"
echo "================================================="
echo ""
warn "安全提醒："
echo "  1. 立即执行 reset_admin_password.sh 修改超管默认密码"
echo "  2. 在宝塔面板「安全」中关闭 MySQL/Redis 公网端口"
echo "  3. 在后台「系统配置 > 支付」中配置平台易支付参数"
echo "  4. 在宝塔面板「网站」中绑定域名并申请 SSL 证书"
