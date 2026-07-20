-- ============================================================
-- KeyAuth SaaS v0.4.0 2FA 备用码 DB 持久化迁移
-- 说明：
--   1. sys_admin / sys_tenant / agent 三表增加 backup_codes 字段
--      存 AES-256-GCM 加密的逗号分隔字符串（最多 5 个备用码）
--   2. backup_codes 字段长度 512（5 个备用码 × 64 字符 + 4 分隔符，加密后冗余 ~2x，安全）
--   3. 兼容策略：profile.go Verify2FA 优先落库；Disable2FA 清空字段
--      老数据 Redis key (2fa:backup:{role}:{user_id}) 在升级后由 Disable2FA 自动清理
-- 严格遵循铁律 04/05：不引入新可变参数；铁律 06：向前兼容（字段默认空字符串）
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ============================================================
-- 1. sys_admin 增加 backup_codes
-- ============================================================
ALTER TABLE `sys_admin`
  ADD COLUMN `backup_codes` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '2FA 备用码（AES 加密的逗号分隔字符串）' AFTER `totp_secret`;

-- ============================================================
-- 2. sys_tenant 增加 backup_codes
-- ============================================================
ALTER TABLE `sys_tenant`
  ADD COLUMN `backup_codes` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '2FA 备用码（AES 加密的逗号分隔字符串）' AFTER `totp_secret`;

-- ============================================================
-- 3. agent 增加 backup_codes
-- ============================================================
ALTER TABLE `agent`
  ADD COLUMN `backup_codes` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '2FA 备用码（AES 加密的逗号分隔字符串）' AFTER `totp_secret`;

SET FOREIGN_KEY_CHECKS = 1;
