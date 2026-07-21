-- v0.4.0 终端用户体系（H5 用户注册 / 登录 / 卡密绑定 / 个人中心）
-- 1. end_user 表：终端用户（用户名 / 邮箱 / 手机 / 密码 / 应用绑定）
-- 2. end_user_card 表：用户-卡密绑定关系（一个用户可绑多张卡，一张卡只能绑一个用户）
-- 3. end_user_token 表：refresh token（jti 单点踢出兼容）
-- 4. sys_config 10 项：注册/登录/密码/验证码/Token 配置
-- 5. app_card 表新增 end_user_id 字段（向前兼容，可空）
--
-- 幂等性说明（v0.6.1 修复）：
--   * 历史问题：原版本使用 `ADD COLUMN IF NOT EXISTS` / `ADD INDEX IF NOT EXISTS`，
--     这两个语法仅 MariaDB 支持，MySQL 8.0 任意小版本都会抛 syntax error，
--     导致 v15 在 ALTER TABLE 处必然失败，前序 CREATE TABLE 因 DDL 隐式提交已落库，
--     最终 schema_migrations 留下 dirty=1 的"半成品"状态。
--   * 修复策略：所有 DDL 改用 INFORMATION_SCHEMA + PREPARE/EXECUTE 做存在性检查，
--     在 MySQL 8.0.x 与 MariaDB 上均可重复执行；DDL 失败后重新执行也能继续推进。
--   * 禁止使用 DROP TABLE / DROP COLUMN / DROP INDEX 作为正常迁移逻辑。

-- ============== end_user ==============
CREATE TABLE IF NOT EXISTS `end_user` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id`      BIGINT UNSIGNED NOT NULL COMMENT '归属租户（开发者）',
    `app_id`         BIGINT UNSIGNED NOT NULL COMMENT '归属应用',
    `username`       VARCHAR(64)  NOT NULL COMMENT '用户名（应用内唯一）',
    `phone`          VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '手机号（可空，带国际区号）',
    `email`          VARCHAR(128) NOT NULL DEFAULT '' COMMENT '邮箱（可空）',
    `password_hash`  VARCHAR(255) NOT NULL COMMENT 'bcrypt(cost=12) 密码哈希',
    `nickname`       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '昵称',
    `avatar_url`     VARCHAR(512) NOT NULL DEFAULT '' COMMENT '头像 URL',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'active' COMMENT 'active / banned / deleted',
    `last_login_at`  DATETIME     NULL COMMENT '最近登录时间',
    `last_login_ip`  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '最近登录 IP',
    `last_login_ua`  VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最近登录 User-Agent',
    `login_count`    INT          NOT NULL DEFAULT 0 COMMENT '累计登录次数',
    `remark`         VARCHAR(255) NOT NULL DEFAULT '' COMMENT '备注',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_tenant_app_username` (`tenant_id`, `app_id`, `username`),
    KEY `idx_phone` (`phone`),
    KEY `idx_email` (`email`),
    KEY `idx_status` (`status`),
    KEY `idx_tenant_app` (`tenant_id`, `app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 终端用户';

-- ============== end_user_card ==============
CREATE TABLE IF NOT EXISTS `end_user_card` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`        BIGINT UNSIGNED NOT NULL COMMENT '终端用户 ID',
    `card_id`        BIGINT UNSIGNED NOT NULL COMMENT '卡密 ID',
    `tenant_id`      BIGINT UNSIGNED NOT NULL COMMENT '冗余租户 ID（便于查询）',
    `app_id`         BIGINT UNSIGNED NOT NULL COMMENT '冗余应用 ID',
    `bound_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '绑定时间',
    `unbound_at`     DATETIME     NULL COMMENT '解绑时间',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'active' COMMENT 'active / unbound',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_card_id` (`card_id`),
    KEY `idx_user` (`user_id`),
    KEY `idx_tenant_app` (`tenant_id`, `app_id`),
    KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 终端用户-卡密绑定关系';

-- ============== end_user_token ==============
CREATE TABLE IF NOT EXISTS `end_user_token` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`        BIGINT UNSIGNED NOT NULL COMMENT '终端用户 ID',
    `jti`            VARCHAR(64)  NOT NULL COMMENT 'JWT ID（用于精准单点踢出）',
    `device_name`    VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设备名（UA 解析）',
    `device_type`    VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '设备类型 pc/mobile/bot',
    `ip`             VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '登录 IP',
    `user_agent`     VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'User-Agent',
    `refresh_token`  VARCHAR(255) NOT NULL COMMENT 'refresh token 哈希（SHA-512）',
    `expires_at`     DATETIME     NOT NULL COMMENT '过期时间',
    `revoked_at`     DATETIME     NULL COMMENT '撤销时间（注销/踢出）',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_jti` (`jti`),
    KEY `idx_user` (`user_id`),
    KEY `idx_expires` (`expires_at`),
    KEY `idx_revoked` (`revoked_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 终端用户 Refresh Token';

-- ============== app_card 表新增 end_user_id 字段（幂等，兼容 MySQL 8.0 / MariaDB） ==============
-- 不使用 `ADD COLUMN IF NOT EXISTS`（MariaDB only），改用 INFORMATION_SCHEMA + PREPARE/EXECUTE
-- 通过 DATABASE() 取当前库，避免硬编码 schema 名
-- @col_exists = 1 表示字段已存在，跳过 ALTER；= 0 才执行 ALTER
SET @col_exists := (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME   = 'app_card'
      AND COLUMN_NAME  = 'end_user_id'
);
SET @ddl := IF(@col_exists = 0,
    'ALTER TABLE `app_card` ADD COLUMN `end_user_id` BIGINT UNSIGNED NULL DEFAULT NULL COMMENT ''v0.4.0 绑定的终端用户 ID（可空）'' AFTER `bound_device_id`',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- ============== app_card 表新增 idx_end_user_id 索引（幂等，兼容 MySQL 8.0 / MariaDB） ==============
-- 不使用 `ADD INDEX IF NOT EXISTS`（MariaDB only），改用 INFORMATION_SCHEMA.STATISTICS + PREPARE/EXECUTE
-- 仅当上一轮字段创建后(或已存在)才需要建索引；索引存在则跳过
SET @idx_exists := (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME   = 'app_card'
      AND INDEX_NAME   = 'idx_end_user_id'
);
-- 仅当字段已存在但索引不存在时，才执行 ADD INDEX
SET @col_exists2 := (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME   = 'app_card'
      AND COLUMN_NAME  = 'end_user_id'
);
SET @ddl2 := IF(@idx_exists = 0 AND @col_exists2 = 1,
    'ALTER TABLE `app_card` ADD INDEX `idx_end_user_id` (`end_user_id`)',
    'SELECT 1');
PREPARE stmt2 FROM @ddl2;
EXECUTE stmt2;
DEALLOCATE PREPARE stmt2;

-- ============== sys_config 10 项 ==============
-- 铁律 04/05：注册/登录/密码/验证码/Token 配置 全部走 sys_config
-- 用 INSERT ... ON DUPLICATE KEY UPDATE 保证可重复执行
-- 注意：sys_config.config_key 须有 UNIQUE 索引（001_init_schema 已建）
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('enduser.register_enabled',          '1',                                'bool',   '终端用户注册开关',           'enduser', 'v0.4.0 1=允许注册；0=仅管理员可创建'),
('enduser.login_method',              'username',                         'string', '登录方式',                   'enduser', 'v0.4.0 username/phone/email，多选用逗号分隔'),
('enduser.password_min_length',       '8',                                'number', '密码最小长度',               'enduser', 'v0.4.0 密码最小位数，默认 8'),
('enduser.verify_code_ttl',           '5',                                'number', '验证码有效期（分钟）',       'enduser', 'v0.4.0 验证码 TTL，默认 5 分钟'),
('enduser.verify_code_length',        '6',                                'number', '验证码长度',                 'enduser', 'v0.4.0 验证码位数，默认 6 位数字'),
('enduser.access_token_ttl',          '2',                                'number', 'Access Token 有效期（小时）','enduser', 'v0.4.0 JWT access token 有效期，默认 2 小时'),
('enduser.refresh_token_ttl',         '30',                               'number', 'Refresh Token 有效期（天）', 'enduser', 'v0.4.0 refresh token 有效期，默认 30 天'),
('enduser.bind_card_per_user_max',    '10',                               'number', '单用户最多绑定卡密数',       'enduser', 'v0.4.0 单个终端用户最多绑定的卡密数量，0=不限'),
('enduser.allow_anonymous_query',     '1',                                'bool',   '允许匿名查卡',               'enduser', 'v0.4.0 1=未登录用户可按 card_key 查询；0=必须登录'),
('enduser.ip_rate_limit_per_minute',  '20',                               'number', 'IP 每分钟限流',              'enduser', 'v0.4.0 单 IP 每分钟最多请求注册/登录次数')
ON DUPLICATE KEY UPDATE `config_value` = VALUES(`config_value`),
                        `config_type`  = VALUES(`config_type`),
                        `config_name`  = VALUES(`config_name`),
                        `config_group` = VALUES(`config_group`),
                        `remark`       = VALUES(`remark`);
