-- v0.4.x 阿里云短信增强配置项（第二十一项迁移）
-- 在 014 通知系统基础上新增 2 项 notify.sms.* 配置：
--   1. notify.sms.region  - 阿里云短信区域（默认 cn-hangzhou）
--   2. notify.sms.endpoint - 阿里云短信 API endpoint（默认 dysmsapi.aliyuncs.com）
--
-- 严格遵循铁律 04/05：
--   04 - 无硬编码：区域 / endpoint 全部从 sys_config 读取，便于切换国际节点
--   05 - 配置走后端：新增 2 项配置可通过后台实时调整，无需重启

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('notify.sms.region',   'cn-hangzhou',            'string', '阿里云短信区域',         'notify', 'v0.4.x 阿里云 Dysms API 区域，如 cn-hangzhou / cn-shanghai / ap-southeast-1'),
('notify.sms.endpoint', 'dysmsapi.aliyuncs.com',  'string', '阿里云短信 API endpoint', 'notify', 'v0.4.x 阿里云 Dysms API 域名，国际节点可改为 dysmsapi.ap-southeast-1.aliyuncs.com')
ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
