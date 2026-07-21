#!/usr/bin/env bash
# ============================================================
# KeyAuth SaaS —— 015_v0.4.0_end_user_system.up.sql 静态验证脚本
# ============================================================
# 目的：在不连接真实 MySQL 的情况下，通过文本分析验证迁移 SQL 的幂等性和兼容性
#
# 检查项：
#   1. 不包含 MySQL 8.0 不兼容的语法（ADD COLUMN IF NOT EXISTS / ADD INDEX IF NOT EXISTS）
#   2. CREATE TABLE 都使用 IF NOT EXISTS
#   3. ALTER TABLE 都通过 INFORMATION_SCHEMA + PREPARE/EXECUTE 保护
#   4. INSERT 使用 ON DUPLICATE KEY UPDATE
#   5. 不包含 DROP TABLE / DROP COLUMN / DROP INDEX（除 IF EXISTS 兜底）
# ============================================================
set -euo pipefail

MIGRATION_FILE="$(cd "$(dirname "$0")/.." && pwd)/apps/server/migrations/015_v0.4.0_end_user_system.up.sql"

if [[ ! -f "$MIGRATION_FILE" ]]; then
    echo "✗ 迁移文件不存在：$MIGRATION_FILE"
    exit 1
fi

echo "==== 静态验证 $MIGRATION_FILE ===="
PASS=0
FAIL=0

check_pass() { echo "✓ $1"; PASS=$((PASS + 1)); }
check_fail() { echo "✗ $1"; FAIL=$((FAIL + 1)); }

# 去除注释行（-- 开头）后再检查，避免误报
STRIPPED_FILE=$(mktemp)
grep -v '^\s*--' "$MIGRATION_FILE" > "$STRIPPED_FILE" || true
trap 'rm -f "$STRIPPED_FILE"' EXIT

# 检查 1：不应包含 ADD COLUMN IF NOT EXISTS（去除注释后）
if grep -q "ADD COLUMN IF NOT EXISTS" "$STRIPPED_FILE"; then
    check_fail "包含 MySQL 8.0 不兼容的 'ADD COLUMN IF NOT EXISTS'"
else
    check_pass "不包含 ADD COLUMN IF NOT EXISTS（MySQL 8.0 兼容）"
fi

# 检查 2：不应包含 ADD INDEX IF NOT EXISTS（去除注释后）
if grep -q "ADD INDEX IF NOT EXISTS" "$STRIPPED_FILE"; then
    check_fail "包含 MySQL 8.0 不兼容的 'ADD INDEX IF NOT EXISTS'"
else
    check_pass "不包含 ADD INDEX IF NOT EXISTS（MySQL 8.0 兼容）"
fi

# 检查 3：CREATE TABLE 应使用 IF NOT EXISTS
CREATE_TABLE_COUNT=$(grep -ciE "CREATE TABLE" "$STRIPPED_FILE" || echo 0)
CREATE_TABLE_IF_NOT_EXISTS_COUNT=$(grep -ciE "CREATE TABLE IF NOT EXISTS" "$STRIPPED_FILE" || echo 0)
if [[ "$CREATE_TABLE_COUNT" -eq "$CREATE_TABLE_IF_NOT_EXISTS_COUNT" && "$CREATE_TABLE_COUNT" -gt 0 ]]; then
    check_pass "所有 CREATE TABLE 都使用 IF NOT EXISTS（$CREATE_TABLE_COUNT 个）"
else
    check_fail "存在未使用 IF NOT EXISTS 的 CREATE TABLE（$CREATE_TABLE_COUNT 总数，$CREATE_TABLE_IF_NOT_EXISTS_COUNT 安全）"
fi

# 检查 4：ALTER TABLE 应通过 INFORMATION_SCHEMA + PREPARE/EXECUTE 保护
if grep -q "INFORMATION_SCHEMA.COLUMNS" "$STRIPPED_FILE"; then
    check_pass "通过 INFORMATION_SCHEMA.COLUMNS 检查字段存在性"
else
    check_fail "缺少 INFORMATION_SCHEMA.COLUMNS 字段检查"
fi

if grep -q "INFORMATION_SCHEMA.STATISTICS" "$STRIPPED_FILE"; then
    check_pass "通过 INFORMATION_SCHEMA.STATISTICS 检查索引存在性"
else
    check_fail "缺少 INFORMATION_SCHEMA.STATISTICS 索引检查"
fi

if grep -q "PREPARE" "$STRIPPED_FILE" && grep -q "EXECUTE" "$STRIPPED_FILE"; then
    check_pass "使用 PREPARE/EXECUTE 执行动态 SQL（兼容 MySQL 8.0 / MariaDB）"
else
    check_fail "缺少 PREPARE/EXECUTE 动态 SQL"
fi

# 检查 5：INSERT 应使用 ON DUPLICATE KEY UPDATE
if grep -q "ON DUPLICATE KEY UPDATE" "$STRIPPED_FILE"; then
    check_pass "INSERT 使用 ON DUPLICATE KEY UPDATE（幂等）"
else
    check_fail "INSERT 缺少 ON DUPLICATE KEY UPDATE（不幂等）"
fi

# 检查 6：不包含破坏性 DDL
if grep -qiE "^\s*DROP (TABLE|COLUMN|INDEX)" "$STRIPPED_FILE"; then
    check_fail "包含破坏性 DROP 语句"
else
    check_pass "不包含破坏性 DROP 语句"
fi

# 检查 7：使用 DATABASE() 而非硬编码 schema 名
if grep -q "DATABASE()" "$STRIPPED_FILE"; then
    check_pass "使用 DATABASE() 取当前库，避免硬编码 schema 名"
else
    check_fail "未使用 DATABASE()，可能硬编码 schema 名"
fi

# 检查 8：DEALLOCATE PREPARE 配对
PREPARE_COUNT=$(grep -ciE "^PREPARE " "$STRIPPED_FILE" || echo 0)
DEALLOCATE_COUNT=$(grep -ciE "^DEALLOCATE PREPARE" "$STRIPPED_FILE" || echo 0)
if [[ "$PREPARE_COUNT" -eq "$DEALLOCATE_COUNT" ]]; then
    check_pass "PREPARE / DEALLOCATE PREPARE 配对（$PREPARE_COUNT 对）"
else
    check_fail "PREPARE ($PREPARE_COUNT) 与 DEALLOCATE PREPARE ($DEALLOCATE_COUNT) 不配对"
fi

echo ""
echo "==== 验证结果 ===="
echo "通过：$PASS"
echo "失败：$FAIL"
if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
