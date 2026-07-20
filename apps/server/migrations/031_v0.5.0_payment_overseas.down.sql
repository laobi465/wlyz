-- v0.5.0 集成扩展批次 3：海外支付通道回滚
-- 删除 13 项 pay.{usdt,paypal,stripe}.* sys_config 配置

DELETE FROM `sys_config` WHERE `config_key` IN (
  'pay.usdt.enabled',
  'pay.usdt.trc20_address',
  'pay.usdt.contract_address',
  'pay.usdt.hmac_secret_enc',
  'pay.usdt.exchange_rate',
  'pay.usdt.polling_enabled',
  'pay.usdt.polling_interval_seconds',
  'pay.usdt.trongrid_api_key',
  'pay.usdt.expire_seconds',
  'pay.paypal.enabled',
  'pay.paypal.client_id',
  'pay.paypal.client_secret_enc',
  'pay.paypal.webhook_id',
  'pay.paypal.sandbox',
  'pay.paypal.expire_seconds',
  'pay.stripe.enabled',
  'pay.stripe.secret_key_enc',
  'pay.stripe.webhook_secret_enc',
  'pay.stripe.expire_seconds'
);
