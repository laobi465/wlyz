-- =====================================================================
-- Migration 003: 认证模块新增配置项
-- 关联功能：超管/开发者/代理登录、JWT 双 Token、TOTP 2FA、登录失败锁定
-- 铁律 05：所有可变参数必须后台化（sys_config 表）
-- =====================================================================

-- 先删除可能存在的旧记录（幂等迁移）
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

-- ========== 登录失败锁定 ==========
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('security.login.max_attempts',   '5',     'number', '登录失败锁定阈值',     'security', '同一账号连续登录失败达到该次数后锁定，0 表示不锁定'),
('security.login.lock_seconds',   '900',   'number', '账号锁定时长(秒)',     'security', '默认 15 分钟，900 秒'),
('security.login.window_seconds', '600',   'number', '失败计数窗口(秒)',     'security', '默认 10 分钟，超过窗口后计数自动清零'),
('security.login.require_captcha','3',     'number', '触发验证码的失败次数', 'security', '达到该失败次数后前端展示图形验证码，0 表示从不');

-- ========== JWT Token 配置 ==========
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('jwt.access_ttl_seconds',  '7200',   'number', 'Access Token 有效期(秒)', 'jwt', '默认 2 小时，7200 秒'),
('jwt.refresh_ttl_seconds', '604800', 'number', 'Refresh Token 有效期(秒)', 'jwt', '默认 7 天，604800 秒'),
('jwt.issuer',              'keyauth-saas', 'string', 'JWT 签发者',  'jwt', 'Token 中的 iss 字段值');

-- ========== TOTP 2FA 配置 ==========
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('totp.issuer',             'KeyAuth SaaS', 'string', 'TOTP 发行方',     'totp', '显示在 Google Authenticator 中的应用名'),
('totp.period',             '30',           'number', 'TOTP 周期(秒)',   'totp', '默认 30 秒，与 Google Authenticator 兼容'),
('totp.digits',             '6',            'number', 'TOTP 位数',       'totp', '6 或 8，默认 6 位'),
('totp.algorithm',          'SHA1',         'string', 'TOTP 算法',       'totp', 'SHA1/SHA256/SHA512，SHA1 兼容性最好'),
('totp.skew',               '1',            'number', 'TOTP 容差周期',   'totp', '允许前后偏移的周期数，默认 1（±30s）'),
('totp.backup_codes_count', '10',           'number', '备用码数量',      'totp', '丢失 TOTP 设备时使用的备用码数量，一次性');

-- ========== 2FA 强制策略 ==========
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('admin.2fa_required',  '0', 'bool', '强制超管开启 2FA',  'security', '开启后超管账号必须绑定 TOTP 才能登录'),
('tenant.2fa_required', '0', 'bool', '强制开发者开启 2FA', 'security', '开启后开发者账号必须绑定 TOTP 才能登录'),
('agent.2fa_required',  '0', 'bool', '强制代理开启 2FA',  'security', '开启后代理账号必须绑定 TOTP 才能登录');

-- ========== 开发者注册配置 ==========
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('tenant.register.enabled',           '1', 'bool',   '开放开发者注册',   'tenant', '总开关，关闭后只能由超管在后台创建开发者'),
('tenant.register.default_package_id','1', 'number', '默认注册套餐 ID',  'tenant', '新开发者注册后默认绑定的套餐 ID（指向 sys_package.id）'),
('tenant.register.trial_days',        '7', 'number', '试用天数',         'tenant', '注册后免费试用天数，0 表示无试用');
