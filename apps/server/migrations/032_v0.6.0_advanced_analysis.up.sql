-- v0.6.0 高级分析（用户行为分析 / 卡密使用画像 / 风险用户识别）
-- 严格遵循铁律 05：所有可变参数走 sys_config 后台可视化编辑
-- 严格遵循铁律 04：配置键名集中声明，禁止硬编码

-- ============== 1. 3 张分析表 ==============

-- 1.1 终端用户行为画像（按日聚合）
CREATE TABLE IF NOT EXISTS `user_behavior_profile` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id` BIGINT UNSIGNED NOT NULL,
    `app_id` BIGINT UNSIGNED NOT NULL,
    `end_user_id` BIGINT UNSIGNED NOT NULL,
    `stat_date` VARCHAR(10) NOT NULL COMMENT 'YYYY-MM-DD',
    `login_count` INT NOT NULL DEFAULT 0,
    `verify_count` INT NOT NULL DEFAULT 0,
    `heartbeat_count` INT NOT NULL DEFAULT 0,
    `bind_count` INT NOT NULL DEFAULT 0,
    `unbind_count` INT NOT NULL DEFAULT 0,
    `success_count` INT NOT NULL DEFAULT 0,
    `fail_count` INT NOT NULL DEFAULT 0,
    `banned_count` INT NOT NULL DEFAULT 0,
    `distinct_ip_count` INT NOT NULL DEFAULT 0,
    `distinct_device_count` INT NOT NULL DEFAULT 0,
    `first_active_at` DATETIME NULL,
    `last_active_at` DATETIME NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_date` (`end_user_id`, `stat_date`),
    KEY `idx_tenant_app_date` (`tenant_id`, `app_id`, `stat_date`),
    KEY `idx_stat_date` (`stat_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='终端用户行为画像（按日聚合）';

-- 1.2 卡密使用画像（按日聚合）
CREATE TABLE IF NOT EXISTS `card_usage_profile` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id` BIGINT UNSIGNED NOT NULL,
    `app_id` BIGINT UNSIGNED NOT NULL,
    `card_id` BIGINT UNSIGNED NOT NULL,
    `stat_date` VARCHAR(10) NOT NULL COMMENT 'YYYY-MM-DD',
    `verify_count` INT NOT NULL DEFAULT 0,
    `heartbeat_count` INT NOT NULL DEFAULT 0,
    `bind_count` INT NOT NULL DEFAULT 0,
    `success_count` INT NOT NULL DEFAULT 0,
    `fail_count` INT NOT NULL DEFAULT 0,
    `banned_count` INT NOT NULL DEFAULT 0,
    `device_mismatch_count` INT NOT NULL DEFAULT 0,
    `distinct_ip_count` INT NOT NULL DEFAULT 0,
    `distinct_device_count` INT NOT NULL DEFAULT 0,
    `first_active_at` DATETIME NULL,
    `last_active_at` DATETIME NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_card_date` (`card_id`, `stat_date`),
    KEY `idx_tenant_app_date` (`tenant_id`, `app_id`, `stat_date`),
    KEY `idx_stat_date` (`stat_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='卡密使用画像（按日聚合）';

-- 1.3 用户风险评分累计表
CREATE TABLE IF NOT EXISTS `user_risk_score` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `app_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `user_type` VARCHAR(32) NOT NULL COMMENT 'admin/tenant/agent/enduser',
    `user_id` BIGINT UNSIGNED NOT NULL,
    `username` VARCHAR(64) NOT NULL DEFAULT '',
    `risk_score` INT NOT NULL DEFAULT 0 COMMENT '累计评分（含衰减）',
    `risk_level` VARCHAR(16) NOT NULL DEFAULT 'low' COMMENT 'low/medium/high/critical',
    `event_count` INT NOT NULL DEFAULT 0,
    `high_freq_hits` INT NOT NULL DEFAULT 0,
    `geo_anomaly_hits` INT NOT NULL DEFAULT 0,
    `new_device_hits` INT NOT NULL DEFAULT 0,
    `abnormal_ua_hits` INT NOT NULL DEFAULT 0,
    `fail_rate_high_hits` INT NOT NULL DEFAULT 0 COMMENT '失败率超阈值次数',
    `multi_ip_hits` INT NOT NULL DEFAULT 0 COMMENT '24h 内多 IP 命中次数',
    `multi_dev_hits` INT NOT NULL DEFAULT 0 COMMENT '24h 内多设备命中次数',
    `last_event_at` DATETIME NULL,
    `last_eval_at` DATETIME NULL COMMENT '最近一次评分重算时间',
    `banned` TINYINT(1) NOT NULL DEFAULT 0,
    `banned_reason` VARCHAR(255) NOT NULL DEFAULT '',
    `banned_at` DATETIME NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_type_id` (`user_type`, `user_id`),
    KEY `idx_risk_level` (`risk_level`),
    KEY `idx_banned` (`banned`),
    KEY `idx_risk_score` (`risk_score`),
    KEY `idx_tenant_app` (`tenant_id`, `app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户风险评分累计表';

-- ============== 2. sys_config 配置项（10 项） ==============

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_group`, `description`, `is_sensitive`, `created_at`, `updated_at`) VALUES
-- 总开关
('analysis.enabled', '1', 'bool', 'analysis', '高级分析总开关', 0, NOW(), NOW()),
-- 三大模块独立开关
('analysis.behavior.enabled', '1', 'bool', 'analysis', '用户行为分析模块开关', 0, NOW(), NOW()),
('analysis.card_profile.enabled', '1', 'bool', 'analysis', '卡密使用画像模块开关', 0, NOW(), NOW()),
('analysis.risk_score.enabled', '1', 'bool', 'analysis', '风险评分模块开关', 0, NOW(), NOW()),
-- 风险评分阈值
('analysis.risk_score.high_threshold', '70', 'int', 'analysis', '高风险阈值（达到则标记 high，>=critical_threshold 自动封禁候选）', 0, NOW(), NOW()),
('analysis.risk_score.medium_threshold', '40', 'int', 'analysis', '中风险阈值（达到则标记 medium）', 0, NOW(), NOW()),
('analysis.risk_score.critical_threshold', '100', 'int', 'analysis', '致命风险阈值（达到自动封禁候选）', 0, NOW(), NOW()),
-- 聚合参数
('analysis.aggregate_interval_seconds', '3600', 'int', 'analysis', '聚合 worker 运行间隔（秒）', 0, NOW(), NOW()),
('analysis.top_n', '20', 'int', 'analysis', 'TOP N 列表长度', 0, NOW(), NOW()),
('analysis.lookback_days', '30', 'int', 'analysis', '回溯分析天数', 0, NOW(), NOW());

-- ============== 3. 风险评分权重配置（7 项，可调） ==============

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_group`, `description`, `is_sensitive`, `created_at`, `updated_at`) VALUES
('analysis.risk_score.weight.high_freq', '25', 'int', 'analysis', '高频请求风险权重', 0, NOW(), NOW()),
('analysis.risk_score.weight.geo_anomaly', '20', 'int', 'analysis', '异地登录风险权重', 0, NOW(), NOW()),
('analysis.risk_score.weight.new_device', '10', 'int', 'analysis', '新设备风险权重', 0, NOW(), NOW()),
('analysis.risk_score.weight.abnormal_ua', '15', 'int', 'analysis', '异常 UA 风险权重', 0, NOW(), NOW()),
('analysis.risk_score.weight.fail_rate_high', '20', 'int', 'analysis', '失败率超阈值风险权重', 0, NOW(), NOW()),
('analysis.risk_score.weight.multi_ip', '15', 'int', 'analysis', '24h 多 IP 风险权重', 0, NOW(), NOW()),
('analysis.risk_score.weight.multi_dev', '15', 'int', 'analysis', '24h 多设备风险权重', 0, NOW(), NOW());

-- ============== 4. 异常模式阈值（4 项） ==============

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_group`, `description`, `is_sensitive`, `created_at`, `updated_at`) VALUES
('analysis.risk_score.threshold.fail_rate', '50', 'int', 'analysis', '失败率告警阈值（百分比，>50% 触发加分）', 0, NOW(), NOW()),
('analysis.risk_score.threshold.multi_ip_count', '3', 'int', 'analysis', '24h 内多 IP 告警阈值（>=3 触发加分）', 0, NOW(), NOW()),
('analysis.risk_score.threshold.multi_dev_count', '5', 'int', 'analysis', '24h 内多设备告警阈值（>=5 触发加分）', 0, NOW(), NOW()),
('analysis.risk_score.decay_days', '7', 'int', 'analysis', '风险评分衰减周期（天，每天衰减 1/decay_days）', 0, NOW(), NOW());
