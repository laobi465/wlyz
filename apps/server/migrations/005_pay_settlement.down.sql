-- =====================================================================
-- Migration 005 DOWN: 回滚平台总支付结算记录表 + 配置项
-- =====================================================================

-- 删除新增的配置项
DELETE FROM `sys_config` WHERE `config_key` IN (
  'pay.platform.sign_type',
  'pay.platform.return_front_url',
  'pay.platform.order_name_prefix',
  'pay.platform.callback_retry_max',
  'pay.settlement.min_amount',
  'pay.settlement.auto_enabled'
);

-- 恢复 002 seed 中被修正的默认值（仅当当前值为修正后的值才回滚）
UPDATE `sys_config` SET `config_value` = '/api/v1/pay/platform/notify'
  WHERE `config_key` = 'pay.platform.notify_path' AND `config_value` = '/api/v1/pay/notify/epay';

UPDATE `sys_config` SET `config_value` = '/pay/return'
  WHERE `config_key` = 'pay.platform.return_path' AND `config_value` = '/api/v1/pay/return/epay';

-- 删除结算记录表
DROP TABLE IF EXISTS `platform_settlement`;
