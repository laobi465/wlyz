-- v0.4.0 通知系统（短信 / 邮件 / 站内信 三通道）
-- 1. notify_template 表：通知模板（渠道 / 代码 / 标题 / 内容 / 变量占位符）
-- 2. notify_log 表：发送日志（接收人 / 模板 / 状态 / 重试次数 / 错误信息）
-- 3. sys_config 12 项：三通道开关与配置 + 重试策略 + 限流

-- ============== notify_template ==============
CREATE TABLE IF NOT EXISTS `notify_template` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `code`           VARCHAR(64)  NOT NULL COMMENT '模板代码（如 verify_code / order_paid / card_expiring / agent_commission）',
    `name`           VARCHAR(128) NOT NULL COMMENT '模板名称',
    `channel`        VARCHAR(16)  NOT NULL COMMENT '渠道：sms / email / inapp',
    `subject`        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '标题（email 用，sms/inapp 留空）',
    `content`        TEXT         NOT NULL COMMENT '模板内容，支持 {{var}} 占位符',
    `variables`      VARCHAR(512) NOT NULL DEFAULT '[]' COMMENT '变量列表 JSON，如 ["code","app_name"]',
    `tenant_id`      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '归属租户（0=平台通用模板）',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'enabled' COMMENT 'enabled / disabled',
    `remark`         VARCHAR(255) NOT NULL DEFAULT '' COMMENT '备注',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code_channel_tenant` (`code`, `channel`, `tenant_id`),
    KEY `idx_channel` (`channel`),
    KEY `idx_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 通知模板';

-- ============== notify_log ==============
CREATE TABLE IF NOT EXISTS `notify_log` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `template_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '模板 ID（0=无模板直接发送）',
    `template_code`  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '模板代码冗余（便于查询）',
    `channel`        VARCHAR(16)  NOT NULL COMMENT 'sms / email / inapp',
    `recipient`      VARCHAR(255) NOT NULL COMMENT '接收人（手机号 / 邮箱 / user_id）',
    `subject`        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '标题',
    `content`        TEXT         NOT NULL COMMENT '实际发送内容（变量已替换）',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT 'pending / sent / failed',
    `provider_msgid` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '服务商返回的消息 ID',
    `error_message`  VARCHAR(512) NOT NULL DEFAULT '' COMMENT '失败原因',
    `retry_count`    INT          NOT NULL DEFAULT 0 COMMENT '重试次数',
    `priority`       TINYINT      NOT NULL DEFAULT 0 COMMENT '0=普通 1=高 2=紧急',
    `tenant_id`      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '归属租户',
    `sent_at`        DATETIME     NULL COMMENT '发送完成时间',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_status` (`status`),
    KEY `idx_channel` (`channel`),
    KEY `idx_recipient` (`recipient`),
    KEY `idx_created` (`created_at`),
    KEY `idx_tenant_status` (`tenant_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.0 通知发送日志';

-- ============== sys_config 12 项 ==============
-- 铁律 04/05：三通道开关 + 服务商密钥 + 重试 + 限流 全部走 sys_config
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('notify.sms.enabled',                '0',                                'bool',   '短信通知开关',               'notify', 'v0.4.0 1=启用短信通道；0=关闭'),
('notify.sms.provider',               'none',                             'string', '短信服务商',                 'notify', 'v0.4.0 none/aliyun/tencent，none=不发送'),
('notify.sms.access_key_id',          '',                                 'string', '短信 AccessKeyId',           'notify', 'v0.4.0 服务商 AccessKeyId（明文存）'),
('notify.sms.access_secret_enc',      '',                                 'string', '短信 AccessSecret（AES加密）','notify', 'v0.4.0 服务商 AccessSecret，AES-256-GCM 加密'),
('notify.sms.sign_name',              '',                                 'string', '短信签名',                   'notify', 'v0.4.0 短信签名，如 KeyAuth'),
('notify.email.enabled',              '0',                                'bool',   '邮件通知开关',               'notify', 'v0.4.0 1=启用邮件通道；0=关闭'),
('notify.email.smtp_host',            '',                                 'string', 'SMTP 服务器',                'notify', 'v0.4.0 SMTP 服务器地址，如 smtp.qq.com'),
('notify.email.smtp_port',            '465',                              'number', 'SMTP 端口',                  'notify', 'v0.4.0 SSL=465 / STARTTLS=587'),
('notify.email.smtp_username',        '',                                 'string', 'SMTP 用户名',                'notify', 'v0.4.0 SMTP 登录用户名'),
('notify.email.smtp_password_enc',    '',                                 'string', 'SMTP 密码（AES加密）',       'notify', 'v0.4.0 SMTP 登录密码，AES-256-GCM 加密'),
('notify.email.from_address',         '',                                 'string', '发件人地址',                 'notify', 'v0.4.0 发件人邮箱，如 noreply@example.com'),
('notify.email.from_name',            'KeyAuth SaaS',                     'string', '发件人名称',                 'notify', 'v0.4.0 发件人显示名称'),
('notify.inapp.enabled',              '1',                                'bool',   '站内信开关',                 'notify', 'v0.4.0 1=启用站内信通道；0=关闭'),
('notify.retry.times',                '3',                                'number', '失败重试次数',               'notify', 'v0.4.0 单条通知失败后的最大重试次数'),
('notify.retry.interval_seconds',     '60',                               'number', '重试间隔（秒）',             'notify', 'v0.4.0 重试之间的等待秒数'),
('notify.rate_limit.per_minute',      '60',                               'number', '单租户每分钟限流',           'notify', 'v0.4.0 单租户每分钟最多发送通知数') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);

-- ============== 预置 4 个平台通用模板 ==============
INSERT INTO `notify_template` (`code`, `name`, `channel`, `subject`, `content`, `variables`, `tenant_id`, `status`, `remark`) VALUES
('verify_code', '验证码', 'sms', '', '您的验证码为 {{code}}，{{ttl}} 分钟内有效，请勿泄露。', '["code","ttl"]', 0, 'enabled', 'v0.4.0 通用验证码短信'),
('verify_code_email', '验证码邮件', 'email', '【{{app_name}}】验证码', '您正在 {{app_name}} 进行身份验证，验证码为：<b>{{code}}</b>，{{ttl}} 分钟内有效。', '["app_name","code","ttl"]', 0, 'enabled', 'v0.4.0 通用验证码邮件'),
('order_paid', '订单支付成功', 'inapp', '', '您的订单 {{order_no}} 已支付成功，卡密 {{card_key}} 已发放。', '["order_no","card_key"]', 0, 'enabled', 'v0.4.0 订单支付成功站内信'),
('agent_commission', '代理佣金到账', 'inapp', '', '您有 {{amount}} 元佣金到账，来自订单 {{order_no}}。', '["amount","order_no"]', 0, 'enabled', 'v0.4.0 代理佣金到账站内信');
