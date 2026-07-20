-- v0.4.0 数据备份恢复系统
-- 1. system_backup_log 表：备份审计日志（操作人 / 类型 / 文件路径 / 大小 / 状态 / 校验和）
-- 2. sys_config 5 项：备份目录 / 保留天数 / 自动备份开关 / 加密密钥 / 备份策略

-- ============== system_backup_log ==============
CREATE TABLE IF NOT EXISTS `system_backup_log` (
    `id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `backup_type    VARCHAR(32)  NOT NULL DEFAULT 'manual' COMMENT 'manual / auto / restore_source',
    `trigger_by     BIGINT       NOT NULL DEFAULT 0 COMMENT '触发者 admin id（auto 时为 0）',
    `trigger_ip     VARCHAR(45)  NOT NULL DEFAULT '' COMMENT '触发者 IP',
    `file_path      VARCHAR(512) NOT NULL DEFAULT '' COMMENT '备份文件相对路径',
    `file_size      BIGINT       NOT NULL DEFAULT 0 COMMENT '文件大小（字节）',
    `checksum       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'SHA-256 校验和',
    `status         VARCHAR(32)  NOT NULL DEFAULT 'pending' COMMENT 'pending / running / success / failed / deleted',
    `error_message  VARCHAR(512) NOT NULL DEFAULT '' COMMENT '失败原因摘要',
    `duration_ms    INT          NOT NULL DEFAULT 0 COMMENT '总耗时（毫秒）',
    `tables_count   INT          NOT NULL DEFAULT 0 COMMENT '备份的表数量',
    `rows_count     BIGINT       NOT NULL DEFAULT 0 COMMENT '备份的总行数',
    `restored_from  BIGINT       NOT NULL DEFAULT 0 COMMENT '若为恢复，原备份 id（0=非恢复）',
    `created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_backup_status` (`status`),
    KEY `idx_backup_type` (`backup_type`),
    KEY `idx_backup_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 数据备份审计日志';

-- ============== sys_config 5 项 ==============
-- 铁律 04/05：备份目录 / 保留天数 / 自动开关 / 加密密钥 / 策略 全部走 sys_config
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('backup.dir',                'data/backups',                    'string', '备份目录（相对项目根）',     'backup', 'v0.4.0 备份文件存放目录，相对项目根目录'),
('backup.retention_days',     '30',                              'number', '备份保留天数',              'backup', 'v0.4.0 自动清理超过此天数的备份文件（0=永不清理）'),
('backup.auto_enabled',       '0',                               'bool',   '自动备份开关',              'backup', 'v0.4.0 1=每天凌晨自动备份；0=仅手动触发'),
('backup.encryption_key',     '',                                'string', '备份加密密钥（AES-256）',   'backup', 'v0.4.0 备份文件 AES-256-GCM 加密密钥（hex 编码 32 字节），空=不加密'),
('backup.compress',           '1',                               'bool',   '备份压缩开关',              'backup', 'v0.4.0 1=gzip 压缩备份文件；0=不压缩'),
('backup.tables_filter',      '',                                'string', '备份表过滤',                'backup', 'v0.4.0 逗号分隔表名白名单，空=备份所有业务表（不含 sys_admin）') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
