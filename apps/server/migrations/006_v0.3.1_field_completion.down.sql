-- ============================================================
-- KeyAuth SaaS v0.3.1 字段补全回滚
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- 删除新增表
DROP TABLE IF EXISTS `refresh_token_device`;
DROP TABLE IF EXISTS `log_login_failed`;

-- 回滚 log_operation 字段
ALTER TABLE `log_operation` DROP COLUMN `status`;
ALTER TABLE `log_operation` DROP COLUMN `user_agent`;
ALTER TABLE `log_operation` DROP COLUMN `username`;

-- 回滚 sec_ip_blacklist 字段
ALTER TABLE `sec_ip_blacklist` DROP COLUMN `created_by_type`;
ALTER TABLE `sec_ip_blacklist` DROP COLUMN `created_by`;

-- 回滚 notice 字段
ALTER TABLE `notice` DROP COLUMN `sort`;

-- 回滚 agent_invite_code 字段
ALTER TABLE `agent_invite_code` DROP COLUMN `used_by_agent_id`;

-- 回滚 agent 字段
ALTER TABLE `agent` DROP COLUMN `last_login_ip`;
ALTER TABLE `agent` DROP COLUMN `totp_secret`;
ALTER TABLE `agent` DROP COLUMN `email`;
ALTER TABLE `agent` DROP COLUMN `inviter_id`;
ALTER TABLE `agent` DROP COLUMN `commission_mode`;

-- 回滚 app_version 字段
ALTER TABLE `app_version` DROP COLUMN `channel`;

-- 回滚 app_cloud_var 字段
ALTER TABLE `app_cloud_var` DROP COLUMN `read_only`;

-- 回滚 sys_package 字段
ALTER TABLE `sys_package` DROP COLUMN `description`;

-- 回滚 sys_tenant 字段
ALTER TABLE `sys_tenant` DROP COLUMN `remark`;

SET FOREIGN_KEY_CHECKS = 1;
