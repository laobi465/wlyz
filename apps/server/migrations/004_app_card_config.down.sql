-- 回滚 004_app_card_config.up.sql
DELETE FROM `sys_config` WHERE `config_key` IN (
  'app.default.max_devices',
  'app.default.heartbeat_interval',
  'app.default.heartbeat_timeout',
  'app.default.offline_grace',
  'app.default.unbind_deduct_seconds',
  'card.generate.max_batch',
  'card.generate.charset',
  'card.generate.segment_length',
  'card.generate.segment_count',
  'verify.log.async_enabled',
  'verify.log.sample_rate'
);
