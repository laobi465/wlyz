-- v0.4.0 数据备份恢复系统 回滚
DROP TABLE IF EXISTS `system_backup_log`;

DELETE FROM `sys_config` WHERE `config_key` IN (
    'backup.dir',
    'backup.retention_days',
    'backup.auto_enabled',
    'backup.encryption_key',
    'backup.compress',
    'backup.tables_filter'
);
