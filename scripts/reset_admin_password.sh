#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— 重置超管密码脚本
# ============================================================
# 用法：bash scripts/reset_admin_password.sh [新密码]
#   不传密码则交互式输入（隐藏输入）
# ============================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] [WARN]${NC} $*"; }
err()  { echo -e "${RED}[$(date '+%H:%M:%S')] [ERROR]${NC} $*" >&2; }

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

# ---------- 获取新密码 ----------
if [[ $# -ge 1 ]]; then
    NEW_PASSWORD="$1"
else
    read -s -p "请输入新的超管密码（≥ 8 位）: " NEW_PASSWORD
    echo ""
    read -s -p "再次确认密码: " CONFIRM
    echo ""
    if [[ "${NEW_PASSWORD}" != "${CONFIRM}" ]]; then
        err "两次输入不一致，已退出"
        exit 1
    fi
fi

# 校验长度
if [[ ${#NEW_PASSWORD} -lt 8 ]]; then
    err "密码长度不足 8 位"
    exit 1
fi

# ---------- 检查是否已部署 ----------
if ! docker compose ps server 2>/dev/null | grep -q "keyauth-server"; then
    err "未检测到 keyauth-server 容器，请先完成部署"
    exit 1
fi

# ---------- 加载环境 ----------
if [[ -f .env ]]; then
    set -a; source .env; set +a
fi

# ---------- 方式 1：调用后端内置重置命令 ----------
# 后端支持：keyauth-server --reset-admin-password=NEW_PASSWORD
# 待核实：实际实现需在 cmd/main.go 添加 subcommand 处理逻辑
log "尝试调用后端重置命令..."
if docker compose exec -T server /app/keyauth-server --reset-admin-password="${NEW_PASSWORD}" 2>/dev/null; then
    log "密码已通过后端命令重置成功"
    exit 0
fi

# ---------- 方式 2：直接通过 MySQL 更新哈希 ----------
warn "后端命令调用失败，回退到直接 SQL 更新方式"
warn "此方式需在主机上安装 htpasswd（apache2-utils）"

if ! command -v htpasswd >/dev/null 2>&1; then
    err "未找到 htpasswd 命令，请安装：apt install -y apache2-utils  或  yum install -y httpd-tools"
    exit 1
fi

# 生成 bcrypt(cost=12) 哈希
HASH=$(htpasswd -bnBC 12 "" "${NEW_PASSWORD}" | tr -d ':\n' | sed 's/$2y/$2a/')
if [[ -z "${HASH}" ]]; then
    err "哈希生成失败"
    exit 1
fi

# 更新数据库
log "更新超管密码哈希..."
docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" <<SQL
UPDATE sys_admin SET password_hash='${HASH}' WHERE username='admin';
SELECT username, status, updated_at FROM sys_admin WHERE username='admin';
SQL

log "超管密码已重置成功"
log "请使用以下凭据登录管理后台："
echo "  用户名：admin"
echo "  密码：${NEW_PASSWORD}"
warn "请妥善保管密码，建议立即在后台开启 2FA"
