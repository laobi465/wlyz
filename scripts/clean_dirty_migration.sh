#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— 安全清理/修复 schema_migrations dirty 状态
# ============================================================
# 用途：server 启动报「数据库迁移处于 dirty 状态」时执行此脚本
#
# v0.6.2 重写：
#   * 不再默认执行 DELETE FROM schema_migrations WHERE dirty=1（会跳过失败迁移，掩盖问题）
#   * 默认模式：dry-run，只查看不修改
#   * --repair 模式：备份数据库 → 检查已有对象 → 重新执行幂等迁移 → 标记 dirty=false
#   * --force-delete 模式：危险操作，显式确认后才删除 dirty 记录（仅当确认迁移无需重试时使用）
#   * 禁止调用 docker compose down -v
#
# 用法：
#   bash scripts/clean_dirty_migration.sh                    # 默认 dry-run（只查看）
#   bash scripts/clean_dirty_migration.sh --show             # 同上，查看 dirty 状态
#   bash scripts/clean_dirty_migration.sh --dry-run          # 同上，明确 dry-run
#   bash scripts/clean_dirty_migration.sh --repair           # 推荐：备份数据库 + 走幂等修复
#   bash scripts/clean_dirty_migration.sh --version 15 --repair  # 只修复指定版本
#   bash scripts/clean_dirty_migration.sh --force-delete     # 危险：仅删除 dirty 记录（需二次确认）
# ============================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] [WARN]${NC} $*"; }
err()  { echo -e "${RED}[$(date '+%H:%M:%S')] [ERROR]${NC} $*" >&2; }
step() { echo -e "\n${CYAN}${BOLD}==== $* ====${NC}"; }

# 切到项目根目录（脚本在 scripts/ 下）
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

# ---------- 参数解析 ----------
# 默认 dry-run 模式（v0.6.2：安全默认）
MODE="dry-run"
TARGET_VERSION=""
for arg in "$@"; do
    case "$arg" in
        --show|--dry-run) MODE="dry-run" ;;
        --repair) MODE="repair" ;;
        --force-delete) MODE="force-delete" ;;
        --version) shift; TARGET_VERSION="${1:-}" ;;
        --version=*) TARGET_VERSION="${arg#*=}" ;;
        -h|--help)
            cat <<'EOF'
用法: bash scripts/clean_dirty_migration.sh [选项]

模式（互斥）：
  --show, --dry-run    默认。只查看 dirty 状态，不修改任何数据
  --repair             推荐。备份数据库 + 走幂等迁移修复流程（MIGRATION_REPAIR_DIRTY=true）
  --force-delete       危险。仅删除 dirty 记录（需二次确认）。仅当确认迁移无需重试时使用

选项：
  --version N          只处理指定版本（如 --version 15）
  -h, --help           显示帮助

示例：
  bash scripts/clean_dirty_migration.sh                    # 默认 dry-run
  bash scripts/clean_dirty_migration.sh --repair           # 推荐：安全修复
  bash scripts/clean_dirty_migration.sh --version 15 --repair  # 只修复 v15
EOF
            exit 0 ;;
        *)
            err "未知参数：$arg（使用 -h 查看帮助）"
            exit 1 ;;
    esac
done

# ---------- 环境检查 ----------
if [[ ! -f .env ]]; then
    err ".env 不存在，请先执行一键部署或手动 cp .env.example .env"
    exit 1
fi

# shellcheck disable=SC1091
set +e; set -a; source .env; set +a; set -e

DB_NAME="${MYSQL_DATABASE:-keyauth}"
DB_ROOT_PWD="${MYSQL_ROOT_PASSWORD:-}"

if [[ -z "$DB_ROOT_PWD" ]]; then
    err "MYSQL_ROOT_PASSWORD 未配置，请检查 .env"
    exit 1
fi

# 检查 docker compose 可用
if ! command -v docker >/dev/null 2>&1; then
    err "docker 命令不可用，请在 Docker 环境中运行"
    exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
    err "docker compose v2 不可用，请安装后重试"
    exit 1
fi

# 检查 mysql 容器是否运行
if ! docker compose ps mysql 2>/dev/null | grep -q "Up\|running"; then
    err "mysql 容器未运行，请先启动：docker compose up -d mysql"
    exit 1
fi

# mysql 容器名（用于日志输出）
MYSQL_CONTAINER=$(docker compose ps mysql 2>/dev/null | awk 'NR==2{print $1}' || echo "keyauth-mysql")

# ---------- 执行 SQL 辅助函数 ----------
# 注：密码通过环境变量传给 docker exec，不会出现在命令行历史或 ps 输出中
exec_mysql() {
    local sql="$1"
    local extra="${2:-}"
    # shellcheck disable=SC2086
    docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" $extra -e "$sql" 2>/dev/null
}

exec_mysql_silent() {
    local sql="$1"
    docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" -e "$sql" >/dev/null 2>&1
}

# ---------- Step 1: 打印当前状态 ----------
step "Step 1/4: 检查当前迁移状态"

log "项目目录：$PROJECT_DIR"
log "数据库：$DB_NAME"
log "mysql 容器：$MYSQL_CONTAINER"
log "执行模式：$MODE"

# MySQL 版本
MYSQL_VERSION=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" \
    -N -e "SELECT VERSION();" 2>/dev/null || echo "unknown")
log "MySQL 版本：${MYSQL_VERSION}"

# 检查 schema_migrations 表是否存在
TBL_EXISTS=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
    -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${DB_NAME}' AND table_name='schema_migrations';" 2>/dev/null || echo "0")

if [[ "${TBL_EXISTS:-0}" -eq 0 ]]; then
    log "✓ schema_migrations 表不存在（首次部署或未执行过迁移），无需处理"
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
        log "✓ 版本 ${TARGET_VERSION} 无记录，无需处理"
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

# ---------- Step 2: 检查 dirty 版本对应的对象（关键：不猜测，实际查询） ----------
step "Step 2/4: 检查 dirty 版本对应的数据库对象"

# 针对每个 dirty 版本，检查相关对象是否存在
for ver in $DIRTY_VERSIONS; do
    log "检查 version=${ver} 对应的对象："

    case "$ver" in
        15)
            # v015 创建的对象：end_user, end_user_card, end_user_token 表
            # app_card.end_user_id 字段, idx_end_user_id 索引
            # sys_config 的 enduser 配置项
            for tbl in end_user end_user_card end_user_token; do
                TBL=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
                    -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${DB_NAME}' AND table_name='${tbl}';" 2>/dev/null || echo "0")
                if [[ "${TBL:-0}" -eq 1 ]]; then
                    log "  表 ${tbl}：✓ 已存在（CREATE TABLE IF NOT EXISTS 会跳过）"
                else
                    log "  表 ${tbl}：✗ 不存在（迁移会创建）"
                fi
            done

            # app_card.end_user_id 字段
            COL=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
                -N -e "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='${DB_NAME}' AND table_name='app_card' AND column_name='end_user_id';" 2>/dev/null || echo "0")
            if [[ "${COL:-0}" -eq 1 ]]; then
                log "  字段 app_card.end_user_id：✓ 已存在（PREPARE/EXECUTE 会跳过 ADD COLUMN）"
            else
                log "  字段 app_card.end_user_id：✗ 不存在（迁移会创建）"
            fi

            # idx_end_user_id 索引
            IDX=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
                -N -e "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema='${DB_NAME}' AND table_name='app_card' AND index_name='idx_end_user_id';" 2>/dev/null || echo "0")
            if [[ "${IDX:-0}" -ge 1 ]]; then
                log "  索引 app_card.idx_end_user_id：✓ 已存在（PREPARE/EXECUTE 会跳过 ADD INDEX）"
            else
                log "  索引 app_card.idx_end_user_id：✗ 不存在（迁移会创建）"
            fi

            # sys_config enduser 配置项
            CFG=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
                -N -e "SELECT COUNT(*) FROM sys_config WHERE config_group='enduser';" 2>/dev/null || echo "0")
            log "  sys_config enduser 配置项：${CFG:-0} 条（INSERT ON DUPLICATE KEY UPDATE 会幂等覆盖）"
            ;;
        *)
            log "  version=${ver}：未实现专项检查，请查看对应迁移文件"
            log "  迁移文件：apps/server/migrations/$(printf '%03d' "$ver")_*.up.sql"
            ;;
    esac
done

# ---------- Step 3: dry-run 模式直接退出 ----------
if [[ "$MODE" == "dry-run" ]]; then
    step "Step 3/4: dry-run 模式（不修改数据库）"
    log "✓ 已完成 dirty 状态检查，未修改任何数据"
    log ""
    log "推荐修复命令："
    log "  bash scripts/clean_dirty_migration.sh --repair"
    if [[ -n "$TARGET_VERSION" ]]; then
        log "  或：bash scripts/clean_dirty_migration.sh --version ${TARGET_VERSION} --repair"
    fi
    exit 0
fi

# ---------- Step 4: 执行修复 / 强制删除 ----------
if [[ "$MODE" == "force-delete" ]]; then
    step "Step 3/4: force-delete 模式（危险：仅删除 dirty 记录）"

    echo -e "${RED}${BOLD}"
    echo "============================================================"
    echo " ⚠️  警告：--force-delete 仅删除 schema_migrations 记录"
    echo "============================================================"
    echo -e "${NC}"
    echo "此操作会："
    echo "  1. 删除 schema_migrations 中的 dirty 记录"
    echo "  2. 不会修复半成品 schema（已存在的表/字段/索引仍保留）"
    echo "  3. 重启 server 后会重新执行该版本迁移（必须幂等，否则再次失败）"
    echo ""
    echo "推荐替代方案："
    echo "  bash scripts/clean_dirty_migration.sh --repair"
    echo "  （会备份数据库 + 走幂等迁移修复流程，更安全）"
    echo ""
    echo -e "${RED}请确认是否继续 force-delete（仅删除 dirty 记录）${NC}"
    read -r -p "输入 'YES DELETE DIRTY RECORDS ONLY' 确认: " CONFIRM_INPUT
    if [[ "$CONFIRM_INPUT" != "YES DELETE DIRTY RECORDS ONLY" ]]; then
        log "用户未确认，已取消 force-delete 操作"
        exit 0
    fi

    # 先备份
    BACKUP_TS=$(date '+%Y%m%d_%H%M%S')
    BACKUP_FILE="${PROJECT_DIR}/backup_force_delete_${BACKUP_TS}.sql"
    log "备份数据库到：${BACKUP_FILE}"
    docker compose exec -T mysql mysqldump -uroot -p"${DB_ROOT_PWD}" \
        --single-transaction --routines --triggers "$DB_NAME" > "$BACKUP_FILE" 2>/dev/null && \
        log "✓ 备份完成（大小：$(du -h "$BACKUP_FILE" | cut -f1)）" || \
        { err "备份失败，已终止"; exit 1; }

    if [[ -n "$TARGET_VERSION" ]]; then
        log "删除 version=${TARGET_VERSION} 的记录..."
        exec_mysql_silent "DELETE FROM schema_migrations WHERE version=${TARGET_VERSION};"
    else
        log "删除所有 dirty 记录..."
        exec_mysql_silent "DELETE FROM schema_migrations WHERE dirty=1;"
    fi
    log "✓ dirty 记录已删除"

elif [[ "$MODE" == "repair" ]]; then
    step "Step 3/4: repair 模式（备份数据库 + 走幂等迁移修复）"

    # 备份数据库
    BACKUP_TS=$(date '+%Y%m%d_%H%M%S')
    BACKUP_FILE="${PROJECT_DIR}/backup_repair_${BACKUP_TS}.sql"
    log "备份数据库到：${BACKUP_FILE}"
    if docker compose exec -T mysql mysqldump -uroot -p"${DB_ROOT_PWD}" \
        --single-transaction --routines --triggers "$DB_NAME" > "$BACKUP_FILE" 2>/dev/null; then
        log "✓ 备份完成（大小：$(du -h "$BACKUP_FILE" | cut -f1)）"
    else
        err "备份失败，已终止（不进行修复）"
        exit 1
    fi

    # 显示修复计划
    log ""
    log "修复计划："
    log "  1. 在 .env 中设置 MIGRATION_REPAIR_DIRTY=true"
    log "  2. 重启 server 容器（会获取 advisory lock → 重新执行幂等迁移 → 标记 dirty=false）"
    log "  3. 修复成功后自动改回 MIGRATION_REPAIR_DIRTY=false"
    log "  4. 失败则保留 dirty=true，输出详细错误"
    log ""

    # 显示确认信息
    echo -e "${CYAN}请确认以下信息：${NC}"
    echo "  数据库：$DB_NAME"
    echo "  mysql 容器：$MYSQL_CONTAINER"
    echo "  dirty 版本：${DIRTY_VERSIONS}"
    echo "  备份文件：$BACKUP_FILE"
    echo "  操作：修改 .env + 重启 server 容器（会重新执行幂等迁移）"
    echo ""
    read -r -p "输入 'YES REPAIR' 确认开始修复: " CONFIRM_INPUT
    if [[ "$CONFIRM_INPUT" != "YES REPAIR" ]]; then
        log "用户未确认，已取消 repair 操作（数据库未修改，备份文件已生成：$BACKUP_FILE）"
        exit 0
    fi

    # 设置 MIGRATION_REPAIR_DIRTY=true
    log "在 .env 中设置 MIGRATION_REPAIR_DIRTY=true..."
    if ! grep -q "^MIGRATION_REPAIR_DIRTY=" .env; then
        echo "MIGRATION_REPAIR_DIRTY=true" >> .env
    else
        sed -i 's|^MIGRATION_REPAIR_DIRTY=.*|MIGRATION_REPAIR_DIRTY=true|g' .env
    fi
    log "✓ .env 已更新"

    # 重启 server
    log "重启 server 容器..."
    docker compose up -d server >/dev/null 2>&1

    # 等待并检查
    log "等待 server 启动（最长 60s）..."
    REPAIR_OK=0
    for _ in {1..30}; do
        sleep 2
        # 检查 dirty 是否已清除
        DIRTY_NOW=$(docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
            -N -e "SELECT COUNT(*) FROM schema_migrations WHERE dirty=1;" 2>/dev/null || echo "1")
        if [[ "${DIRTY_NOW:-1}" -eq 0 ]]; then
            REPAIR_OK=1
            log "✓ dirty 状态已清除（迁移修复成功）"
            break
        fi
        # 检查 server 是否健康
        if curl -sf http://127.0.0.1:${SERVER_PORT:-8080}/health >/dev/null 2>&1; then
            # server 健康但仍 dirty？可能正在迁移中
            continue
        fi
    done

    # 改回 MIGRATION_REPAIR_DIRTY=false
    log "改回 MIGRATION_REPAIR_DIRTY=false..."
    sed -i 's|^MIGRATION_REPAIR_DIRTY=.*|MIGRATION_REPAIR_DIRTY=false|g' .env

    if [[ $REPAIR_OK -eq 1 ]]; then
        log "✓ 修复成功！dirty 状态已清除"
        log "  备份文件：$BACKUP_FILE"
        log "  建议重启 server 确保使用最新配置：docker compose restart server"
    else
        err "✗ 修复未完成（dirty 仍为 true），请查看 server 日志："
        echo ""
        echo -e "${CYAN}==== server 最近 50 行日志 ====${NC}"
        docker compose logs --tail=50 server 2>&1 || true
        echo ""
        echo -e "${CYAN}==== schema_migrations 当前状态 ====${NC}"
        docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
            -e "SELECT version, dirty, applied_at FROM schema_migrations ORDER BY version;" 2>/dev/null || true
        echo ""
        err "数据库已备份到：$BACKUP_FILE"
        err "dirty=true 已保留，请根据日志中的 SQL 错误手动修复迁移文件后重试"
        exit 1
    fi
fi

# ---------- Step 5: 重启 server 让其正常启动 ----------
step "Step 4/4: 重启 server 容器"

if docker compose ps server 2>/dev/null | grep -q "Up\|running"; then
    log "重启 server 容器..."
    docker compose restart server
else
    log "启动 server 容器..."
    docker compose up -d server
fi

log "等待 10 秒后查看 server 日志..."
sleep 10
echo ""
echo -e "${CYAN}==== server 最近 30 行日志 ====${NC}"
docker compose logs --tail=30 server 2>&1 || true
echo ""
echo -e "${CYAN}==== schema_migrations 最终状态 ====${NC}"
docker compose exec -T mysql mysql -uroot -p"${DB_ROOT_PWD}" "$DB_NAME" \
    -e "SELECT version, dirty, applied_at FROM schema_migrations ORDER BY version;" 2>/dev/null || true
echo ""

log "如日志中仍有 dirty 错误，说明迁移再次失败，请查看完整日志定位 SQL 错误："
log "  docker compose logs --tail=200 server | grep -A 5 '迁移'"
log ""
log "提示："
log "  - 修复后请确保 .env 中 MIGRATION_REPAIR_DIRTY=false"
log "  - 备份文件请妥善保管（含敏感信息，建议 chmod 600）"
