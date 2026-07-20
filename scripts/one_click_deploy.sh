#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— SSH 一行命令一键自动化部署脚本
# ============================================================
# 功能（一行命令完成全部，无需事先上传源码）：
#   1. 检测操作系统（CentOS/Ubuntu/Debian 系）
#   2. 检测宝塔面板，未安装则拉取官方脚本自动安装
#   3. 检测 Docker，未安装则自动安装（Docker 官方脚本 + 阿里云镜像）
#   4. 自动 git clone 仓库到 /www/wwwroot/keyauth
#   5. 自动生成 RSA-4096 密钥对 + .env 强随机密钥
#   6. docker compose up -d --build 构建并启动 mysql/redis/server/admin
#   7. 等待服务就绪
#   8. 把所有部署信息（宝塔入口/密钥/访问地址/运维命令）写入 /root/keyauth_deploy_info.txt
#
# 用法（远程一行命令，推荐）：
#   sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh)"
#
# 或先下载再执行：
#   curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh -o one_click_deploy.sh
#   sudo bash one_click_deploy.sh
# ============================================================
set -euo pipefail

# ---------- 颜色与日志 ----------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] [WARN]${NC} $*"; }
err()  { echo -e "${RED}[$(date '+%H:%M:%S')] [ERROR]${NC} $*" >&2; }
step() { echo -e "\n${CYAN}${BOLD}==== $* ====${NC}"; }

# ---------- 基础校验 ----------
if [[ $EUID -ne 0 ]]; then
    err "请使用 root 用户执行（sudo bash scripts/one_click_deploy.sh）"
    exit 1
fi

# 项目配置
REPO_URL="https://github.com/laobi465/wlyz.git"
PROJECT_DIR="/www/wwwroot/keyauth"
BT_PANEL_DIR="/www/server/panel"
BT_PORT_FILE="${BT_PANEL_DIR}/data/port.pl"
DEPLOY_INFO_FILE="/root/keyauth_deploy_info.txt"

# 部署状态记录（最终写入 txt）
declare -A DEPLOY_STATE
DEPLOY_STATE[os_pretty]="未知"
DEPLOY_STATE[bt_status]="未安装"
DEPLOY_STATE[bt_url]="未获取"
DEPLOY_STATE[bt_username]="未获取"
DEPLOY_STATE[bt_password]="未获取"
DEPLOY_STATE[bt_port]="8888"
DEPLOY_STATE[docker_status]="未安装"
DEPLOY_STATE[project_dir]="$PROJECT_DIR"
DEPLOY_STATE[mysql_root_pwd]="未生成"
DEPLOY_STATE[mysql_pwd]="未生成"
DEPLOY_STATE[redis_pwd]="未生成"
DEPLOY_STATE[aes_key]="未生成"
DEPLOY_STATE[jwt_secret]="未生成"
DEPLOY_STATE[server_port]="8080"
DEPLOY_STATE[admin_port]="8081"
DEPLOY_STATE[mysql_host_port]="3306"
DEPLOY_STATE[redis_host_port]="6379"
DEPLOY_STATE[port_changed]="未检测"
DEPLOY_STATE[server_ip]="未知"
DEPLOY_STATE[frontend_url]="未知"
DEPLOY_STATE[api_url]="未知"
DEPLOY_STATE[server_ok]="false"
DEPLOY_STATE[mysql_ok]="false"
DEPLOY_STATE[deploy_time]="$(date '+%Y-%m-%d %H:%M:%S')"
DEPLOY_STATE[admin_username]="admin"
DEPLOY_STATE[admin_password]="admin123"
DEPLOY_STATE[admin_init_ok]="false"

# 记录是否全新安装宝塔（影响后续提示）
BT_FRESH_INSTALLED=0

# ---------- Step 1: 检测操作系统 ----------
step "Step 1/10: 检测操作系统"

if [[ ! -f /etc/os-release ]]; then
    err "无法识别操作系统（缺少 /etc/os-release），脚本仅支持主流 Linux 发行版"
    exit 1
fi
. /etc/os-release
OS_ID="${ID:-unknown}"
OS_FAMILY="${ID_LIKE:-}"
OS_PRETTY="${PRETTY_NAME:-$OS_ID}"
DEPLOY_STATE[os_pretty]="$OS_PRETTY"
log "操作系统：$OS_PRETTY"

is_centos=0; is_ubuntu=0
case "$OS_ID" in
    centos|rhel|rocky|almalinux|fedora|anolis|opencloud) is_centos=1 ;;
    ubuntu|debian|linuxmint|deepin|uos) is_ubuntu=1 ;;
    *)
        if echo "$OS_FAMILY" | grep -qi "rhel\|fedora"; then is_centos=1
        elif echo "$OS_FAMILY" | grep -qi "debian\|ubuntu"; then is_ubuntu=1
        else
            err "不支持的操作系统：$OS_ID（仅支持 CentOS/Ubuntu/Debian 系）"
            exit 1
        fi ;;
esac

# ---------- 安装基础工具 ----------
install_pkgs() {
    if [[ $is_centos -eq 1 ]]; then
        yum install -y "$@" >/dev/null 2>&1
    else
        apt-get install -y "$@" >/dev/null 2>&1 || apt install -y "$@" >/dev/null 2>&1
    fi
}

log "检查基础工具（curl wget openssl git）..."
if ! command -v curl >/dev/null 2>&1 || ! command -v wget >/dev/null 2>&1 || \
   ! command -v openssl >/dev/null 2>&1 || ! command -v git >/dev/null 2>&1; then
    if [[ $is_centos -eq 1 ]]; then
        yum makecache -y >/dev/null 2>&1 || true
    else
        apt-get update -y >/dev/null 2>&1 || true
    fi
    install_pkgs curl wget openssl git ca-certificates
fi

command -v curl >/dev/null 2>&1 || { err "curl 安装失败"; exit 1; }
command -v openssl >/dev/null 2>&1 || { err "openssl 安装失败"; exit 1; }
log "基础工具就绪"

# ---------- Step 2: 检测并安装宝塔面板 ----------
step "Step 2/10: 检测宝塔面板"

if [[ -d "$BT_PANEL_DIR" && -f "$BT_PANEL_DIR/BT-Panel" ]]; then
    log "✓ 宝塔面板已安装：$BT_PANEL_DIR"
    DEPLOY_STATE[bt_status]="已安装"
    if [[ -f "$BT_PORT_FILE" ]]; then
        DEPLOY_STATE[bt_port]=$(cat "$BT_PORT_FILE" 2>/dev/null || echo 8888)
        log "  面板端口：${DEPLOY_STATE[bt_port]}"
    fi
else
    log "未检测到宝塔面板，开始拉取官方脚本安装..."
    warn "宝塔安装约需 2-5 分钟，期间会自动应答 y 继续安装"

    cd /tmp || exit 1
    if command -v curl >/dev/null 2>&1; then
        curl -sSO https://download.bt.cn/install/install_panel.sh
    else
        wget -O install_panel.sh https://download.bt.cn/install/install_panel.sh
    fi

    # 自动应答 y 给宝塔安装脚本的交互提示
    # ed8484bec 为宝塔官方推荐标识符（来源：https://www.bt.cn/new/download）
    echo "y" | bash install_panel.sh ed8484bec || {
        err "宝塔面板安装失败，请手动执行官方命令安装后重试"
        err "官方命令：https://www.bt.cn/new/download"
        exit 1
    }

    rm -f install_panel.sh
    cd "$PROJECT_DIR" 2>/dev/null || cd /root

    if [[ ! -d "$BT_PANEL_DIR" ]]; then
        err "宝塔安装完成但目录不存在：$BT_PANEL_DIR"
        exit 1
    fi

    BT_FRESH_INSTALLED=1
    DEPLOY_STATE[bt_status]="全新安装"
    if [[ -f "$BT_PORT_FILE" ]]; then
        DEPLOY_STATE[bt_port]=$(cat "$BT_PORT_FILE" 2>/dev/null || echo 8888)
    fi
    log "✓ 宝塔面板安装完成（端口：${DEPLOY_STATE[bt_port]}）"
fi

# 尝试读取宝塔默认登录信息
if command -v bt >/dev/null 2>&1; then
    BT_DEFAULT_OUT=$(bt default 2>/dev/null || true)
    if [[ -n "$BT_DEFAULT_OUT" ]]; then
        # 解析 "外网面板地址: xxx" / "username: xxx" / "password: xxx"
        BT_URL=$(echo "$BT_DEFAULT_OUT" | grep -iE "面板地址|panel url|外网" | head -1 | sed 's/.*[:：]\s*//' | tr -d ' \r' || true)
        BT_USER=$(echo "$BT_DEFAULT_OUT" | grep -iE "username|账号|用户名" | head -1 | sed 's/.*[:：]\s*//' | tr -d ' \r' || true)
        BT_PWD=$(echo "$BT_DEFAULT_OUT" | grep -iE "password|密码" | head -1 | sed 's/.*[:：]\s*//' | tr -d ' \r' || true)
        [[ -n "$BT_URL" ]] && DEPLOY_STATE[bt_url]="$BT_URL"
        [[ -n "$BT_USER" ]] && DEPLOY_STATE[bt_username]="$BT_USER"
        [[ -n "$BT_PWD" ]] && DEPLOY_STATE[bt_password]="$BT_PWD"
    fi
fi

# ---------- Step 3: 检测并安装 Docker ----------
step "Step 3/10: 检测 Docker 环境"

if ! command -v docker >/dev/null 2>&1; then
    log "未检测到 Docker，开始安装..."

    # 方式 1：优先使用宝塔自带的 Docker 安装脚本
    BT_DOCKER_INSTALLER="${BT_PANEL_DIR}/install/docker_install.sh"
    if [[ -f "$BT_DOCKER_INSTALLER" ]]; then
        log "通过宝塔 Docker 管理器脚本安装..."
        if ! bash "$BT_DOCKER_INSTALLER" >/dev/null 2>&1; then
            warn "宝塔 Docker 脚本执行失败，回退到官方脚本"
            curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
        fi
    else
        log "通过 Docker 官方脚本安装（阿里云镜像加速）..."
        curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
    fi

    systemctl enable docker >/dev/null 2>&1 || true
    systemctl start docker >/dev/null 2>&1 || true
fi

if ! command -v docker >/dev/null 2>&1; then
    err "Docker 安装失败，请通过宝塔软件商店「Docker 管理器」手动安装后重试"
    exit 1
fi
DEPLOY_STATE[docker_status]="已安装 $(docker --version | cut -d' ' -f3 | tr -d ',')"
log "✓ Docker 已就绪：$(docker --version)"

if ! docker compose version >/dev/null 2>&1; then
    err "未检测到 docker compose v2 插件，请通过宝塔软件商店「Docker 管理器」安装 compose 插件后重试"
    exit 1
fi
log "✓ Docker Compose v2 已就绪"

# ---------- Step 4: 拉取/更新项目源码 ----------
step "Step 4/10: 拉取项目源码"

mkdir -p /www/wwwroot
if [[ -d "$PROJECT_DIR/.git" ]]; then
    log "项目已存在，执行 git pull 更新..."
    cd "$PROJECT_DIR"
    git fetch --all >/dev/null 2>&1 || warn "git fetch 失败"
    git reset --hard origin/main >/dev/null 2>&1 || warn "git reset 失败"
    git pull origin main || warn "git pull 失败，使用现有代码继续"
else
    log "从 GitHub 克隆项目到 $PROJECT_DIR ..."
    if ! git clone --depth 1 "$REPO_URL" "$PROJECT_DIR"; then
        err "git clone 失败，请检查服务器是否能访问 GitHub"
        err "如服务器无法访问 GitHub，可在宝塔文件管理上传源码包到 $PROJECT_DIR 后重试"
        exit 1
    fi
    cd "$PROJECT_DIR"
fi
log "✓ 当前目录：$(pwd)"

# ---------- Step 5: 生成 RSA 密钥对 ----------
step "Step 5/10: 生成 RSA-4096 密钥对"

mkdir -p keys
if [[ ! -f keys/rsa_private.pem || ! -f keys/rsa_public.pem ]]; then
    log "生成 RSA-4096 密钥对（响应签名用）..."
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out keys/rsa_private.pem 2>/dev/null
    openssl rsa -in keys/rsa_private.pem -pubout -out keys/rsa_public.pem 2>/dev/null
    chmod 600 keys/rsa_private.pem
    chmod 644 keys/rsa_public.pem
    log "✓ RSA 密钥对已生成：keys/rsa_private.pem / keys/rsa_public.pem"
else
    log "✓ RSA 密钥对已存在，跳过"
fi

# ---------- Step 6: 生成 .env 配置 ----------
step "Step 6/10: 生成 .env 配置文件"

if [[ ! -f .env ]]; then
    if [[ ! -f .env.example ]]; then
        err ".env.example 不存在，请确认项目完整性"
        exit 1
    fi
    log "生成 .env 并自动填充强随机密钥..."
    cp .env.example .env

    # 生成强随机密钥
    # AES_KEY 必须正好 32 字节字符串（后端 config.go 校验 len != 32）
    # 用 openssl rand -hex 16 生成 16 字节随机 → hex 编码 32 字符
    # MySQL/Redis 密码去除 shell 不友好字符 / +=
    MYSQL_ROOT_PWD=$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)
    MYSQL_PWD=$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)
    REDIS_PWD=$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)
    AES_KEY=$(openssl rand -hex 16)
    JWT_SECRET=$(openssl rand -hex 32)

    # 替换占位符（兼容 CHANGE_ME_XXX 形式）
    sed -i "s|^MYSQL_ROOT_PASSWORD=.*|MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PWD}|g" .env
    sed -i "s|^MYSQL_PASSWORD=.*|MYSQL_PASSWORD=${MYSQL_PWD}|g" .env
    sed -i "s|^REDIS_PASSWORD=.*|REDIS_PASSWORD=${REDIS_PWD}|g" .env
    sed -i "s|^AES_KEY=.*|AES_KEY=${AES_KEY}|g" .env
    sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${JWT_SECRET}|g" .env

    # 记录到状态表（写入最终 txt）
    DEPLOY_STATE[mysql_root_pwd]="$MYSQL_ROOT_PWD"
    DEPLOY_STATE[mysql_pwd]="$MYSQL_PWD"
    DEPLOY_STATE[redis_pwd]="$REDIS_PWD"
    DEPLOY_STATE[aes_key]="$AES_KEY"
    DEPLOY_STATE[jwt_secret]="$JWT_SECRET"

    log "✓ .env 已生成并自动填充强随机密钥"
    warn ".env 路径：$(pwd)/.env —— 请妥善备份"
else
    log "✓ .env 已存在，跳过生成（从已有 .env 读取密钥）"
    set +e
    set -a; source .env; set +a
    set -e
    DEPLOY_STATE[mysql_root_pwd]="${MYSQL_ROOT_PASSWORD:-未配置}"
    DEPLOY_STATE[mysql_pwd]="${MYSQL_PASSWORD:-未配置}"
    DEPLOY_STATE[redis_pwd]="${REDIS_PASSWORD:-未配置}"
    DEPLOY_STATE[aes_key]="${AES_KEY:-未配置}"
    DEPLOY_STATE[jwt_secret]="${JWT_SECRET:-未配置}"
fi

# 复制后端配置（如不存在）
if [[ ! -f configs/config.yaml && -f configs/config.yaml.example ]]; then
    cp configs/config.yaml.example configs/config.yaml
    log "✓ configs/config.yaml 已生成"
fi

# 读取端口配置
DEPLOY_STATE[server_port]="${SERVER_PORT:-8080}"
DEPLOY_STATE[admin_port]="${ADMIN_PORT:-8081}"

# ---------- 端口冲突检测（关键：宿主机若已装 MySQL/Redis 会占用 3306/6379）----------
log ""
log "检测端口冲突（MYSQL/REDIS/SERVER/ADMIN 默认 3306/6379/8080/8081）..."
warn "如服务器自带 MySQL/Redis/Nginx，会占用 3306/6379/80 端口导致 docker compose 启动失败"
warn "本步骤自动检测并切换到下一个可用端口（仅改宿主机映射端口，容器内通信端口不变）"

# 端口检测：返回 0 表示占用，1 表示空闲
is_port_in_use() {
    local port="$1"
    if command -v ss >/dev/null 2>&1; then
        ss -tlnH 2>/dev/null | awk '{print $4}' | grep -qE "[.:]${port}\$"
        return $?
    elif command -v netstat >/dev/null 2>&1; then
        netstat -tln 2>/dev/null | awk '{print $4}' | grep -qE "[.:]${port}\$"
        return $?
    else
        # 退化方案：用 bash /dev/tcp 探测（能连 = 端口被占用）
        if timeout 1 bash -c "(echo > /dev/tcp/127.0.0.1/${port})" 2>/dev/null; then
            return 0
        else
            return 1
        fi
    fi
}

# 从 start 开始找下一个可用端口（最多尝试 50 个）
find_free_port() {
    local start="$1"
    local port=$((start))
    local max=$((start + 50))
    while [[ $port -le $max ]]; do
        if ! is_port_in_use "$port"; then
            echo "$port"
            return 0
        fi
        port=$((port + 1))
    done
    echo "$start"
    return 1
}

# 检测并自动切换冲突端口
# 注意：仅改 .env 里的宿主机映射端口（${PORT}:container_port 的左侧），
# 容器之间通信端口固定（mysql:3306 / redis:6379 / server:8080），后端 config.go 的 MYSQL_PORT=3306 指向容器内端口，不受影响
PORT_CHANGED=0
PORT_CHANGELOG=""
for entry in "MYSQL_PORT:${MYSQL_PORT:-3306}" "REDIS_PORT:${REDIS_PORT:-6379}" "SERVER_PORT:${SERVER_PORT:-8080}" "ADMIN_PORT:${ADMIN_PORT:-8081}"; do
    key="${entry%%:*}"
    port="${entry##*:}"
    if is_port_in_use "$port"; then
        new_port=$(find_free_port $((port + 1)))
        if [[ -n "$new_port" && "$new_port" != "$port" ]]; then
            warn "  端口 ${port} 被占用 → ${key} 自动切换到 ${new_port}"
            sed -i "s|^${key}=.*|${key}=${new_port}|g" .env
            PORT_CHANGED=1
            PORT_CHANGELOG="${PORT_CHANGELOG}${key}: ${port}→${new_port}；"
        else
            err "  端口 ${port} 被占用且找不到可用端口，请手动修改 .env 的 ${key} 后重试"
            exit 1
        fi
    else
        log "  ✓ ${key}=${port} 空闲"
    fi
done

# 重新 source .env + 更新状态（端口可能已变）
set +e
set -a; source .env; set +a
set -e
DEPLOY_STATE[server_port]="${SERVER_PORT:-8080}"
DEPLOY_STATE[admin_port]="${ADMIN_PORT:-8081}"
DEPLOY_STATE[mysql_host_port]="${MYSQL_PORT:-3306}"
DEPLOY_STATE[redis_host_port]="${REDIS_PORT:-6379}"

if [[ $PORT_CHANGED -eq 1 ]]; then
    warn "端口已自动调整：${PORT_CHANGELOG}"
    warn "提示：容器之间通信端口不变（mysql:3306/redis:6379/server:8080），仅宿主机映射端口改变"
    DEPLOY_STATE[port_changed]="$PORT_CHANGELOG"
else
    DEPLOY_STATE[port_changed]="无冲突"
fi

# ---------- Step 7: 构建并启动服务 ----------
step "Step 7/10: 构建并启动 Docker 服务"

# 关键：先单独启动 mysql + redis，等 mysql 就绪后清理可能的 dirty 状态，再启动全部服务
# 这样可避免「重试部署时 schema_migrations 残留 dirty 记录 → server 启动被拒绝」的死循环
log "先启动 mysql + redis（确保数据库就绪后再启动 server）..."
docker compose up -d --build mysql redis

log "等待 mysql 就绪（最长 60s）..."
MYSQL_READY=0
for _ in {1..30}; do
    if docker compose exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot -p"${MYSQL_ROOT_PASSWORD}" >/dev/null 2>&1; then
        MYSQL_READY=1
        log "✓ mysql 已就绪"
        break
    fi
    sleep 2
done
[[ $MYSQL_READY -eq 0 ]] && warn "mysql 等待超时，跳过 dirty 清理"

# 清理 schema_migrations 的 dirty 状态（重试部署时常见）
# 成因：之前部署时 mysql 容器异常重启 / 网络断开 / 迁移执行到一半中断
# 后果：schema_migrations 表留下 dirty=1 记录，server 启动时检测到直接拒绝
# 修复：删除 dirty 记录，server 重启后会重新执行该版本迁移
if [[ $MYSQL_READY -eq 1 ]]; then
    DIRTY_COUNT=$(docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
        -N -e "SELECT COUNT(*) FROM schema_migrations WHERE dirty=1;" 2>/dev/null || echo "0")
    if [[ "${DIRTY_COUNT:-0}" -gt 0 ]]; then
        warn "检测到 schema_migrations 有 ${DIRTY_COUNT} 条 dirty 记录，正在清理..."
        DIRTY_VERSIONS=$(docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
            -N -e "SELECT version FROM schema_migrations WHERE dirty=1;" 2>/dev/null || echo "")
        log "  dirty 版本：${DIRTY_VERSIONS}"
        docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
            -e "DELETE FROM schema_migrations WHERE dirty=1;" >/dev/null 2>&1
        log "✓ dirty 记录已清理，server 重启后会重新执行该版本迁移"
    else
        log "✓ 无 dirty 记录"
    fi

    # 额外安全检查：如果 system_update_log 表已存在但 schema_migrations 没有记录 v11
    # （说明 v11 部分执行成功但事务回滚了），手动 DROP 让 v11 能重新创建
    # 注意：只在首次部署场景（sys_admin 表无真实数据）执行，避免破坏业务数据
    ADMIN_REAL_CHECK=$(docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
        -N -e "SELECT COUNT(*) FROM sys_admin WHERE id=1 AND password_hash NOT LIKE '%PLACEHOLDER_BCRYPT_HASH%';" 2>/dev/null || echo "0")
    if [[ "${ADMIN_REAL_CHECK:-0}" -eq 0 ]]; then
        # 首次部署场景，安全清理可能的部分执行残留
        for tbl in system_update_log; do
            TBL_EXISTS=$(docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
                -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${MYSQL_DATABASE:-keyauth}' AND table_name='${tbl}';" 2>/dev/null || echo "0")
            V11_APPLIED=$(docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
                -N -e "SELECT COUNT(*) FROM schema_migrations WHERE version=11;" 2>/dev/null || echo "0")
            if [[ "${TBL_EXISTS:-0}" -eq 1 && "${V11_APPLIED:-0}" -eq 0 ]]; then
                warn "  检测到 ${tbl} 表残留但 v11 未标记完成，DROP 后让迁移重新创建"
                docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
                    -e "DROP TABLE IF EXISTS ${tbl};" >/dev/null 2>&1
            fi
        done
    fi
fi

# 启动所有服务（包括 server 和 admin）
log "启动全部服务（docker compose up -d --build，首次约 5-10 分钟，正在拉取 Go 模块 + 编译）..."
docker compose up -d --build

# ---------- Step 8: 等待服务就绪 ----------
step "Step 8/10: 等待服务就绪"

set +e
set -a; source .env; set +a
set -e

log "等待 MySQL 就绪（最长 60s）..."
for _ in {1..30}; do
    if docker compose exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot -p"${MYSQL_ROOT_PASSWORD}" >/dev/null 2>&1; then
        log "✓ MySQL 已就绪"
        DEPLOY_STATE[mysql_ok]="true"
        break
    fi
    sleep 2
done
[[ "${DEPLOY_STATE[mysql_ok]}" == "false" ]] && warn "MySQL 等待超时，请手动检查：docker compose logs mysql"

log "等待后端 server 就绪（最长 60s）..."
for _ in {1..30}; do
    if curl -sf http://127.0.0.1:${SERVER_PORT:-8080}/health >/dev/null 2>&1; then
        log "✓ 后端 server 已就绪"
        DEPLOY_STATE[server_ok]="true"
        break
    fi
    sleep 2
done
[[ "${DEPLOY_STATE[server_ok]}" == "false" ]] && warn "后端 server 等待超时，请手动检查：docker compose logs server"

# ---------- Step 9: 初始化超管账号 admin/admin123 ----------
step "Step 9/10: 初始化超管账号 admin/admin123"

# 默认初始账号（用户可在此处修改）
DEFAULT_ADMIN_USERNAME="admin"
DEFAULT_ADMIN_PASSWORD="admin123"

log "开始初始化超管账号（默认 ${DEFAULT_ADMIN_USERNAME}/${DEFAULT_ADMIN_PASSWORD}）..."
warn "默认密码 admin123 仅为初始密码，登录后请立即在「个人资料 > 修改密码」中修改"

# 等待 migration 完成（sys_admin 表必须存在）
ADMIN_INIT_OK="false"
for _ in {1..30}; do
    # 检查 sys_admin 表是否存在且有 id=1 的占位行
    ADMIN_CHECK=$(docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
        -N -e "SELECT COUNT(*) FROM sys_admin WHERE id=1;" 2>/dev/null || echo "0")
    if [[ "$ADMIN_CHECK" == "1" ]]; then
        ADMIN_INIT_OK="true"
        break
    fi
    sleep 2
done

if [[ "$ADMIN_INIT_OK" != "true" ]]; then
    warn "sys_admin 表未就绪，跳过自动初始化。请稍后手动执行："
    warn "  cd ${PROJECT_DIR} && bash scripts/reset_admin_password.sh"
else
    # 检查是否仍是占位 hash（首次部署）还是已被设置过（已部署）
    PLACEHOLDER_CHECK=$(docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
        -N -e "SELECT password_hash FROM sys_admin WHERE id=1;" 2>/dev/null | grep -c "PLACEHOLDER_BCRYPT_HASH" || echo "0")

    if [[ "$PLACEHOLDER_CHECK" -ge 1 ]]; then
        log "检测到首次部署（占位 hash），开始写入 admin/admin123..."

        # 用 htpasswd 生成 bcrypt(cost=12) 哈希
        # 确保 htpasswd 可用（apache2-utils / httpd-tools）
        if ! command -v htpasswd >/dev/null 2>&1; then
            log "安装 htpasswd（bcrypt 生成工具）..."
            if [[ $is_centos -eq 1 ]]; then
                yum install -y httpd-tools >/dev/null 2>&1
            else
                apt-get install -y apache2-utils >/dev/null 2>&1 || apt install -y apache2-utils >/dev/null 2>&1
            fi
        fi

        if command -v htpasswd >/dev/null 2>&1; then
            # htpasswd -bnBC 12 输出格式 ":$2y$12$..."，需去掉前导冒号并把 $2y 改为 $2a
            ADMIN_HASH=$(htpasswd -bnBC 12 "" "${DEFAULT_ADMIN_PASSWORD}" | tr -d ':\n' | sed 's/\$2y\$/\$2a\$/')

            if [[ -n "$ADMIN_HASH" ]]; then
                # 转义单引号（避免 SQL 注入，虽然密码固定）
                ESCAPED_HASH=$(echo "$ADMIN_HASH" | sed "s/'/\\\\'/g")
                docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
                    -e "UPDATE sys_admin SET username='${DEFAULT_ADMIN_USERNAME}', password_hash='${ESCAPED_HASH}', status='active' WHERE id=1;" >/dev/null 2>&1

                # 验证
                VERIFY=$(docker compose exec -T mysql mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE:-keyauth}" \
                    -N -e "SELECT username, status FROM sys_admin WHERE id=1;" 2>/dev/null)
                if echo "$VERIFY" | grep -q "^${DEFAULT_ADMIN_USERNAME}"; then
                    log "✓ 超管账号初始化成功：${DEFAULT_ADMIN_USERNAME}/${DEFAULT_ADMIN_PASSWORD}"
                    DEPLOY_STATE[admin_username]="$DEFAULT_ADMIN_USERNAME"
                    DEPLOY_STATE[admin_password]="$DEFAULT_ADMIN_PASSWORD"
                    DEPLOY_STATE[admin_init_ok]="true"
                else
                    warn "超管账号初始化失败，请手动执行：bash scripts/reset_admin_password.sh"
                    DEPLOY_STATE[admin_init_ok]="false"
                fi
            else
                warn "bcrypt 哈希生成失败，请手动执行：bash scripts/reset_admin_password.sh"
                DEPLOY_STATE[admin_init_ok]="false"
            fi
        else
            warn "htpasswd 不可用，无法生成 bcrypt 哈希。请手动执行：bash scripts/reset_admin_password.sh"
            DEPLOY_STATE[admin_init_ok]="false"
        fi
    else
        log "✓ sys_admin 已有真实密码（非首次部署），跳过初始化"
        warn "如需重置密码，请执行：cd ${PROJECT_DIR} && bash scripts/reset_admin_password.sh"
        DEPLOY_STATE[admin_init_ok]="skipped"
        DEPLOY_STATE[admin_username]="$DEFAULT_ADMIN_USERNAME"
        DEPLOY_STATE[admin_password]="（已部署，未修改）"
    fi
fi

# ---------- Step 10: 生成 /root/keyauth_deploy_info.txt ----------
step "Step 10/10: 生成部署信息文件 /root/keyauth_deploy_info.txt"

# 获取服务器 IP：优先公网 IP（curl 公网服务返回），失败回退 hostname -I 第一个 IP
# 关键：hostname -I 在某些云主机返回内网 IP（如 10.0.0.5），用户无法用此 IP 访问
log "获取服务器 IP..."

# 公网 IP 探测服务（顺序尝试，任一成功即用）
PUBLIC_IP=""
for ip_service in "https://api.ipify.org" "https://ifconfig.me" "https://icanhazip.com" "https://ipinfo.io/ip"; do
    PUBLIC_IP=$(curl -fsSL --connect-timeout 5 --max-time 8 "$ip_service" 2>/dev/null | tr -d '[:space:]' || true)
    # 简单校验：必须是合法 IPv4 或 IPv6
    if [[ -n "$PUBLIC_IP" ]] && [[ "$PUBLIC_IP" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ || "$PUBLIC_IP" =~ : ]]; then
        log "  公网 IP（来源 $ip_service）：$PUBLIC_IP"
        break
    fi
    PUBLIC_IP=""
done

# 回退：用 hostname -I 取本机 IP（通常是内网 IP）
if [[ -z "$PUBLIC_IP" ]]; then
    FALLBACK_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
    if [[ -n "$FALLBACK_IP" ]]; then
        warn "  无法获取公网 IP（可能服务器无公网或外网受限），使用本机 IP：$FALLBACK_IP"
        warn "  如这是内网 IP，请手动替换访问地址中的 IP 为你的公网 IP"
        PUBLIC_IP="$FALLBACK_IP"
    fi
fi

if [[ -z "$PUBLIC_IP" ]]; then
    warn "  无法获取服务器 IP，请手动替换访问地址中的 <服务器IP>"
    PUBLIC_IP="<服务器IP>"
fi

SERVER_IP="$PUBLIC_IP"
DEPLOY_STATE[server_ip]="$SERVER_IP"
DEPLOY_STATE[frontend_url]="http://${SERVER_IP}:${DEPLOY_STATE[admin_port]}"
DEPLOY_STATE[api_url]="http://${SERVER_IP}:${DEPLOY_STATE[server_port]}"
DEPLOY_STATE[deploy_time]="$(date '+%Y-%m-%d %H:%M:%S')"

cat > "$DEPLOY_INFO_FILE" <<EOF
================================================================
  KeyAuth SaaS 部署信息 —— 请妥善保存（含敏感信息，勿外传）
================================================================

部署时间：${DEPLOY_STATE[deploy_time]}
操作系统：${DEPLOY_STATE[os_pretty]}
项目目录：${DEPLOY_STATE[project_dir]}
部署信息文件：${DEPLOY_INFO_FILE}

================================================================
  一、宝塔面板信息
================================================================

状态：${DEPLOY_STATE[bt_status]}
面板端口：${DEPLOY_STATE[bt_port]}
面板地址：${DEPLOY_STATE[bt_url]}
账号：${DEPLOY_STATE[bt_username]}
密码：${DEPLOY_STATE[bt_password]}

如以上信息为"未获取"，请在服务器执行：bt default
修改宝塔密码：bt 5
修改宝塔端口：bt 8

================================================================
  二、Docker 环境
================================================================

Docker：${DEPLOY_STATE[docker_status]}
项目容器：mysql / redis / server / admin

端口映射（宿主机:容器内）：
  MySQL  ：${DEPLOY_STATE[mysql_host_port]}:3306
  Redis  ：${DEPLOY_STATE[redis_host_port]}:6379
  后端 API：${DEPLOY_STATE[server_port]}:8080
  前端后台：${DEPLOY_STATE[admin_port]}:80
端口冲突检测：${DEPLOY_STATE[port_changed]}

================================================================
  三、KeyAuth SaaS 访问地址
================================================================

前端后台：${DEPLOY_STATE[frontend_url]}
后端 API：${DEPLOY_STATE[api_url]}
健康检查：${DEPLOY_STATE[api_url]}/health

================================================================
  四、管理员账号（敏感信息，务必保密）
================================================================

后台登录地址：${DEPLOY_STATE[frontend_url]}/admin/login
用户名：${DEPLOY_STATE[admin_username]}
密码：${DEPLOY_STATE[admin_password]}
初始化状态：$([ "${DEPLOY_STATE[admin_init_ok]}" == "true" ] && echo "✓ 已自动初始化" || ([ "${DEPLOY_STATE[admin_init_ok]}" == "skipped" ] && echo "已部署（保留原密码）" || echo "✗ 未初始化，请手动执行 reset_admin_password.sh"))

⚠️ 安全提示：
- admin123 仅为初始密码，登录后请立即在「个人资料 > 修改密码」中修改
- 建议在「个人资料 > 2FA」中开启 TOTP 双因素认证
- 如忘记密码，可在服务器执行：cd ${DEPLOY_STATE[project_dir]} && bash scripts/reset_admin_password.sh

================================================================
  五、密钥与配置（敏感信息，务必保密）
================================================================

文件位置：${DEPLOY_STATE[project_dir]}/.env
RSA 私钥：${DEPLOY_STATE[project_dir]}/keys/rsa_private.pem
RSA 公钥：${DEPLOY_STATE[project_dir]}/keys/rsa_public.pem

MYSQL_ROOT_PASSWORD = ${DEPLOY_STATE[mysql_root_pwd]}
MYSQL_PASSWORD      = ${DEPLOY_STATE[mysql_pwd]}
REDIS_PASSWORD      = ${DEPLOY_STATE[redis_pwd]}
AES_KEY             = ${DEPLOY_STATE[aes_key]}
JWT_SECRET          = ${DEPLOY_STATE[jwt_secret]}

================================================================
  六、服务状态
================================================================

后端 server：$([ "${DEPLOY_STATE[server_ok]}" == "true" ] && echo "✓ 运行中" || echo "✗ 未就绪，请查看日志")
MySQL：      $([ "${DEPLOY_STATE[mysql_ok]}" == "true" ] && echo "✓ 运行中" || echo "✗ 未就绪")

================================================================
  七、宝塔反代 + 免费 SSL 完整教程（推荐，必做）
================================================================

通过宝塔反向代理 + Let's Encrypt 免费 SSL 证书，把域名绑定到 127.0.0.1:${DEPLOY_STATE[admin_port]}，
这样可以通过 https://yourdomain.com 安全访问前端后台，无需暴露 ${DEPLOY_STATE[admin_port]} 端口。

【前置条件】
- 已有域名（如 keyauth.example.com），并把 A 记录解析到本服务器 IP：${DEPLOY_STATE[server_ip]}
- 域名解析生效后才能申请 SSL 证书（一般 5-30 分钟生效）

【步骤 1：宝塔面板「网站」→ 添加站点】
1. 登录宝塔面板：${DEPLOY_STATE[bt_url]}
2. 顶部菜单「网站」→「添加站点」
3. 域名：填写你的域名（如 keyauth.example.com，不带 http://）
4. 根目录：默认 /www/wwwroot/你的域名
5. PHP 版本：纯静态
6. 数据库：不创建
7. 点击「提交」

【步骤 2：配置反向代理】
1. 在网站列表找到刚创建的站点，点「设置」→「反向代理」
2. 点击「添加反向代理」
3. 代理名称：keyauth
4. 目标URL：http://127.0.0.1:${DEPLOY_STATE[admin_port]}
5. 发送域名：\$host
6. 内容替换：留空
7. 开启「代理」开关，点「提交」

【步骤 3：申请免费 SSL 证书（Let's Encrypt）】
1. 在站点设置里点「SSL」→「Let's Encrypt」
2. 勾选你的域名
3. 点「申请」→ 等待 10-60 秒（宝塔自动完成验证 + 部署）
4. 申请成功后，开启「强制 HTTPS」开关

【步骤 4：如果前端调用了 API，需同步配置 API 反代】
如果你的后端 API 也想用独立域名（如 api.example.com）：
1. 重复步骤 1-3，新建站点绑定 api.example.com
2. 反代目标URL 改为：http://127.0.0.1:${DEPLOY_STATE[server_port]}
3. 申请 SSL 同步骤 3

【步骤 5：在 KeyAuth 后台配置平台域名】
1. 浏览器访问：https://你的域名/admin/login
2. 用 admin/admin123 登录
3. 进入「系统配置」→ 修改 platform.domain 为你的域名
4. 进入「系统配置 > 支付」配置易支付参数（支付回调地址会用到域名）

【步骤 6：防火墙安全（重要）】
1. 宝塔面板「安全」→ 释放端口只保留：22 80 443 ${DEPLOY_STATE[bt_port]}
2. 关闭公网访问：${DEPLOY_STATE[admin_port]}（前端） ${DEPLOY_STATE[server_port]}（后端 API）
   ${DEPLOY_STATE[mysql_host_port]}（MySQL） ${DEPLOY_STATE[redis_host_port]}（Redis）—— 这四个务必关闭公网！
3. 云服务商安全组（阿里云/腾讯云）也只放行：22 80 443 ${DEPLOY_STATE[bt_port]}

【常见问题】
Q: SSL 申请失败提示「域名未解析」？
A: 检查域名 A 记录是否已生效（nslookup yourdomain.com 应返回服务器 IP）

Q: 反代后访问 502 Bad Gateway？
A: 检查后端容器是否运行：docker compose ps，确认 keyauth-server 是 Up 状态

Q: docker compose 启动报 "address already in use"？
A: 宿主机已有服务占用了默认端口（3306/6379/8080/8081）。本脚本已自动检测并切换到下一个可用端口；
   如仍报错，请手动修改 .env 里的 MYSQL_PORT/REDIS_PORT/SERVER_PORT/ADMIN_PORT 为未占用端口。

Q: HTTPS 后部分资源加载失败（mixed content）？
A: 已通过强制 HTTPS 解决；如仍有问题检查浏览器控制台报错

================================================================
  八、常用运维命令
================================================================

cd ${DEPLOY_STATE[project_dir]}

# 查看服务状态
docker compose ps

# 查看后端实时日志
docker compose logs -f server

# 重启后端
docker compose restart server

# 代码更新后重新构建
git pull origin main && docker compose up -d --build server admin

# 进入 MySQL
docker compose exec mysql mysql -ukeyauth -p${DEPLOY_STATE[mysql_pwd]} keyauth

# 重置超管密码
bash scripts/reset_admin_password.sh

# 停止全部服务（数据保留）
docker compose down

# 停止并删除数据（谨慎！）
docker compose down -v

================================================================
  九、备份建议
================================================================

定期备份以下文件到安全位置：
1. ${DEPLOY_STATE[project_dir]}/.env                   （所有密钥）
2. ${DEPLOY_STATE[project_dir]}/keys/rsa_private.pem   （RSA 私钥）
3. ${DEPLOY_STATE[project_dir]}/keys/rsa_public.pem    （RSA 公钥）
4. MySQL 数据卷（docker volume inspect keyauth_mysql-data）
5. 本部署信息文件：${DEPLOY_INFO_FILE}

================================================================
EOF

chmod 600 "$DEPLOY_INFO_FILE"

# ---------- 最终输出 ----------
echo ""
echo -e "${GREEN}${BOLD}==================== 部署完成 ====================${NC}"
echo ""
echo -e "${BOLD}服务状态：${NC}"
echo "  后端 API：    $([ "${DEPLOY_STATE[server_ok]}" == "true" ] && echo "✓ 运行中" || echo "✗ 未就绪")"
echo "  MySQL：       $([ "${DEPLOY_STATE[mysql_ok]}" == "true" ] && echo "✓ 运行中" || echo "✗ 未就绪")"
echo ""
echo -e "${BOLD}访问地址：${NC}"
echo "  前端后台：    ${DEPLOY_STATE[frontend_url]}"
echo "  后端 API：    ${DEPLOY_STATE[api_url]}"
echo "  宝塔面板：    http://${DEPLOY_STATE[server_ip]}:${DEPLOY_STATE[bt_port]}"
echo ""
echo -e "${BOLD}超管账号：${NC}"
if [[ "${DEPLOY_STATE[admin_init_ok]}" == "true" ]]; then
    echo -e "  用户名：      ${GREEN}${DEPLOY_STATE[admin_username]}${NC}"
    echo -e "  密码：        ${GREEN}${DEPLOY_STATE[admin_password]}${NC}"
    echo -e "  ${YELLOW}⚠️ 登录后请立即在「个人资料 > 修改密码」中修改默认密码${NC}"
elif [[ "${DEPLOY_STATE[admin_init_ok]}" == "skipped" ]]; then
    echo -e "  ${YELLOW}已部署环境，保留原密码${NC}"
else
    echo -e "  ${RED}✗ 自动初始化失败，请手动执行：bash scripts/reset_admin_password.sh${NC}"
fi
echo ""
echo -e "${GREEN}${BOLD}==================== 重要：部署信息已保存 ====================${NC}"
echo ""
echo -e "${BOLD}所有部署信息（含宝塔账号/管理员账号/密钥/反代教程/运维命令）已写入：${NC}"
echo -e "  ${CYAN}${DEPLOY_INFO_FILE}${NC}"
echo ""
echo -e "${BOLD}查看部署信息：${NC}"
echo "  cat $DEPLOY_INFO_FILE"
echo ""
echo -e "${CYAN}${BOLD}==================== 下一步必做（按顺序执行）====================${NC}"
echo ""
echo -e "${BOLD}[验证部署]${NC} 立即可做（无需域名）"
echo "  1. 浏览器打开：${DEPLOY_STATE[frontend_url]}/admin/login"
echo "  2. 用 ${GREEN}admin / ${DEPLOY_STATE[admin_password]}${NC} 登录测试"
echo "  3. 如登录失败，查后端日志：cd ${DEPLOY_STATE[project_dir]} && docker compose logs --tail=50 server"
echo ""
echo -e "${BOLD}[安全加固]${NC} 登录成功后立即做"
echo "  4. 进入「个人资料 > 修改密码」改掉默认密码 admin123"
echo "  5. 进入「个人资料 > 2FA」开启 TOTP 双因素认证"
echo ""
echo -e "${BOLD}[域名 + HTTPS]${NC} 有域名时做（详见 txt 第七章反代教程）"
echo "  6. 域名 A 记录解析到本服务器 IP（${DEPLOY_STATE[server_ip]}）"
echo "  7. 宝塔「网站」→ 添加站点 → 反代到 http://127.0.0.1:${DEPLOY_STATE[admin_port]}"
echo "  8. 宝塔「SSL」→ Let's Encrypt → 申请免费证书 → 强制 HTTPS"
echo "  9. 登录后台「系统配置」→ 修改 platform.domain 为你的域名"
echo ""
echo -e "${BOLD}[防火墙]${NC} 域名 + SSL 配置完成后做"
echo "  10. 宝塔「安全」→ 只放行 22 80 443 ${DEPLOY_STATE[bt_port]}"
echo "  11. 关闭公网端口：${DEPLOY_STATE[admin_port]} ${DEPLOY_STATE[server_port]} ${DEPLOY_STATE[mysql_host_port]} ${DEPLOY_STATE[redis_host_port]}"
echo ""
echo -e "${BOLD}[完整文档]${NC}"
echo "  cat $DEPLOY_INFO_FILE"
echo "  （含宝塔入口/管理员账号/密钥/反代教程/运维命令/备份建议 9 大章节）"
echo ""
if [[ $PORT_CHANGED -eq 1 ]]; then
    echo -e "${YELLOW}⚠️ 端口已自动调整（避免与宿主机已有服务冲突）：${NC}"
    echo "  ${PORT_CHANGELOG}"
    echo "  容器之间通信端口不变，仅宿主机映射端口改变（详见 .env）"
    echo ""
fi
if [[ $BT_FRESH_INSTALLED -eq 1 ]]; then
    echo -e "${YELLOW}⚠️ 首次安装宝塔，请登录面板后立即修改默认账号密码和端口${NC}"
    echo ""
fi
echo -e "${YELLOW}提示：${DEPLOY_INFO_FILE} 已设置 chmod 600（仅 root 可读），含敏感信息，请妥善保管${NC}"
echo -e "${GREEN}==================================================================${NC}"
