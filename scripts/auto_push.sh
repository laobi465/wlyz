#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— 自动推送脚本
# ============================================================
# 用法：
#   bash scripts/auto_push.sh                    # 用已配置凭证推送
#   bash scripts/auto_push.sh ghp_xxxx           # 用指定 token 推送 + 更新仓库描述
#   GH_TOKEN=ghp_xxx bash scripts/auto_push.sh   # 通过环境变量传入 token
#
# 行为：
#   1. git add -A（暂存所有变更）
#   2. 如有变更则自动 commit
#   3. git push 到 origin main
#   4. git fetch 刷新本地 origin/main 缓存（避免 git status 误报 ahead）
#   5. 若提供 token，调用 GitHub API 更新仓库「About」描述（自动从 CHANGELOG 提取当前版本）
# ============================================================
set -euo pipefail

GREEN='\033[0;32m'; YELLOW='\033[0;33m'; RED='\033[0;31m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] [WARN]${NC} $*"; }
err()  { echo -e "${RED}[$(date '+%H:%M:%S')] [ERROR]${NC} $*" >&2; }

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

# ---------- 0. 解析 token（命令行参数 > 环境变量） ----------
TOKEN="${1:-${GH_TOKEN:-}}"
REMOTE_URL="https://github.com/laobi465/wlyz.git"
REPO_FULL="laobi465/wlyz"  # owner/repo 用于 GitHub API
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
    PUSH_URL="https://x-access-token:${TOKEN}@github.com/${REPO_FULL}.git"
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

# ---------- 3. fetch 刷新本地 origin/main 缓存 ----------
log "刷新 origin/main 本地缓存..."
if [[ -n "$TOKEN" ]]; then
    FETCH_URL="https://x-access-token:${TOKEN}@github.com/${REPO_FULL}.git"
    git fetch "$FETCH_URL" "$BRANCH:refs/remotes/origin/$BRANCH" >/dev/null 2>&1 && \
        log "fetch 完成" || warn "fetch 失败（不影响推送结果）"
else
    git fetch origin "$BRANCH" >/dev/null 2>&1 && log "fetch 完成" || warn "fetch 失败（凭证无效）"
fi

# ---------- 4. 更新 GitHub 仓库「About」描述（可选） ----------
# 自动从 CHANGELOG.md 提取最新版本号与简介
update_repo_about() {
    [[ -z "$TOKEN" ]] && return 0

    local changelog="docs/CHANGELOG.md"
    if [[ ! -f "$changelog" ]]; then
        warn "未找到 $changelog，跳过仓库描述更新"
        return 0
    fi

    # 提取最新版本号（第一个 ## [x.y.z] 行）
    local version
    version=$(grep -oE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' "$changelog" | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
    if [[ -z "$version" ]]; then
        warn "未能从 CHANGELOG 解析版本号"
        return 0
    fi

    local about="面向开发者的多租户卡密验证 SaaS 平台 | Go + Vue3 + MySQL + Redis | 当前版本 v${version}"
    log "更新 GitHub 仓库描述为：$about"

    local resp
    resp=$(curl -sS -X PATCH \
        -H "Authorization: token ${TOKEN}" \
        -H "Accept: application/vnd.github+json" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        "https://api.github.com/repos/${REPO_FULL}" \
        -d "$(printf '{"description":%s,"homepage":""}' "$(printf '%s' "$about" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')")" \
        2>&1) || true

    if echo "$resp" | grep -q '"full_name"'; then
        log "GitHub 仓库描述已更新"
    else
        warn "GitHub 仓库描述更新失败：$(echo "$resp" | head -c 200)"
    fi

    # 同时设置 topics（标签）
    curl -sS -X PUT \
        -H "Authorization: token ${TOKEN}" \
        -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${REPO_FULL}/topics" \
        -d '{"names":["go","vue3","saas","multi-tenant","card-key","license","authentication","jwt","totp","redis"]}' \
        >/dev/null 2>&1 && log "GitHub topics 已更新" || warn "topics 更新失败"
}

update_repo_about

# ---------- 5. 显示结果 ----------
log "当前状态："
git log --oneline -3
echo ""
git status -sb | head -3
