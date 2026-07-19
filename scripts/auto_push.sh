#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— 自动推送脚本
# ============================================================
# 用法：
#   bash scripts/auto_push.sh                    # 用已配置凭证推送
#   bash scripts/auto_push.sh ghp_xxxx           # 用指定 token 推送
#
# 行为：
#   1. git add -A（暂存所有变更）
#   2. 如有变更则自动 commit
#   3. git push 到 origin main
# ============================================================
set -euo pipefail

GREEN='\033[0;32m'; YELLOW='\033[0;33m'; RED='\033[0;31m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] [WARN]${NC} $*"; }
err()  { echo -e "${RED}[$(date '+%H:%M:%S')] [ERROR]${NC} $*" >&2; }

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

TOKEN="${1:-}"
REMOTE_URL="https://github.com/laobi465/wlyz.git"
BRANCH="main"

# ---------- 1. 自动 commit ----------
log "暂存所有变更..."
git add -A

if git diff --cached --quiet; then
    log "无变更需要提交"
else
    COMMIT_MSG="chore: 自动推送 $(date '+%Y-%m-%d %H:%M:%S')"
    # 检查是否有 user.name 配置，没有则用临时身份
    if git config user.name >/dev/null 2>&1; then
        git commit -m "$COMMIT_MSG" >/dev/null
    else
        git -c user.name="Trae Agent" -c user.email="agent@trae.local" commit -m "$COMMIT_MSG" >/dev/null
    fi
    log "已提交：$COMMIT_MSG"
fi

# ---------- 2. 推送 ----------
log "推送到 $REMOTE_URL ($BRANCH)..."

if [[ -n "$TOKEN" ]]; then
    # 用 token 推送（不写入 config）
    PUSH_URL="https://x-access-token:${TOKEN}@github.com/laobi465/wlyz.git"
    if git push "$PUSH_URL" "$BRANCH" 2>&1; then
        log "推送成功（使用 token）"
    else
        err "推送失败：token 可能无效或已过期"
        exit 1
    fi
else
    # 用已配置的凭证推送
    if git push origin "$BRANCH" 2>&1; then
        log "推送成功（使用已配置凭证）"
    else
        err "推送失败：未配置凭证或凭证无效"
        err "请通过以下方式之一推送："
        err "  1. bash scripts/auto_push.sh <github_token>"
        err "  2. 配置 git credential helper: git config --global credential.helper store"
        err "  3. 配置 GH_TOKEN 环境变量"
        exit 1
    fi
fi

# ---------- 3. 显示结果 ----------
log "当前状态："
git log --oneline -3
echo ""
git status -sb | head -3
