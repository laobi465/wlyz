-- v0.5.0 集成扩展批次 1：通知扩展 3 项（回滚）
-- 删除 10 项 notify.{dingtalk,wecom,telegram}.* 配置
-- 注：不回滚 notify_log 表（channel 字段为字符串，旧数据保留无副作用）
DELETE FROM `sys_config` WHERE `config_key` IN (
  'notify.dingtalk.enabled',
  'notify.dingtalk.webhook_url',
  'notify.dingtalk.secret',
  'notify.dingtalk.at_mobiles',
  'notify.dingtalk.at_all',
  'notify.wecom.enabled',
  'notify.wecom.webhook_url',
  'notify.telegram.enabled',
  'notify.telegram.bot_token',
  'notify.telegram.chat_id'
);
