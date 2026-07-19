-- =====================================================================
-- Migration 004: 应用管理 + 卡密管理新增配置项
-- 关联功能：应用默认参数、卡密批量生成、客户端验证流程
-- 铁律 05：所有可变参数必须后台化（sys_config 表）
-- =====================================================================

-- 先删除可能存在的旧记录（幂等迁移）
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

-- ========== 应用默认参数（创建应用时如未指定则使用默认值） ==========
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('app.default.max_devices',           '1',     'number', '应用默认最大设备数',   'app', '新建应用未指定时使用的默认值，1 表示一机一卡'),
('app.default.heartbeat_interval',    '60',    'number', '默认心跳间隔(秒)',     'app', '客户端两次心跳的最小间隔，默认 60 秒'),
('app.default.heartbeat_timeout',     '180',   'number', '默认心跳超时(秒)',     'app', '超过该时长未心跳则视为离线，默认 180 秒'),
('app.default.offline_grace',         '86400', 'number', '默认离线宽限期(秒)',   'app', '允许离线时长，超过则验证失败，默认 24 小时'),
('app.default.unbind_deduct_seconds', '86400', 'number', '默认解绑扣时(秒)',     'app', '解绑设备时从有效期扣除的秒数，默认 24 小时');

-- ========== 卡密生成配置 ==========
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('card.generate.max_batch',      '10000', 'number', '单批次最大生成数',  'card', '单次 API 调用最多生成的卡密数量，防止 OOM'),
('card.generate.charset',        'ABCDEFGHJKMNPQRSTUVWXYZ23456789', 'string', '卡密字符集', 'card', '去除易混淆字符 0/O/1/I/L'),
('card.generate.segment_length', '4',     'number', '卡密每段长度',      'card', '卡密分段后每段的字符数，默认 4'),
('card.generate.segment_count',  '4',     'number', '卡密段数',          'card', '卡密总段数，默认 4 段（共 16 字符）');

-- ========== 验证日志配置 ==========
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('verify.log.async_enabled',  '0',     'bool',   '异步写日志开关',  'verify', '开启后验证日志走异步队列，关闭则同步写入（v0.2.x 默认同步）'),
('verify.log.sample_rate',    '1.0',   'number', '日志采样率',      'verify', '0-1 之间，1.0 表示全量记录，0.1 表示 10% 采样');
