-- v0.4.0 在线更新系统 回滚
-- 1. 删除 system_update_log 表
-- 2. 删除 sys_config 8 项 update.* 配置

DROP TABLE IF EXISTS `system_update_log`;

DELETE FROM `sys_config` WHERE `config_key` IN (
    'update.webhook.secret',
    'update.webhook.branch',
    'update.webhook.auto_update',
    'update.deploy.script_path',
    'update.healthcheck.url',
    'update.healthcheck.timeout',
    'update.rollback.enabled',
    'update.lock.timeout'
);
