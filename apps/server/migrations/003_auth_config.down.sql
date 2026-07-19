-- 回滚 003_auth_config.up.sql
DELETE FROM `sys_config` WHERE `config_key` IN (
  'security.login.max_attempts',
  'security.login.lock_seconds',
  'security.login.window_seconds',
  'security.login.require_captcha',
  'jwt.access_ttl_seconds',
  'jwt.refresh_ttl_seconds',
  'jwt.issuer',
  'totp.issuer',
  'totp.period',
  'totp.digits',
  'totp.algorithm',
  'totp.skew',
  'totp.backup_codes_count',
  'admin.2fa_required',
  'tenant.2fa_required',
  'agent.2fa_required',
  'tenant.register.enabled',
  'tenant.register.default_package_id',
  'tenant.register.trial_days'
);
