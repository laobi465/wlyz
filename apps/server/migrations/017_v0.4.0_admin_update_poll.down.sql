-- v0.4.0 管理员弹窗通知 - 回滚
-- 删除 2 项 update.poll.* sys_config
DELETE FROM `sys_config` WHERE `config_key` IN ('update.poll.enabled', 'update.poll.interval_seconds');
