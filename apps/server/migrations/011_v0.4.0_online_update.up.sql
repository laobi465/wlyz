-- v0.4.0 在线更新系统
-- 1. system_update_log 表：审计日志（操作人 / 触发源 / 前后版本 / 状态 / 日志）
-- 2. sys_config 4 项：webhook_secret / branch / auto_update / deploy_script

-- ============== system_update_log ==============
CREATE TABLE IF NOT EXISTS `system_update_log` (
    `id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `trigger_source  VARCHAR(32)  NOT NULL DEFAULT 'manual' COMMENT '触发源：webhook / manual / rollback',
    `trigger_by      BIGINT       NOT NULL DEFAULT 0 COMMENT '触发者 admin id（webhook 时为 0）',
    `trigger_ip      VARCHAR(45)  NOT NULL DEFAULT '' COMMENT '触发者 IP',
    `commit_before   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新前 commit hash',
    `commit_after    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新后 commit hash',
    `branch          VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '目标分支',
    `status          VARCHAR(32)  NOT NULL DEFAULT 'pending' COMMENT 'pending / running / success / failed / rolled_back',
    `steps_json      TEXT         COMMENT '执行步骤 JSON 数组 [{step,status,duration_ms,error}]',
    `log_text        MEDIUMTEXT   COMMENT '完整执行日志文本',
    `error_message   VARCHAR(512) NOT NULL DEFAULT '' COMMENT '失败原因摘要',
    `duration_ms     INT          NOT NULL DEFAULT 0 COMMENT '总耗时（毫秒）',
    `rolled_back_from BIGINT      NOT NULL DEFAULT 0 COMMENT '若为回滚，原失败更新 id（0=非回滚）',
    `created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_update_log_status` (`status`),
    KEY `idx_update_log_created` (`created_at`),
    KEY `idx_update_log_trigger` (`trigger_source`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 在线更新审计日志';

-- ============== sys_config 4 项 ==============
-- 铁律 04/05：webhook 密钥 / 分支 / 自动更新开关 / 部署脚本路径全部走 sys_config，可后台调整
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('update.webhook.secret',       '',                                'string', 'GitHub Webhook 密钥',         'update', 'v0.4.0 GitHub Webhook HMAC-SHA256 签名校验密钥（X-Hub-Signature-256 头），空=不校验'),
('update.webhook.branch',       'main',                            'string', 'Webhook 监听分支',            'update', 'v0.4.0 仅 push 到此分支时触发更新（默认 main）'),
('update.webhook.auto_update',  '0',                               'bool',   'Webhook 自动更新开关',        'update', 'v0.4.0 1=收到 webhook 后自动执行更新；0=仅记录通知，需管理员手动触发'),
('update.deploy.script_path',   'scripts/deploy_update.sh',        'string', '部署脚本相对路径',            'update', 'v0.4.0 更新执行的 shell 脚本（git pull + build + restart），相对项目根目录'),
('update.healthcheck.url',      'http://localhost:8080/health',    'string', '更新后健康检查 URL',          'update', 'v0.4.0 更新完成后 GET 此 URL 验证服务可用性，2xx/3xx 视为成功'),
('update.healthcheck.timeout',  '30',                              'number', '健康检查超时（秒）',          'update', 'v0.4.0 健康检查最大等待秒数'),
('update.rollback.enabled',     '1',                               'bool',   '失败自动回滚开关',            'update', 'v0.4.0 更新失败后是否自动回滚到上一版本'),
('update.lock.timeout',         '600',                             'number', '更新锁超时（秒）',            'update', 'v0.4.0 更新过程互斥锁最大持有时间，超时自动释放（防死锁）') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
