#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— 清理 schema_migrations dirty 状态
# ============================================================
# 用途：server 启动报「数据库迁移处于 dirty 状态」时执行此脚本
# 成因：之前启动时某版本迁移执行到一半失败（事务回滚但 dirty 标记已持久化）
# 修复：删除 dirty 记录，重启 server 后会重新执行该版本迁移
#
# 用法：
#   bash scripts/clean_dirty_migration.sh              # 自动检测并清理
#   bash scripts/clean_dirty_migration.sh --show       # 只查看不清理
#   bash scripts/clean_dirty_migration.sh --version 11 # 只清理指定版本
# ============================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] [WARN]${NC} $*"; }
err()  { echo -e "${RED}[$(date '+%H:%M:%S')] [ERROR]${NC} $*" >&2; }

# 切到项目根目录（脚本在 scripts/ 下）
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

# 参数解析
SHOW_ONLY=0
TARGET_VERSION=""
for arg in "$@"; do
    case "$arg" in
        --show) SHOW_ONLY=1 ;;
        --version) shift; TARGET_VERSION="${1:-}" ;;
        --version=*) TARGET_VERSION="${arg#*=}" ;;
        -h|--help)
            echo "用法: bash scripts/clean_dirty_migration.sh [--show] [--version N]"
            echo "  --show       只查看不清理"
            echo "  --version N  只清理指定版本"
            exit 0 ;;
    esac
done

# 检查 .env
if [[ ! -f .env ]]; then
    err ".env 不存在，请先执行一键部署或手动 cp .env.example .env"
    exit 1
fi

set +e
set -a; source .env; set +a
set -e

DB_NAME="${MYSQL_DATABASE:-keyauth}"
DB_ROOT_PWD="${MYSQL_ROOT_PASSWORD:-}"

if [[ -z "$DB_ROOT_PWD" ]]; then
    err "MYSQL_ROOT_PASSWORD 未配置，请检查 .env"
    exit 1
fi

# 检查 mysql 容器是否运行
if ! docker compose ps mysql 2>/dev/null | grep -q "Up\|running"; then
    err "mysql 容器未运行，请先启动：docker compose up -d mysql"
    exit 1
fi

log "数据库：$DB_NAME"
log "检查 schema_migrations 表状态..."

# 检查表是否存在
TBL_EXISTS=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
    -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${DB_NAME}' AND table_name='schema_migrations';" 2>/dev/null || echo "0")

if [[ "${TBL_EXISTS:-0}" -eq 0 ]]; then
    log "✓ schema_migrations 表不存在（首次部署或未执行过迁移），无需清理"
    exit 0
fi

# 查看所有迁移记录
echo ""
echo -e "${CYAN}==== schema_migrations 全部记录 ====${NC}"
docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
    -e "SELECT version, dirty, applied_at FROM schema_migrations ORDER BY version;" 2>/dev/null
echo ""

# 查找 dirty 记录
if [[ -n "$TARGET_VERSION" ]]; then
    DIRTY_VERSIONS="$TARGET_VERSION"
    DIRTY_COUNT=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
        -N -e "SELECT COUNT(*) FROM schema_migrations WHERE version=${TARGET_VERSION};" 2>/dev/null || echo "0")
    if [[ "${DIRTY_COUNT:-0}" -eq 0 ]]; then
        log "✓ 版本 ${TARGET_VERSION} 无记录，无需清理"
        exit 0
    fi
else
    DIRTY_VERSIONS=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
        -N -e "SELECT version FROM schema_migrations WHERE dirty=1;" 2>/dev/null || echo "")
fi

if [[ -z "$DIRTY_VERSIONS" ]]; then
    log "✓ 无 dirty 记录，数据库迁移状态正常"
    exit 0
fi

warn "发现 dirty 版本：${DIRTY_VERSIONS}"

if [[ $SHOW_ONLY -eq 1 ]]; then
    log "（--show 模式，不执行清理）"
    exit 0
fi

# 执行清理
if [[ -n "$TARGET_VERSION" ]]; then
    log "清理版本 ${TARGET_VERSION} 的记录..."
    docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
        -e "DELETE FROM schema_migrations WHERE version=${TARGET_VERSION};" >/dev/null 2>&1
else
    log "清理所有 dirty 记录..."
    docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
        -e "DELETE FROM schema_migrations WHERE dirty=1;" >/dev/null 2>&1
fi
log "✓ dirty 记录已清理"

# 重启 server
if docker compose ps server 2>/dev/null | grep -q "Up\|running"; then
    log "重启 server 容器以重新执行迁移..."
    docker compose restart server
    log "等待 10 秒后查看 server 日志..."
    sleep 10
    echo ""
    echo -e "${CYAN}==== server 最近 30 行日志 ====${NC}"
    docker compose logs --tail=30 server 2>&1 || true
    echo ""
    log "如日志中仍有 dirty 错误，说明迁移再次失败，请查看完整日志定位 SQL 错误："
    log "  docker compose logs --tail=200 server | grep -A 5 '迁移'"
else
    log "server 容器未运行，请手动启动：docker compose up -d server"
fi
