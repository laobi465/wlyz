-- v0.4.0 API 开放平台（开发者 API Token + Webhook 事件推送 + 第三方接入授权）
-- 1. developer_api_token 表：开发者 API Token（SHA-512 哈希存储 + scopes 权限）
-- 2. webhook_endpoint 表：Webhook 推送端点（事件订阅 + HMAC-SHA256 签名）
-- 3. webhook_delivery 表：Webhook 推送日志（payload + 状态 + 重试）
-- 4. sys_config 8 项：Token/Webhook 全局参数
--
-- 严格遵循铁律 04/05/06：
--   04 - 无硬编码：Token 长度 / Webhook 超时 / 重试次数 / 失败阈值 全部从 sys_config 读取
--   05 - 配置走后端：8 项 openapi.* / webhook.* 配置可通过后台实时调整
--   06 - 反幻觉：Token SHA-512 哈希存储（不存明文）+ HMAC-SHA256 签名 + hmac.Equal 常量时间比较防时序攻击

-- ============== developer_api_token ==============
CREATE TABLE IF NOT EXISTS `developer_api_token` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id`      BIGINT UNSIGNED NOT NULL COMMENT '归属租户（开发者）',
    `name`           VARCHAR(64)    NOT NULL COMMENT 'Token 名称（开发者自定义）',
    `token_hash`     VARCHAR(128)   NOT NULL COMMENT 'Token SHA-512 哈希（不存明文）',
    `prefix`         VARCHAR(16)    NOT NULL COMMENT 'Token 前 8 位明文（用于展示识别）',
    `scopes`         VARCHAR(512)   NOT NULL DEFAULT '' COMMENT '权限范围（逗号分隔：card.read/card.write/order.read/order.write/agent.read/webhook.read/webhook.write 等）',
    `expires_at`     DATETIME       NULL COMMENT '过期时间（NULL=永不过期）',
    `last_used_at`   DATETIME       NULL COMMENT '最近使用时间',
    `last_used_ip`   VARCHAR(64)    NOT NULL DEFAULT '' COMMENT '最近使用 IP',
    `status`         VARCHAR(16)    NOT NULL DEFAULT 'active' COMMENT 'active / revoked',
    `revoked_at`     DATETIME       NULL COMMENT '撤销时间',
    `created_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_token_hash` (`token_hash`),
    KEY `idx_tenant` (`tenant_id`),
    KEY `idx_status` (`status`),
    KEY `idx_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 开发者 API Token';

-- ============== webhook_endpoint ==============
CREATE TABLE IF NOT EXISTS `webhook_endpoint` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id`      BIGINT UNSIGNED NOT NULL COMMENT '归属租户（开发者）',
    `name`           VARCHAR(64)    NOT NULL COMMENT '端点名称',
    `url`            VARCHAR(512)   NOT NULL COMMENT '回调 URL（https://开头）',
    `secret_enc`     VARCHAR(512)   NOT NULL DEFAULT '' COMMENT 'HMAC-SHA256 签名密钥（AES-256-GCM 加密存储）',
    `events`         VARCHAR(512)   NOT NULL DEFAULT '' COMMENT '订阅事件类型（逗号分隔：order.paid/card.generated/agent.registered 等）',
    `status`         VARCHAR(16)    NOT NULL DEFAULT 'active' COMMENT 'active / disabled',
    `failure_count`  INT            NOT NULL DEFAULT 0 COMMENT '连续失败次数（达阈值自动 disable）',
    `last_response_code` INT           NOT NULL DEFAULT 0 COMMENT '最近响应 HTTP 状态码（0=未发送）',
    `last_response_at` DATETIME       NULL COMMENT '最近响应时间',
    `last_error`     VARCHAR(512)   NOT NULL DEFAULT '' COMMENT '最近错误信息',
    `created_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_tenant` (`tenant_id`),
    KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 Webhook 推送端点';

-- ============== webhook_delivery ==============
CREATE TABLE IF NOT EXISTS `webhook_delivery` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id`      BIGINT UNSIGNED NOT NULL COMMENT '归属租户',
    `endpoint_id`    BIGINT UNSIGNED NOT NULL COMMENT '推送端点 ID',
    `event_type`     VARCHAR(64)    NOT NULL COMMENT '事件类型（如 order.paid）',
    `event_id`       VARCHAR(64)    NOT NULL COMMENT '事件唯一 ID（UUID，防重放）',
    `payload`        TEXT           NOT NULL COMMENT '事件 payload（JSON）',
    `status`         VARCHAR(16)    NOT NULL DEFAULT 'pending' COMMENT 'pending / success / failed',
    `response_code`  INT            NOT NULL DEFAULT 0 COMMENT '响应 HTTP 状态码',
    `response_body`  VARCHAR(1024)  NOT NULL DEFAULT '' COMMENT '响应 body（截断 1024）',
    `attempt_count`  INT            NOT NULL DEFAULT 0 COMMENT '尝试次数',
    `max_retry`      INT            NOT NULL DEFAULT 3 COMMENT '最大重试次数',
    `next_retry_at`  DATETIME       NULL COMMENT '下次重试时间',
    `delivered_at`   DATETIME       NULL COMMENT '成功送达时间',
    `created_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_tenant` (`tenant_id`),
    KEY `idx_endpoint` (`endpoint_id`),
    KEY `idx_status` (`status`),
    KEY `idx_event` (`event_type`),
    KEY `idx_next_retry` (`next_retry_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 Webhook 推送日志';

-- ============== sys_config 8 项 ==============
-- 铁律 04/05：Token 长度 / Webhook 超时 / 重试次数 / 失败阈值 全部走 sys_config
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('openapi.token.prefix',            'pat_',    'string', 'API Token 前缀',           'openapi', 'v0.4.0 开发者 API Token 前缀（pat_=Platform Access Token）'),
('openapi.token.length',            '40',      'number', 'API Token 随机部分长度',    'openapi', 'v0.4.0 Token 随机字符部分长度（不含前缀），默认 40 位'),
('openapi.token.max_per_tenant',    '10',      'number', '单租户最多 Token 数',       'openapi', 'v0.4.0 单个开发者最多创建的 API Token 数量，0=不限'),
('openapi.token.default_ttl_days',  '365',     'number', 'Token 默认有效期（天）',    'openapi', 'v0.4.0 新建 Token 默认有效期，0=永不过期'),
('openapi.scope.available',         'card.read,card.write,order.read,order.write,agent.read,agent.write,webhook.read,webhook.write', 'string', '可用权限范围', 'openapi', 'v0.4.0 Token 可授权的 scopes 列表（逗号分隔）'),
('webhook.timeout_seconds',         '10',      'number', 'Webhook 请求超时（秒）',    'webhook', 'v0.4.0 HTTP POST 超时时间，默认 10 秒'),
('webhook.max_retry',               '3',       'number', 'Webhook 最大重试次数',      'webhook', 'v0.4.0 失败后重试次数，默认 3 次'),
('webhook.failure_threshold',       '10',      'number', '连续失败阈值',              'webhook', 'v0.4.0 endpoint 连续失败次数达阈值自动 disable，默认 10') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
