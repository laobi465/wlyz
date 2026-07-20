-- ============================================================
-- KeyAuth SaaS v0.4.0 2FA 备用码 DB 持久化回滚
-- 回滚前请确认：所有用户的 2FA 备用码已重新写入 Redis（2fa:backup:{role}:{user_id}）
-- 否则回滚后将丢失备用码，用户需 Disable2FA + Setup2FA 重新生成
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

ALTER TABLE `sys_admin` DROP COLUMN `backup_codes`;
ALTER TABLE `sys_tenant` DROP COLUMN `backup_codes`;
ALTER TABLE `agent` DROP COLUMN `backup_codes`;

SET FOREIGN_KEY_CHECKS = 1;
