#!/usr/bin/env bash
# v0.4.0 在线更新部署脚本
# 由 internal/update.Manager.runDeployScript 调用
# 严格遵循铁律 04/05：脚本路径通过 sys_config(update.deploy.script_path) 可配置
#
# 默认流程：
#   1. 安装 Go 依赖（go mod download）
#   2. 编译 server 二进制
#   3. 执行数据库迁移（启动时由 InitContainer 自动执行）
#   4. 重启服务（systemd / docker / pm2 自适应）
#
# 部署环境适配：
#   - 生产环境建议通过 systemd / docker-compose 管理进程，本脚本只负责编译
#   - 重启命令根据 DEPLOY_MODE 环境变量选择：systemd / docker / pm2 / none
#
# 退出码：0=成功，非 0=失败（update.Manager 会自动触发回滚）

set -euo pipefail

# 切换到项目根目录（脚本位于 scripts/ 下，向上返回一层）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] deploy_update.sh start, root=$PROJECT_ROOT"

# ============== 1. Go 依赖下载与编译 ==============
echo "[$(date '+%H:%M:%S')] step 1: go mod download"
cd apps/server
go mod download

echo "[$(date '+%H:%M:%S')] step 2: go build server"
go build -o ../../bin/keyauth-server ./cmd/main.go
cd "$PROJECT_ROOT"

# ============== 2. 数据库迁移 ==============
# 注：迁移由 server 启动时 InitContainer 自动执行（migration.Run），此处不重复
echo "[$(date '+%H:%M:%S')] step 3: migration deferred to server startup"

# ============== 3. 重启服务 ==============
DEPLOY_MODE="${DEPLOY_MODE:-none}"

case "$DEPLOY_MODE" in
    systemd)
        echo "[$(date '+%H:%M:%S')] step 4: systemctl restart keyauth-server"
        sudo systemctl restart keyauth-server || {
            echo "ERROR: systemctl restart failed"
            exit 1
        }
        ;;
    docker)
        echo "[$(date '+%H:%M:%S')] step 4: docker-compose restart"
        docker-compose restart keyauth-server || {
            echo "ERROR: docker-compose restart failed"
            exit 1
        }
        ;;
    pm2)
        echo "[$(date '+%H:%M:%S')] step 4: pm2 restart keyauth-server"
        pm2 restart keyauth-server || {
            echo "ERROR: pm2 restart failed"
            exit 1
        }
        ;;
    none)
        echo "[$(date '+%H:%M:%S')] step 4: DEPLOY_MODE=none, skip restart (assume external supervisor)"
        ;;
    *)
        echo "ERROR: unknown DEPLOY_MODE=$DEPLOY_MODE (supported: systemd / docker / pm2 / none)"
        exit 1
        ;;
esac

echo "[$(date '+%H:%M:%S')] deploy_update.sh done"
exit 0
