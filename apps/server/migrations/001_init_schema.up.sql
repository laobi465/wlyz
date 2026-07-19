-- ============================================================
-- KeyAuth SaaS 数据库初始化脚本
-- 版本：0.2.0
-- 说明：建表 + sys_config seed 数据 + 默认套餐 + 默认超管
-- ============================================================

-- 数据库字符集
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ============================================================
-- 1. 平台层
-- ============================================================

-- 系统配置表（铁律 05 核心，所有可变参数走此表）
CREATE TABLE IF NOT EXISTS `sys_config` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `config_key` VARCHAR(128) NOT NULL COMMENT '配置键',
  `config_value` TEXT NULL COMMENT '配置值',
  `config_type` ENUM('string','number','bool','json') NOT NULL DEFAULT 'string',
  `config_name` VARCHAR(128) NULL COMMENT '后台显示名称',
  `config_group` VARCHAR(64) NOT NULL DEFAULT 'system' COMMENT '分组',
  `remark` VARCHAR(255) NULL COMMENT '说明',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_key` (`config_key`),
  KEY `idx_config_group` (`config_group`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统配置表';

-- 平台超管表
CREATE TABLE IF NOT EXISTS `sys_admin` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL COMMENT 'bcrypt cost=12',
  `email` VARCHAR(128) NULL,
  `phone` VARCHAR(32) NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `totp_secret` VARCHAR(64) NULL COMMENT '2FA 密钥(AES加密)',
  `last_login_at` DATETIME NULL,
  `last_login_ip` VARCHAR(45) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台超管表';

-- 平台套餐表
CREATE TABLE IF NOT EXISTS `sys_package` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL,
  `monthly_price` DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  `yearly_price` DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  `max_apps` INT NOT NULL DEFAULT 1,
  `max_cards` INT NOT NULL DEFAULT 1000,
  `max_agents` INT NOT NULL DEFAULT 0,
  `allow_custom_pay` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许开发者自定义易支付',
  `custom_pay_fee` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '开通自定义支付的附加月费',
  `platform_commission_rate` DECIMAL(5,2) NOT NULL DEFAULT 5.00 COMMENT '使用平台总支付时平台抽成(%)',
  `features` JSON NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台套餐表';

-- 租户表
CREATE TABLE IF NOT EXISTS `sys_tenant` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_code` VARCHAR(32) NOT NULL,
  `username` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `email` VARCHAR(128) NULL,
  `phone` VARCHAR(32) NULL,
  `company` VARCHAR(128) NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/active/suspended/deleted',
  `package_id` BIGINT UNSIGNED NOT NULL,
  `expires_at` DATETIME NULL,
  `totp_secret` VARCHAR(64) NULL,
  `last_login_at` DATETIME NULL,
  `last_login_ip` VARCHAR(45) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_code` (`tenant_code`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_status` (`status`),
  KEY `idx_package` (`package_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='租户表';

-- 租户自有易支付配置
CREATE TABLE IF NOT EXISTS `tenant_pay_config` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `channel` VARCHAR(32) NOT NULL DEFAULT 'epay',
  `enabled` TINYINT(1) NOT NULL DEFAULT 0,
  `gateway_url` VARCHAR(255) NULL,
  `pid` VARCHAR(64) NULL,
  `key_encrypted` TEXT NULL COMMENT '商户密钥(AES-256-GCM加密)',
  `methods` JSON NULL,
  `notify_path` VARCHAR(255) NULL,
  `return_path` VARCHAR(255) NULL,
  `last_test_at` DATETIME NULL,
  `last_test_result` VARCHAR(32) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_channel` (`tenant_id`,`channel`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='租户支付配置表';

-- ============================================================
-- 2. 应用层
-- ============================================================

CREATE TABLE IF NOT EXISTS `app` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `app_key` VARCHAR(64) NOT NULL,
  `app_secret` VARCHAR(255) NOT NULL COMMENT 'AES加密',
  `sign_secret` VARCHAR(255) NOT NULL COMMENT 'AES加密',
  `sign_secret_prev` VARCHAR(255) NULL COMMENT '旧签名密钥(轮换期保留)',
  `name` VARCHAR(128) NOT NULL,
  `description` TEXT NULL,
  `icon` VARCHAR(255) NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `max_devices` INT NOT NULL DEFAULT 1 COMMENT '一机一卡=1',
  `heartbeat_interval` INT NOT NULL DEFAULT 60,
  `heartbeat_timeout` INT NOT NULL DEFAULT 180,
  `offline_grace` INT NOT NULL DEFAULT 86400,
  `unbind_deduct_seconds` INT NOT NULL DEFAULT 86400,
  `agent_commission_mode` VARCHAR(32) NOT NULL DEFAULT 'diff' COMMENT 'percentage/diff',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_key` (`app_key`),
  KEY `idx_tenant` (`tenant_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用表';

CREATE TABLE IF NOT EXISTS `app_card_type` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `app_id` BIGINT UNSIGNED NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `type` VARCHAR(32) NOT NULL COMMENT 'duration/count/permanent/trial/feature',
  `duration_seconds` BIGINT NOT NULL DEFAULT 0 COMMENT '永久卡=-1',
  `max_uses` INT NOT NULL DEFAULT 1,
  `price` DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  `agent_base_price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '代理底价',
  `features` JSON NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_tenant_app` (`tenant_id`,`app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='卡类表';

CREATE TABLE IF NOT EXISTS `app_card` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `app_id` BIGINT UNSIGNED NOT NULL,
  `card_type_id` BIGINT UNSIGNED NOT NULL,
  `card_key` VARCHAR(64) NOT NULL,
  `card_key_hash` CHAR(128) NOT NULL COMMENT 'SHA-512',
  `checksum` CHAR(8) NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'unused' COMMENT 'unused/active/expired/banned/disabled',
  `batch_no` VARCHAR(32) NULL,
  `prefix` VARCHAR(16) NULL,
  `group_tag` VARCHAR(64) NULL,
  `duration_seconds` BIGINT NOT NULL,
  `used_count` INT NOT NULL DEFAULT 0,
  `max_uses` INT NOT NULL DEFAULT 1,
  `bound_device_id` BIGINT UNSIGNED NULL,
  `activated_at` DATETIME NULL,
  `expires_at` DATETIME NULL,
  `last_verify_at` DATETIME NULL,
  `created_by` BIGINT UNSIGNED NOT NULL,
  `creator_type` VARCHAR(32) NOT NULL DEFAULT 'tenant',
  `order_id` BIGINT UNSIGNED NULL,
  `banned_at` DATETIME NULL,
  `banned_reason` VARCHAR(255) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_card_key_hash` (`card_key_hash`),
  KEY `idx_app_status` (`app_id`,`status`),
  KEY `idx_tenant` (`tenant_id`),
  KEY `idx_batch` (`batch_no`),
  KEY `idx_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='卡密表';

CREATE TABLE IF NOT EXISTS `app_device` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `app_id` BIGINT UNSIGNED NOT NULL,
  `card_id` BIGINT UNSIGNED NOT NULL,
  `hwid` CHAR(128) NOT NULL COMMENT '硬件指纹SHA-512',
  `hwid_raw` TEXT NULL,
  `device_name` VARCHAR(128) NULL,
  `device_type` VARCHAR(32) NULL,
  `ip_address` VARCHAR(45) NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/offline/banned/unbound',
  `last_heartbeat_at` DATETIME NULL,
  `first_bound_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `unbound_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_app_card_hwid` (`app_id`,`card_id`,`hwid`),
  KEY `idx_card` (`card_id`),
  KEY `idx_heartbeat` (`last_heartbeat_at`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='设备绑定表';

CREATE TABLE IF NOT EXISTS `app_order` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `app_id` BIGINT UNSIGNED NOT NULL,
  `card_type_id` BIGINT UNSIGNED NOT NULL,
  `order_no` VARCHAR(64) NOT NULL,
  `buyer_user_id` BIGINT UNSIGNED NULL,
  `buyer_contact` VARCHAR(128) NULL,
  `agent_id` BIGINT UNSIGNED NULL,
  `quantity` INT NOT NULL DEFAULT 1,
  `unit_price` DECIMAL(10,2) NOT NULL,
  `total_amount` DECIMAL(10,2) NOT NULL,
  `commission_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  `pay_channel` VARCHAR(32) NOT NULL,
  `pay_status` VARCHAR(32) NOT NULL DEFAULT 'pending',
  `pay_trade_no` VARCHAR(128) NULL,
  `paid_at` DATETIME NULL,
  `card_ids` JSON NULL,
  `refund_amount` DECIMAL(10,2) NULL,
  `refunded_at` DATETIME NULL,
  `client_ip` VARCHAR(45) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_tenant_status` (`tenant_id`,`pay_status`),
  KEY `idx_agent` (`agent_id`),
  KEY `idx_paid_at` (`paid_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';

CREATE TABLE IF NOT EXISTS `app_cloud_var` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `app_id` BIGINT UNSIGNED NOT NULL,
  `var_key` VARCHAR(128) NOT NULL,
  `var_value` TEXT NULL,
  `var_type` VARCHAR(32) NOT NULL DEFAULT 'string',
  `remark` VARCHAR(255) NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_app` (`app_id`),
  KEY `idx_tenant_app_key` (`tenant_id`,`app_id`,`var_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='云变量表';

CREATE TABLE IF NOT EXISTS `app_version` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `app_id` BIGINT UNSIGNED NOT NULL,
  `version` VARCHAR(32) NOT NULL,
  `min_version` VARCHAR(32) NOT NULL,
  `download_url` VARCHAR(255) NULL,
  `backup_url` VARCHAR(255) NULL,
  `force_update` TINYINT(1) NOT NULL DEFAULT 0,
  `update_content` TEXT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_app` (`app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用版本表';

-- ============================================================
-- 3. 代理层
-- ============================================================

CREATE TABLE IF NOT EXISTS `agent` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `username` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `real_name` VARCHAR(64) NULL,
  `phone` VARCHAR(32) NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `balance` DECIMAL(12,2) NOT NULL DEFAULT 0.00,
  `commission_rate` DECIMAL(5,2) NOT NULL DEFAULT 10.00,
  `subdomain` VARCHAR(64) NULL,
  `last_login_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_username` (`tenant_id`,`username`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理商表';

CREATE TABLE IF NOT EXISTS `agent_invite_code` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `code` VARCHAR(32) NOT NULL,
  `max_uses` INT NOT NULL DEFAULT 1,
  `used_count` INT NOT NULL DEFAULT 0,
  `valid_days` INT NOT NULL DEFAULT 30,
  `expires_at` DATETIME NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `allowed_apps` JSON NULL,
  `default_commission_rate` DECIMAL(5,2) NOT NULL DEFAULT 10.00,
  `created_by` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_tenant_status` (`tenant_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理邀请码表';

CREATE TABLE IF NOT EXISTS `agent_balance_log` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `agent_id` BIGINT UNSIGNED NOT NULL,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `type` VARCHAR(32) NOT NULL COMMENT 'recharge/deduct/commission/withdraw/adjust',
  `amount` DECIMAL(12,2) NOT NULL,
  `balance_after` DECIMAL(12,2) NOT NULL,
  `related_order_id` BIGINT UNSIGNED NULL,
  `related_card_ids` JSON NULL,
  `pay_method` VARCHAR(32) NULL,
  `pay_voucher` VARCHAR(255) NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending',
  `remark` VARCHAR(255) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_agent` (`agent_id`),
  KEY `idx_tenant` (`tenant_id`),
  KEY `idx_type_status` (`type`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理余额流水表';

CREATE TABLE IF NOT EXISTS `agent_withdraw` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `agent_id` BIGINT UNSIGNED NOT NULL,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `amount` DECIMAL(12,2) NOT NULL,
  `pay_method` VARCHAR(32) NOT NULL,
  `pay_account` VARCHAR(128) NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending',
  `audit_remark` VARCHAR(255) NULL,
  `pay_trade_no` VARCHAR(128) NULL,
  `paid_at` DATETIME NULL,
  `audited_by` BIGINT UNSIGNED NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_agent_status` (`agent_id`,`status`),
  KEY `idx_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理提现表';

CREATE TABLE IF NOT EXISTS `agent_commission` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `agent_id` BIGINT UNSIGNED NOT NULL,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `order_id` BIGINT UNSIGNED NOT NULL,
  `card_id` BIGINT UNSIGNED NULL,
  `sale_amount` DECIMAL(12,2) NOT NULL,
  `commission_rate` DECIMAL(5,2) NOT NULL,
  `amount` DECIMAL(12,2) NOT NULL,
  `settle_status` VARCHAR(32) NOT NULL DEFAULT 'pending',
  `settled_at` DATETIME NULL,
  `settle_method` VARCHAR(32) NULL,
  `settle_remark` VARCHAR(255) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_agent` (`agent_id`),
  KEY `idx_tenant` (`tenant_id`),
  KEY `idx_order` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理佣金表';

CREATE TABLE IF NOT EXISTS `agent_registration_order` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_no` VARCHAR(64) NOT NULL,
  `invite_code_id` BIGINT UNSIGNED NOT NULL,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `agent_id` BIGINT UNSIGNED NULL,
  `username` VARCHAR(64) NOT NULL,
  `phone` VARCHAR(32) NULL,
  `amount` DECIMAL(10,2) NOT NULL,
  `pay_channel` VARCHAR(32) NOT NULL,
  `pay_status` VARCHAR(32) NOT NULL DEFAULT 'pending',
  `pay_trade_no` VARCHAR(128) NULL,
  `paid_at` DATETIME NULL,
  `client_ip` VARCHAR(45) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_invite_code` (`invite_code_id`),
  KEY `idx_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理注册订单表';

-- ============================================================
-- 4. 公告层
-- ============================================================

CREATE TABLE IF NOT EXISTS `notice` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `type` VARCHAR(32) NOT NULL COMMENT 'platform/developer/app/agent_notify',
  `tenant_id` BIGINT UNSIGNED NULL,
  `app_id` BIGINT UNSIGNED NULL,
  `title` VARCHAR(255) NOT NULL,
  `content` TEXT NOT NULL,
  `is_pinned` TINYINT(1) NOT NULL DEFAULT 0,
  `is_popup` TINYINT(1) NOT NULL DEFAULT 0,
  `show_badge` TINYINT(1) NOT NULL DEFAULT 1,
  `start_at` DATETIME NOT NULL,
  `end_at` DATETIME NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'draft',
  `view_count` INT NOT NULL DEFAULT 0,
  `created_by` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_type_status` (`type`,`status`),
  KEY `idx_tenant` (`tenant_id`),
  KEY `idx_app` (`app_id`),
  KEY `idx_time` (`start_at`,`end_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='统一公告表';

CREATE TABLE IF NOT EXISTS `notice_target` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `notice_id` BIGINT UNSIGNED NOT NULL,
  `target_type` VARCHAR(32) NOT NULL,
  `target_id` BIGINT UNSIGNED NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_notice` (`notice_id`),
  KEY `idx_target` (`target_type`,`target_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='公告精准投递表';

CREATE TABLE IF NOT EXISTS `notice_read` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `notice_id` BIGINT UNSIGNED NOT NULL,
  `user_type` VARCHAR(32) NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `read_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notice_user` (`notice_id`,`user_type`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='公告已读记录表';

-- ============================================================
-- 5. 安全 / 日志
-- ============================================================

CREATE TABLE IF NOT EXISTS `sec_ip_blacklist` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `ip` VARCHAR(45) NOT NULL,
  `reason` VARCHAR(255) NULL,
  `source` VARCHAR(32) NOT NULL DEFAULT 'manual',
  `expires_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ip` (`ip`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='IP 黑名单';

CREATE TABLE IF NOT EXISTS `log_verify` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `app_id` BIGINT UNSIGNED NOT NULL,
  `card_id` BIGINT UNSIGNED NULL,
  `device_id` BIGINT UNSIGNED NULL,
  `action` VARCHAR(32) NOT NULL,
  `result` VARCHAR(32) NOT NULL,
  `client_ip` VARCHAR(45) NULL,
  `user_agent` VARCHAR(255) NULL,
  `extra` JSON NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`, `created_at`),
  KEY `idx_app_action` (`app_id`,`action`),
  KEY `idx_card` (`card_id`),
  KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='验证日志表'
PARTITION BY RANGE (TO_DAYS(created_at)) (
  PARTITION p202607 VALUES LESS THAN (TO_DAYS('2026-08-01')),
  PARTITION p202608 VALUES LESS THAN (TO_DAYS('2026-09-01')),
  PARTITION pmax VALUES LESS THAN MAXVALUE
);

CREATE TABLE IF NOT EXISTS `log_operation` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `operator_type` VARCHAR(32) NOT NULL,
  `operator_id` BIGINT UNSIGNED NOT NULL,
  `operator_ip` VARCHAR(45) NULL,
  `module` VARCHAR(64) NULL,
  `action` VARCHAR(64) NOT NULL,
  `target_type` VARCHAR(64) NULL,
  `target_id` BIGINT UNSIGNED NULL,
  `detail` JSON NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_operator` (`operator_type`,`operator_id`),
  KEY `idx_module` (`module`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作日志表';

SET FOREIGN_KEY_CHECKS = 1;
