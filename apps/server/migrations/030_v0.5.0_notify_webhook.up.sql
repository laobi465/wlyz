-- v0.5.0 集成扩展批次 1：通知扩展 3 项
-- 1. 钉钉群机器人：notify.dingtalk.* 共 5 项配置
-- 2. 企业微信群机器人：notify.wecom.* 共 2 项配置
-- 3. Telegram Bot：notify.telegram.* 共 3 项配置
--
-- 严格遵循铁律 04/05：
--   04 - webhook URL / Bot Token / 加签 secret 全部走 sys_config，无硬编码
--   05 - 10 项 notify.{dingtalk,wecom,telegram}.* 配置可通过后台「系统配置」实时调整
--
-- 安全：webhook URL 与 Bot Token 视同敏感凭证（一旦泄露可被恶意发消息）
--   生产环境建议通过管理后台写入（明文存储，权限仅超管可见）；
--   未来如需 AES 加密存储可复用 sms.access_secret_enc 的 crypto.DecryptAES 模式

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
-- 钉钉群机器人
('notify.dingtalk.enabled',     '0', '', 'bool',   '钉钉机器人开关',       'notify', 'v0.5.0 1=启用钉钉群机器人通道；0=关闭'),
('notify.dingtalk.webhook_url', '',  '', 'string', '钉钉机器人 Webhook URL','notify', 'v0.5.0 形如 https://oapi.dingtalk.com/robot/send?access_token=xxx'),
('notify.dingtalk.secret',      '',  '', 'string', '钉钉加签 secret',     'notify', 'v0.5.0 安全设置「加签」模式下的 secret（可选，留空表示不加签）'),
('notify.dingtalk.at_mobiles',  '',  '', 'string', '钉钉 @ 手机号列表',   'notify', 'v0.5.0 逗号分隔，如 13800138000,13900139000；@ 仅在 markdown 中追加，实际 @ 行为由 at.isAtAll 决定'),
('notify.dingtalk.at_all',      '0', '', 'bool',   '钉钉 @ 所有人',       'notify', 'v0.5.0 1=@ 所有人；0=不 @ 所有人'),
-- 企业微信群机器人
('notify.wecom.enabled',        '0', '', 'bool',   '企业微信机器人开关',  'notify', 'v0.5.0 1=启用企业微信群机器人通道；0=关闭'),
('notify.wecom.webhook_url',    '',  '', 'string', '企业微信 Webhook URL','notify', 'v0.5.0 形如 https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx'),
-- Telegram Bot
('notify.telegram.enabled',     '0', '', 'bool',   'Telegram Bot 开关',   'notify', 'v0.5.0 1=启用 Telegram Bot 通道；0=关闭'),
('notify.telegram.bot_token',   '',  '', 'string', 'Telegram Bot Token',  'notify', 'v0.5.0 BotFather 创建机器人后获取的 token，如 123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11'),
('notify.telegram.chat_id',     '',  '', 'string', 'Telegram Chat ID',    'notify', 'v0.5.0 频道=@xxx / 群组=-1001234567890 / 私聊=用户 ID') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
