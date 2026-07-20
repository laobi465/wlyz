-- v0.4.0 监控告警系统 回滚
DROP TABLE IF EXISTS `system_alert`;
DROP TABLE IF EXISTS `system_metric`;

DELETE FROM `sys_config` WHERE `config_key` IN (
    'monitor.collect_interval',
    'monitor.alert_enabled',
    'monitor.notify.webhook_url',
    'monitor.silence_minutes',
    'monitor.threshold.cpu_usage',
    'monitor.threshold.memory_usage',
    'monitor.threshold.disk_usage',
    'monitor.threshold.error_rate',
    'monitor.retention_days'
);
