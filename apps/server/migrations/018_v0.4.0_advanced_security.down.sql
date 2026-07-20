-- 回滚 v0.4.0 第十五项迁移：高级安全
-- 顺序：先删 sys_config → 再删表 → 最后回滚 app_device 字段

-- 1. 删除 15 项 sys_config
DELETE FROM `sys_config` WHERE `config_key` IN (
  'cloudflare.enabled',
  'cloudflare.real_ip_header',
  'cloudflare.ip_country_header',
  'cloudflare.trusted_cidrs',
  'risk.engine.enabled',
  'risk.engine.score_threshold',
  'risk.engine.default_action',
  'risk.geo_login_alert.enabled',
  'risk.geo_login_alert.ipv4_prefix',
  'risk.geo_login_alert.ipv6_prefix',
  'risk.geo_login_alert.notify_channels',
  'risk.new_device_alert.enabled',
  'risk.abnormal_ua_alert.enabled',
  'risk.abnormal_time_alert.enabled',
  'risk.abnormal_time_start',
  'risk.abnormal_time_end'
);

-- 2. 删除 5 条 seed 风控规则（仅删 system 创建的内置规则，保留管理员自定义规则便于人工恢复）
DELETE FROM `risk_rule` WHERE `created_by` = 'system';

-- 3. 删除三张新表
DROP TABLE IF EXISTS `login_geo_alert`;
DROP TABLE IF EXISTS `risk_event`;
DROP TABLE IF EXISTS `risk_rule`;

-- 4. 回滚 app_device 扩展字段
ALTER TABLE `app_device`
  DROP COLUMN `hwid_components`,
  DROP COLUMN `user_agent`,
  DROP COLUMN `client_ip_ext`,
  DROP COLUMN `screen_resolution`,
  DROP COLUMN `timezone`,
  DROP COLUMN `language`;
