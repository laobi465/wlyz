-- v0.4.x 阿里云短信增强配置项 - 回滚
-- 删除 2 项 notify.sms.* 配置（region / endpoint）

DELETE FROM `sys_config` WHERE `config_key` IN (
  'notify.sms.region',
  'notify.sms.endpoint'
);
