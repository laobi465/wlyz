-- 回滚 v0.4.x SMTP 加密方式配置项
DELETE FROM `sys_config` WHERE `config_key` IN ('notify.email.smtp_encryption', 'notify.email.smtp_timeout_seconds');
