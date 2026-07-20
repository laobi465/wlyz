-- v0.4.x 开发者月费订单 - 回滚
-- 1. 删除 sys_config 3 项 pay.tenant_monthly_fee.* 配置
-- 2. 删除 tenant_monthly_fee_order 表

DELETE FROM `sys_config` WHERE `config_key` IN (
  'pay.tenant_monthly_fee.enabled',
  'pay.tenant_monthly_fee.amount',
  'pay.tenant_monthly_fee.free_days'
);

DROP TABLE IF EXISTS `tenant_monthly_fee_order`;
