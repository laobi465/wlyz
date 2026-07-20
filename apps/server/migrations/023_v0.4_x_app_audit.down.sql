-- v0.4.x S-04 应用审核 - 回滚
-- 1. 删除 sys_config 1 项 app.audit.* 配置
-- 2. 删除 app 表 4 个审核字段 + 索引

DELETE FROM `sys_config` WHERE `config_key` IN (
  'app.audit.enabled'
);

DROP INDEX `idx_app_audit_status` ON `app`;
ALTER TABLE `app` DROP COLUMN `audit_status`;
ALTER TABLE `app` DROP COLUMN `audit_remark`;
ALTER TABLE `app` DROP COLUMN `audited_at`;
ALTER TABLE `app` DROP COLUMN `audited_by`;
