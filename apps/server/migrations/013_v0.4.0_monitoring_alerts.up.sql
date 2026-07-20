-- v0.4.0 监控告警系统
-- 1. system_metric 表：时序指标数据（简化版，每分钟采集一次）
-- 2. system_alert 表：告警事件（触发时间 / 严重程度 / 状态 / 关联 metric）
-- 3. sys_config 7 项：采集间隔 / 阈值 / 通知 webhook / 静默期 / 告警开关

-- ============== system_metric ==============
CREATE TABLE IF NOT EXISTS `system_metric` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `metric_name`    VARCHAR(64)  NOT NULL COMMENT '指标名：cpu_usage/memory_usage/disk_usage/qps/verify_count/online_devices/error_rate',
    `metric_value`   DOUBLE       NOT NULL COMMENT '指标值（百分比/计数/比率）',
    `metric_unit`    VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '单位：%/count/ratio/mb',
    `labels_json`    VARCHAR(512) NOT NULL DEFAULT '{}' COMMENT '标签 JSON（如 {"tenant_id":"1","app_id":"2"}）',
    `collected_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '采集时间',
    PRIMARY KEY (`id`),
    KEY `idx_metric_name_time` (`metric_name`, `collected_at`),
    KEY `idx_metric_time` (`collected_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 系统指标时序数据';

-- ============== system_alert ==============
CREATE TABLE IF NOT EXISTS `system_alert` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `alert_rule`     VARCHAR(64)  NOT NULL COMMENT '触发的告警规则名（与 system_metric.metric_name 对应）',
    `severity`       VARCHAR(16)  NOT NULL DEFAULT 'warning' COMMENT 'info / warning / critical / fatal',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'firing' COMMENT 'firing / resolved / silenced / acked',
    `metric_value`   DOUBLE       NOT NULL COMMENT '触发时的指标值',
    `threshold`      DOUBLE       NOT NULL COMMENT '告警阈值',
    `operator`       VARCHAR(8)   NOT NULL DEFAULT '>' COMMENT '比较运算符：> / < / >= / <= / ==',
    `message`        VARCHAR(512) NOT NULL DEFAULT '' COMMENT '告警消息',
    `labels_json`    VARCHAR(512) NOT NULL DEFAULT '{}' COMMENT '标签 JSON',
    `fired_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '触发时间',
    `resolved_at`    DATETIME     NULL COMMENT '恢复时间（firing 状态为 NULL）',
    `acked_by`       BIGINT       NOT NULL DEFAULT 0 COMMENT '确认人 admin id',
    `acked_at`       DATETIME     NULL COMMENT '确认时间',
    `notify_sent`    TINYINT      NOT NULL DEFAULT 0 COMMENT '是否已发送通知：0=未发送 1=已发送',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_alert_status` (`status`),
    KEY `idx_alert_rule` (`alert_rule`),
    KEY `idx_alert_severity` (`severity`),
    KEY `idx_alert_fired` (`fired_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 告警事件';

-- ============== sys_config 7 项 ==============
-- 铁律 04/05：采集间隔 / 阈值 / 通知 webhook / 静默期 / 告警开关 全部走 sys_config
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('monitor.collect_interval',          '60',                                  'number', '指标采集间隔（秒）',         'monitor', 'v0.4.0 系统指标采集周期，默认 60 秒'),
('monitor.alert_enabled',             '1',                                   'bool',   '告警总开关',                 'monitor', 'v0.4.0 1=启用告警评估；0=仅采集不告警'),
('monitor.notify.webhook_url',        '',                                    'string', '告警通知 Webhook URL',       'monitor', 'v0.4.0 告警通知接收 URL（POST JSON payload），空=不发通知'),
('monitor.silence_minutes',           '30',                                  'number', '告警静默期（分钟）',         'monitor', 'v0.4.0 同一规则重复告警的静默窗口，默认 30 分钟'),
('monitor.threshold.cpu_usage',       '90',                                  'number', 'CPU 使用率阈值（%）',         'monitor', 'v0.4.0 CPU 使用率告警阈值，超过即触发 warning'),
('monitor.threshold.memory_usage',    '90',                                  'number', '内存使用率阈值（%）',         'monitor', 'v0.4.0 内存使用率告警阈值，超过即触发 warning'),
('monitor.threshold.disk_usage',      '85',                                  'number', '磁盘使用率阈值（%）',         'monitor', 'v0.4.0 磁盘使用率告警阈值，超过即触发 critical'),
('monitor.threshold.error_rate',      '10',                                  'number', '错误率阈值（%）',             'monitor', 'v0.4.0 验证错误率告警阈值，超过即触发 critical'),
('monitor.retention_days',            '30',                                  'number', '指标数据保留天数',           'monitor', 'v0.4.0 超过此天数的指标数据自动清理（0=永不清理）') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
