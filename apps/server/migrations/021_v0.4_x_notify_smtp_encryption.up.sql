-- v0.4.x SMTP 加密方式配置项（增量迁移，不动 014 已上线的数据）
-- 严格遵循铁律 04/05：SMTP 加密方式与连接超时走 sys_config 后台可视化编辑
--
-- 配套代码：apps/server/internal/notify/notify.go dialSMTPClient
--   ssl  = 465 端口隐式 TLS（tls.DialWithDialer）
--   tls  = 587 端口 STARTTLS（client.StartTLS）
--   none = 25  端口明文（不推荐生产使用）

INSERT IGNORE INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('notify.email.smtp_encryption',       'ssl', 'string', 'SMTP 加密方式',     'notify', 'v0.4.x none/ssl/tls，ssl=465 隐式 TLS，tls=587 STARTTLS，none=25 明文；留空时按端口推断（465=ssl/587=tls/其他=none）'),
('notify.email.smtp_timeout_seconds',  '10',  'number', 'SMTP 连接超时秒数', 'notify', 'v0.4.x SMTP 建立 TCP/TLS 连接的超时秒数，默认 10');
