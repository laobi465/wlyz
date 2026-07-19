#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— 生成 RSA-4096 密钥对（响应签名用）
# ============================================================
# 用法：
#   bash scripts/gen_rsa_key.sh                  # 默认输出到项目根 keys/
#   bash scripts/gen_rsa_key.sh /path/to/dir     # 指定输出目录
#   bash scripts/gen_rsa_key.sh --force          # 覆盖已存在的密钥
#   bash scripts/gen_rsa_key.sh /path/to/dir --force
#
# 输出文件：
#   - rsa_private.pem  RSA-4096 私钥（PEM/PKCS#8）— 服务端持有，绝不外泄
#   - rsa_public.pem   RSA-4096 公钥（PEM/PKCSIX）— 客户端 SDK 校验签名用
#
# 依赖：openssl 1.1+ 或 3.x（任何发行版自带即可）
# ============================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] [WARN]${NC} $*"; }
err()  { echo -e "${RED}[$(date '+%H:%M:%S')] [ERROR]${NC} $*" >&2; }

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

# ---------- 解析参数 ----------
OUTPUT_DIR="${PROJECT_DIR}/keys"
FORCE=0
for arg in "$@"; do
    case "$arg" in
        --force|-f) FORCE=1 ;;
        -h|--help)
            sed -n '2,15p' "$0"
            exit 0
            ;;
        *)
            if [[ -d "$arg" || ! -e "$arg" ]]; then
                mkdir -p "$arg"
                OUTPUT_DIR="$(cd "$arg" && pwd)"
            else
                err "未知参数: $arg"
                exit 1
            fi
            ;;
    esac
done

# ---------- 检查 openssl ----------
if ! command -v openssl >/dev/null 2>&1; then
    err "未找到 openssl，请先安装：apt install -y openssl  或  yum install -y openssl"
    exit 1
fi

# ---------- 检查目录与文件冲突 ----------
mkdir -p "$OUTPUT_DIR"
PRIVATE_KEY="${OUTPUT_DIR}/rsa_private.pem"
PUBLIC_KEY="${OUTPUT_DIR}/rsa_public.pem"

if [[ $FORCE -eq 0 ]]; then
    if [[ -f "$PRIVATE_KEY" ]]; then
        err "私钥已存在: $PRIVATE_KEY"
        err "如需覆盖，请使用 --force 参数（注意：覆盖后所有已发布的客户端 SDK 将无法校验新签名）"
        exit 1
    fi
    if [[ -f "$PUBLIC_KEY" ]]; then
        err "公钥已存在: $PUBLIC_KEY"
        err "如需覆盖，请使用 --force 参数"
        exit 1
    fi
fi

# ---------- 生成 RSA-4096 密钥对 ----------
log "生成 RSA-4096 密钥对（输出目录: ${OUTPUT_DIR}）..."

# 1. 私钥：PKCS#8 / PEM，无密码保护
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out "$PRIVATE_KEY"
chmod 600 "$PRIVATE_KEY"

# 2. 公钥：从私钥导出，PEM/PKIX
openssl rsa -in "$PRIVATE_KEY" -pubout -out "$PUBLIC_KEY" 2>/dev/null
chmod 644 "$PUBLIC_KEY"

# ---------- 校验 ----------
PRIV_BITS=$(openssl rsa -in "$PRIVATE_KEY" -text -noout 2>/dev/null | grep -oE 'Private-Key: \([0-9]+ bit' | grep -oE '[0-9]+')
if [[ "${PRIV_BITS}" != "4096" ]]; then
    err "私钥位数校验失败（期望 4096，实际 ${PRIV_BITS}）"
    exit 1
fi

# 验证私钥/公钥配对（用私钥签名，公钥校验）
TEST_FILE="$(mktemp)"
TEST_SIG="$(mktemp)"
trap 'rm -f "$TEST_FILE" "$TEST_SIG"' EXIT
echo "keyauth-rsa-key-test-$(date +%s)" > "$TEST_FILE"
openssl dgst -sha256 -sign "$PRIVATE_KEY" -out "$TEST_SIG" "$TEST_FILE"
if ! openssl dgst -sha256 -verify "$PUBLIC_KEY" -signature "$TEST_SIG" "$TEST_FILE" >/dev/null 2>&1; then
    err "密钥对配对校验失败"
    exit 1
fi

# ---------- 完成 ----------
log "RSA-4096 密钥对已生成并通过校验："
echo "  私钥（服务端持有）：$PRIVATE_KEY  (chmod 600)"
echo "  公钥（客户端 SDK）：$PUBLIC_KEY  (chmod 644)"
echo ""
log "下一步："
echo "  1. 将 $PRIVATE_KEY 路径配置到 .env 的 CRYPTO_RSA_PRIVATE_KEY_PATH"
echo "  2. 将 $PUBLIC_KEY 内容嵌入到客户端 SDK，用于校验服务端响应签名"
echo "  3. ⚠️ 私钥绝不外泄；如需轮换，先发布新版本客户端 SDK 再生成新密钥"

warn "请妥善备份私钥（建议加密离线存储）；私钥丢失将无法签发有效响应"
