-- 回滚 v0.4.x Prometheus 监控集成
DELETE FROM sys_config WHERE config_key IN (
    'monitor.prometheus.enabled',
    'monitor.prometheus.path',
    'monitor.prometheus.basic_auth_user',
    'monitor.prometheus.basic_auth_pass'
);
