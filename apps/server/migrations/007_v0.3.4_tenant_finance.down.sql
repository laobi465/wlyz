-- ============================================================
-- KeyAuth SaaS v0.3.4 回滚迁移
-- ============================================================

DROP TABLE IF EXISTS `tenant_withdraw`;
DROP TABLE IF EXISTS `tenant_balance_log`;

ALTER TABLE `sys_tenant` DROP COLUMN `frozen_balance`;
ALTER TABLE `sys_tenant` DROP COLUMN `balance`;
