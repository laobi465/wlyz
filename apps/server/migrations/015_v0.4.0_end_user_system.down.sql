-- Rollback v0.4.0 终端用户体系
DROP TABLE IF EXISTS `end_user_token`;
DROP TABLE IF EXISTS `end_user_card`;
DROP TABLE IF EXISTS `end_user`;

-- 回滚 app_card.end_user_id 字段（向前兼容字段，回滚时移除）
ALTER TABLE `app_card` DROP INDEX IF EXISTS `idx_end_user_id`;
ALTER TABLE `app_card` DROP COLUMN IF EXISTS `end_user_id`;

-- 删除 enduser.* 配置项
DELETE FROM `sys_config` WHERE `config_group` = 'enduser';
