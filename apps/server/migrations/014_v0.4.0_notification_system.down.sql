-- Rollback v0.4.0 通知系统
DROP TABLE IF EXISTS `notify_log`;
DROP TABLE IF EXISTS `notify_template`;

-- 删除 notify.* 配置项
DELETE FROM `sys_config` WHERE `config_group` = 'notify';
