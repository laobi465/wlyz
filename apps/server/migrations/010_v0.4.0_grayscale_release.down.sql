-- ============================================================
-- KeyAuth SaaS v0.4.0 灰度发布迁移（回滚）
-- 回滚后：app_version 表恢复无灰度字段；sys_config 删除 3 项灰度配置
-- 注意：tenant_id 字段不回滚（修复 001 遗漏的 schema 不一致问题，回滚会破坏数据）
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

ALTER TABLE `app_version`
  DROP INDEX `idx_app_status_strategy`,
  DROP COLUMN `grayscale_channels`,
  DROP COLUMN `grayscale_regions`,
  DROP COLUMN `grayscale_platforms`,
  DROP COLUMN `grayscale_rate`,
  DROP COLUMN `release_strategy`;

DELETE FROM `sys_config` WHERE `config_key` IN (
  'app.version.grayscale.enabled',
  'app.version.grayscale.default_rate',
  'app.version.grayscale.hash_salt'
);

SET FOREIGN_KEY_CHECKS = 1;
