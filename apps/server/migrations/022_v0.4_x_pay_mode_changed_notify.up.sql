-- v0.4.x 支付方式变更通知代理（收尾项 D）
-- 当开发者通过 TenantSavePayConfig 切换支付通道 enabled 状态时，
-- 通知该开发者名下所有启用代理（站内信 + 公告）。
--
-- 严格遵循铁律 04/05：
--   04 - 无硬编码：通知开关走 sys_config，可后台实时调整
--   05 - 配置走后端：notify.pay_mode_changed.enabled=0 时跳过通知
--
-- 配套代码：apps/server/internal/notify/notify.go NotifyAgentsByTenant
--           apps/server/internal/handler/tenant_business.go TenantSavePayConfig

-- ============== 1. 新增 pay_mode_changed 站内信模板（平台通用，tenant_id=0） ==============
-- 列名参照 014 迁移：code/name/channel/subject/content/variables/tenant_id/status/remark
-- status='enabled'（与 014 已有 4 个模板保持一致）
INSERT IGNORE INTO `notify_template` (`code`, `name`, `channel`, `subject`, `content`, `variables`, `tenant_id`, `status`, `remark`) VALUES
('pay_mode_changed', '支付方式变更通知', 'inapp', '',
 '开发者 {{tenant_name}} 的支付通道 {{channel}} 已{{action}}（{{time}}），请知悉。如有疑问请联系开发者。',
 '["tenant_name","channel","action","time"]', 0, 'enabled',
 'v0.4.x 收尾项 D：开发者切换支付配置时通知所有代理（站内信模板）');

-- ============== 2. 新增配置开关 ==============
INSERT IGNORE INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('notify.pay_mode_changed.enabled', '1', 'bool', '支付方式变更通知开关', 'notify',
 'v0.4.x 收尾项 D：开发者切换支付配置时通知所有代理；1=启用，0=关闭');
