-- v0.4.x Prometheus 监控集成
-- 严格遵循铁律 05：所有可变参数走 sys_config 表，后台可视化编辑
-- 4 项 monitor.prometheus.* 配置：
--   enabled         - 暴露开关（默认 1=开启）
--   path            - 端点路径（默认 /metrics）
--   basic_auth_user - BasicAuth 用户名（空=不鉴权，仅内网部署场景）
--   basic_auth_pass - BasicAuth 密码（建议加密存储或使用强随机串）

INSERT INTO sys_config (config_key, config_value, config_type, config_name, config_group, remark) VALUES
('monitor.prometheus.enabled', '1', 'bool', 'Prometheus 指标暴露开关', 'monitor', 'v0.4.x 是否暴露 /metrics 端点给 Prometheus 抓取；关闭后访问返回 503'),
('monitor.prometheus.path', '/metrics', 'string', 'Prometheus 端点路径', 'monitor', 'v0.4.x Prometheus 抓取端点路径，默认 /metrics；修改后需重启服务生效'),
('monitor.prometheus.basic_auth_user', '', 'string', 'Prometheus BasicAuth 用户名', 'monitor', 'v0.4.x 端点访问鉴权用户名；留空表示不启用 BasicAuth（仅内网部署场景）'),
('monitor.prometheus.basic_auth_pass', '', 'string', 'Prometheus BasicAuth 密码', 'monitor', 'v0.4.x 端点访问鉴权密码；建议使用强随机串；留空表示不启用 BasicAuth');
