-- ============================================================
-- v0.4.0 第十五项迁移：高级安全（异地登录告警 + 风控规则引擎 + 设备指纹升级 + Cloudflare WAF 集成）
-- 严格遵循铁律 05：所有阈值与开关走 sys_config 后台可视化编辑
-- 严格遵循铁律 04：禁止硬编码配置键名（在 internal/risk 与 internal/middleware 中以常量声明）
-- ============================================================

-- 1. 风控规则配置表（自定义规则；内置规则硬编码在代码中，仅可调整阈值）
CREATE TABLE IF NOT EXISTS `risk_rule` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`         VARCHAR(64)  NOT NULL COMMENT '规则名称',
  `description`  VARCHAR(255) NOT NULL DEFAULT '' COMMENT '规则描述',
  `rule_type`    VARCHAR(32)  NOT NULL COMMENT 'geo_login/new_device/abnormal_ua/abnormal_time/high_frequency/custom',
  `condition`    TEXT         NOT NULL COMMENT 'JSON 条件（rule_type 对应的参数）',
  `score`        INT          NOT NULL DEFAULT 0 COMMENT '命中加分（0-100）',
  `action`       VARCHAR(32)  NOT NULL DEFAULT 'alert' COMMENT 'alert/challenge/block',
  `priority`     INT          NOT NULL DEFAULT 100 COMMENT '优先级（小先执行）',
  `status`       VARCHAR(32)  NOT NULL DEFAULT 'active' COMMENT 'active/disabled',
  `created_by`   VARCHAR(64)  NOT NULL DEFAULT 'system' COMMENT '创建者（system/admin 用户名）',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `idx_rule_type_status` (`rule_type`, `status`),
  INDEX `idx_priority` (`priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 风控规则配置表';

-- 2. 风控事件表（评分超阈值或命中 block/challenge 规则时落盘）
CREATE TABLE IF NOT EXISTS `risk_event` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `rule_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '命中的规则 ID（0=内置规则）',
  `rule_type`     VARCHAR(32)  NOT NULL COMMENT 'geo_login/new_device/abnormal_ua/abnormal_time/high_frequency/custom',
  `rule_name`     VARCHAR(64)  NOT NULL COMMENT '规则名称快照（便于历史审计）',
  `user_type`     VARCHAR(32)  NOT NULL DEFAULT '' COMMENT 'admin/tenant/agent/enduser（空=匿名）',
  `user_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `username`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '账号快照',
  `client_ip`     VARCHAR(45)  NOT NULL DEFAULT '',
  `user_agent`    VARCHAR(512) NOT NULL DEFAULT '',
  `risk_score`    INT          NOT NULL DEFAULT 0 COMMENT '本次评分（0-100）',
  `action_taken`  VARCHAR(32)  NOT NULL DEFAULT 'alert' COMMENT 'alert/challenge/block',
  `detail`        TEXT         NOT NULL COMMENT 'JSON 详情（触发原因 + 上下文）',
  `acknowledged`  TINYINT      NOT NULL DEFAULT 0 COMMENT '0=未确认 1=已确认',
  `acknowledged_by` VARCHAR(64) NOT NULL DEFAULT '',
  `acknowledged_at` DATETIME   NULL,
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `idx_rule_type_created` (`rule_type`, `created_at`),
  INDEX `idx_user_type_id` (`user_type`, `user_id`),
  INDEX `idx_client_ip` (`client_ip`),
  INDEX `idx_action_taken` (`action_taken`),
  INDEX `idx_acknowledged` (`acknowledged`),
  INDEX `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 风控事件审计表';

-- 3. 异地登录告警表（同 IP 段视为同区域，跨段触发告警）
CREATE TABLE IF NOT EXISTS `login_geo_alert` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_type`        VARCHAR(32) NOT NULL COMMENT 'admin/tenant/agent/enduser',
  `user_id`          BIGINT UNSIGNED NOT NULL,
  `username`         VARCHAR(64) NOT NULL,
  `current_ip`       VARCHAR(45) NOT NULL,
  `current_network`  VARCHAR(64) NOT NULL COMMENT '当前 IP 网段（如 1.2.3.0/24）',
  `previous_ip`      VARCHAR(45) NOT NULL,
  `previous_network` VARCHAR(64) NOT NULL COMMENT '上次 IP 网段',
  `user_agent`       VARCHAR(512) NOT NULL DEFAULT '',
  `alert_status`     VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/acknowledged/closed',
  `notify_channels`  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '已通知的渠道逗号分隔：inapp,email,sms',
  `acknowledged_by`  VARCHAR(64) NOT NULL DEFAULT '',
  `acknowledged_at`  DATETIME NULL,
  `closed_at`        DATETIME NULL,
  `created_at`       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  INDEX `idx_user_type_id` (`user_type`, `user_id`),
  INDEX `idx_alert_status` (`alert_status`),
  INDEX `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 异地登录告警表';

-- 4. 扩展 app_device 表：补充多维设备指纹上下文（向后兼容，老 SDK 不传则为空）
ALTER TABLE `app_device`
  ADD COLUMN `hwid_components` TEXT NULL COMMENT 'v0.4.0 多维指纹 JSON（cpu/motherboard/mac/disk/bios 等）',
  ADD COLUMN `user_agent`      VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'v0.4.0 客户端 UA',
  ADD COLUMN `client_ip_ext`   VARCHAR(45)  NOT NULL DEFAULT '' COMMENT 'v0.4.0 首次绑定 IP（与 ip_address 区分避免污染现有逻辑）',
  ADD COLUMN `screen_resolution` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'v0.4.0 屏幕分辨率 1920x1080',
  ADD COLUMN `timezone`        VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'v0.4.0 客户端时区 Asia/Shanghai',
  ADD COLUMN `language`        VARCHAR(32)  NOT NULL DEFAULT '' COMMENT 'v0.4.0 客户端语言 zh-CN';

-- 5. sys_config 新增 cloudflare.* 与 risk.* 共 15 项配置（铁律 05）
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
-- Cloudflare WAF 集成（4 项）
('cloudflare.enabled',           '0',  'bool',   'Cloudflare WAF 集成开关',           'security', 'v0.4.0 1=启用 CF 真实 IP 中间件，从 CF-Connecting-IP 头取真实客户端 IP；0=直接用 c.ClientIP()'),
('cloudflare.real_ip_header',    'CF-Connecting-IP', 'string', 'CF 真实 IP 头名',     'security', 'v0.4.0 Cloudflare 注入的真实 IP 头名称，默认 CF-Connecting-IP'),
('cloudflare.ip_country_header', 'CF-IPCountry',     'string', 'CF 国家代码头名',     'security', 'v0.4.0 Cloudflare 注入的国家代码头名称，默认 CF-IPCountry（ISO 3166-1 alpha-2）'),
('cloudflare.trusted_cidrs',     '',                 'string', 'CF 受信 CIDR 列表',   'security', 'v0.4.0 逗号分隔 CIDR 列表（仅来自这些网段的请求才信任 CF 头）；空=不校验来源（仅内网测试用）'),
-- 风控规则引擎总开关（3 项）
('risk.engine.enabled',          '1',  'bool',   '风控规则引擎总开关',                'security', 'v0.4.0 1=启用登录/验证请求风控评估；0=关闭所有风控规则'),
('risk.engine.score_threshold',  '80', 'number', '风控评分阈值',                      'security', 'v0.4.0 单次请求累计评分 >= 此值触发 action（0-100）；block 规则不受此阈值限制'),
('risk.engine.default_action',   'alert', 'string', '风控默认动作',                  'security', 'v0.4.0 alert=仅记录告警 / challenge=要求二次验证 / block=拒绝请求'),
-- 异地登录告警（4 项）
('risk.geo_login_alert.enabled', '1',  'bool',   '异地登录告警开关',                  'security', 'v0.4.0 1=启用；0=关闭。基于 IP 网段比较（IPv4 /24，IPv6 /64），无需 GeoIP 数据库'),
('risk.geo_login_alert.ipv4_prefix', '24', 'number', '异地告警 IPv4 前缀长度',      'security', 'v0.4.0 IPv4 网段前缀长度，默认 24（同 /24 视为同区域）；建议 16-32'),
('risk.geo_login_alert.ipv6_prefix', '64', 'number', '异地告警 IPv6 前缀长度',      'security', 'v0.4.0 IPv6 网段前缀长度，默认 64（同 /64 视为同区域）；建议 48-128'),
('risk.geo_login_alert.notify_channels', 'inapp,email', 'string', '异地登录通知渠道', 'security', 'v0.4.0 逗号分隔：inapp=站内信，email=邮件，sms=短信（需 notify 包支持）'),
-- 其他风控规则开关（4 项）
('risk.new_device_alert.enabled', '1', 'bool', '新设备登录告警开关', 'security', 'v0.4.0 1=首次见到的设备指纹登录时告警；0=关闭'),
('risk.abnormal_ua_alert.enabled', '1', 'bool', '异常 UA 告警开关', 'security', 'v0.4.0 1=爬虫/curl/空 UA 等非浏览器 UA 触发告警；0=关闭'),
('risk.abnormal_time_alert.enabled', '0', 'bool', '异常时段告警开关', 'security', 'v0.4.0 1=凌晨 02:00-05:00 登录触发告警；0=关闭（默认关闭避免误报）'),
('risk.abnormal_time_start', '02:00', 'string', '异常时段开始时间', 'security', 'v0.4.0 HH:MM 格式，默认 02:00；abnormal_time_alert.enabled=1 时生效'),
('risk.abnormal_time_end',   '05:00', 'string', '异常时段结束时间', 'security', 'v0.4.0 HH:MM 格式，默认 05:00；abnormal_time_alert.enabled=1 时生效')
ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);

-- 6. seed 5 条内置风控规则（status=active 默认启用；管理员可禁用）
INSERT INTO `risk_rule` (`name`, `description`, `rule_type`, `condition`, `score`, `action`, `priority`, `status`, `created_by`) VALUES
('异地登录检测',     '登录 IP 网段与上次登录不同时触发告警',                       'geo_login',      '{"ipv4_prefix":24,"ipv6_prefix":64}', 60, 'alert',    10, 'active', 'system'),
('新设备登录检测',   '首次见到的设备指纹登录时触发告警',                           'new_device',     '{"check_fields":["hwid"]}',          40, 'alert',    20, 'active', 'system'),
('异常 UA 检测',     'curl/爬虫/空 UA 等非浏览器 UA 触发告警',                     'abnormal_ua',    '{"block_bots":true}',                30, 'alert',    30, 'active', 'system'),
('异常时段检测',     '凌晨 02:00-05:00 登录触发告警（默认禁用，可手动启用）',      'abnormal_time',  '{"start":"02:00","end":"05:00"}',    20, 'alert',    40, 'disabled','system'),
('高频请求检测',     '同 IP 同账号 60 秒内登录/验证 >10 次触发告警',               'high_frequency', '{"window_seconds":60,"threshold":10}',50, 'challenge',50, 'active', 'system');
