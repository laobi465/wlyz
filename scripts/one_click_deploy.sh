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
DEPLOY_STATE[server_ip]="未知"
DEPLOY_STATE[frontend_url]="未知"
DEPLOY_STATE[api_url]="未知"
DEPLOY_STATE[server_ok]="false"
DEPLOY_STATE[mysql_ok]="false"
DEPLOY_STATE[deploy_time]="$(date '+%Y-%m-%d %H:%M:%S')"

# 记录是否全新安装宝塔（影响后续提示）
BT_FRESH_INSTALLED=0

# ---------- Step 1: 检测操作系统 ----------
step "Step 1/9: 检测操作系统"

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
step "Step 2/9: 检测宝塔面板"

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
step "Step 3/9: 检测 Docker 环境"

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
step "Step 4/9: 拉取项目源码"

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
step "Step 5/9: 生成 RSA-4096 密钥对"

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
step "Step 6/9: 生成 .env 配置文件"

if [[ ! -f .env ]]; then
    if [[ ! -f .env.example ]]; then
        err ".env.example 不存在，请确认项目完整性"
        exit 1
    fi
    log "生成 .env 并自动填充强随机密钥..."
    cp .env.example .env

    # 生成强随机密钥（tr 去除 shell 不友好字符 / +=）
    MYSQL_ROOT_PWD=$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)
    MYSQL_PWD=$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)
    REDIS_PWD=$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)
    AES_KEY=$(openssl rand -base64 32)
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

# ---------- Step 7: 构建并启动服务 ----------
step "Step 7/9: 构建并启动 Docker 服务"

log "执行 docker compose up -d --build（首次约 5-10 分钟，正在拉取 Go 模块 + 编译）..."
docker compose up -d --build

# ---------- Step 8: 等待服务就绪 ----------
step "Step 8/9: 等待服务就绪"

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

# ---------- Step 9: 生成 /root/keyauth_deploy_info.txt ----------
step "Step 9/9: 生成部署信息文件 /root/keyauth_deploy_info.txt"

SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[[ -z "$SERVER_IP" ]] && SERVER_IP="<你的服务器IP>"
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

================================================================
  三、KeyAuth SaaS 访问地址
================================================================

前端后台：${DEPLOY_STATE[frontend_url]}
后端 API：${DEPLOY_STATE[api_url]}
健康检查：${DEPLOY_STATE[api_url]}/health

================================================================
  四、密钥与配置（敏感信息，务必保密）
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
  五、服务状态
================================================================

后端 server：$([ "${DEPLOY_STATE[server_ok]}" == "true" ] && echo "✓ 运行中" || echo "✗ 未就绪，请查看日志")
MySQL：      $([ "${DEPLOY_STATE[mysql_ok]}" == "true" ] && echo "✓ 运行中" || echo "✗ 未就绪")

================================================================
  六、下一步操作（必做）
================================================================

1. 重置超管密码（首次部署必须）：
   cd ${DEPLOY_STATE[project_dir]} && bash scripts/reset_admin_password.sh

2. 在宝塔面板「安全」中关闭 MySQL(3306)/Redis(6379) 公网端口

3. 在宝塔面板「网站」绑定域名 + 申请 SSL 证书，反代到 :${DEPLOY_STATE[admin_port]}

4. 登录后台「系统配置 > 支付」配置平台易支付参数

$([ $BT_FRESH_INSTALLED -eq 1 ] && echo "5. 首次安装宝塔，请登录面板后修改默认账号密码和端口" || echo "")

================================================================
  七、常用运维命令
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

# 停止全部服务（数据保留）
docker compose down

# 停止并删除数据（谨慎！）
docker compose down -v

================================================================
  八、备份建议
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
echo -e "${GREEN}${BOLD}==================== 重要：部署信息已保存 ====================${NC}"
echo ""
echo -e "${BOLD}所有部署信息（含宝塔账号、密钥、访问地址、运维命令）已写入：${NC}"
echo -e "  ${CYAN}${DEPLOY_INFO_FILE}${NC}"
echo ""
echo -e "${BOLD}查看部署信息：${NC}"
echo "  cat $DEPLOY_INFO_FILE"
echo ""
echo -e "${BOLD}下一步必做：${NC}"
echo "  1. 查看部署信息：cat $DEPLOY_INFO_FILE"
echo "  2. 重置超管密码：cd ${DEPLOY_STATE[project_dir]} && bash scripts/reset_admin_password.sh"
echo "  3. 宝塔面板「安全」关闭 MySQL/Redis 公网端口"
echo "  4. 宝塔面板「网站」绑定域名 + 申请 SSL 证书"
if [[ $BT_FRESH_INSTALLED -eq 1 ]]; then
    echo "  5. 首次安装宝塔，请登录面板后修改默认账号密码和端口"
fi
echo ""
echo -e "${YELLOW}提示：${DEPLOY_INFO_FILE} 已设置 chmod 600（仅 root 可读），含敏感信息，请妥善保管${NC}"
echo -e "${GREEN}==================================================${NC}"
