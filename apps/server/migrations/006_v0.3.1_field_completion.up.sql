-- ============================================================
-- KeyAuth SaaS v0.3.1 字段补全迁移
-- 说明：补全 v0.3.0 后端实现中标注「待核实」的所有缺失字段
--       + 新增登录失败日志表 + 新增 refresh token 设备表
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ============================================================
-- 1. 平台层字段补全
-- ============================================================

-- sys_tenant 增加 remark（备注）
ALTER TABLE `sys_tenant` ADD COLUMN `remark` VARCHAR(255) NULL COMMENT '备注' AFTER `last_login_ip`;

-- sys_package 增加 description（套餐描述）
ALTER TABLE `sys_package` ADD COLUMN `description` VARCHAR(255) NULL COMMENT '套餐描述' AFTER `name`;

-- ============================================================
-- 2. 应用层字段补全
-- ============================================================

-- app_cloud_var 增加 read_only（只读标记）
ALTER TABLE `app_cloud_var` ADD COLUMN `read_only` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否只读' AFTER `var_type`;

-- app_version 增加 channel（发布渠道）
ALTER TABLE `app_version` ADD COLUMN `channel` VARCHAR(32) NOT NULL DEFAULT 'stable' COMMENT 'stable/beta/dev' AFTER `version`;

-- ============================================================
-- 3. 代理层字段补全
-- ============================================================

-- agent 增加 commission_mode / inviter_id / totp_secret / email / last_login_ip
ALTER TABLE `agent` ADD COLUMN `commission_mode` VARCHAR(32) NOT NULL DEFAULT 'percentage' COMMENT 'percentage/diff' AFTER `commission_rate`;
ALTER TABLE `agent` ADD COLUMN `inviter_id` BIGINT UNSIGNED NULL COMMENT '邀请人 agent_id' AFTER `commission_mode`;
ALTER TABLE `agent` ADD COLUMN `email` VARCHAR(128) NULL AFTER `phone`;
ALTER TABLE `agent` ADD COLUMN `totp_secret` VARCHAR(64) NULL COMMENT '2FA 密钥(AES加密)' AFTER `email`;
ALTER TABLE `agent` ADD COLUMN `last_login_ip` VARCHAR(45) NULL AFTER `last_login_at`;

-- agent_invite_code 增加 used_by_agent_id（已使用该码注册的代理 id，max_uses=1 时填）
ALTER TABLE `agent_invite_code` ADD COLUMN `used_by_agent_id` BIGINT UNSIGNED NULL COMMENT '已使用代理 id' AFTER `used_count`;

-- ============================================================
-- 4. 公告层字段补全
-- ============================================================

-- notice 增加 sort（排序权重，越大越靠前）
ALTER TABLE `notice` ADD COLUMN `sort` INT NOT NULL DEFAULT 0 COMMENT '排序权重' AFTER `view_count`;

-- ============================================================
-- 5. 安全 / 日志层字段补全
-- ============================================================

-- sec_ip_blacklist 增加 created_by / created_by_type
ALTER TABLE `sec_ip_blacklist` ADD COLUMN `created_by` BIGINT UNSIGNED NULL COMMENT '创建者 id' AFTER `source`;
ALTER TABLE `sec_ip_blacklist` ADD COLUMN `created_by_type` VARCHAR(32) NULL COMMENT 'admin/tenant/auto' AFTER `created_by`;

-- log_operation 增加 username / user_agent / status
ALTER TABLE `log_operation` ADD COLUMN `username` VARCHAR(64) NULL COMMENT '操作者用户名冗余' AFTER `operator_id`;
ALTER TABLE `log_operation` ADD COLUMN `user_agent` VARCHAR(255) NULL AFTER `operator_ip`;
ALTER TABLE `log_operation` ADD COLUMN `status` VARCHAR(32) NOT NULL DEFAULT 'success' COMMENT 'success/fail' AFTER `action`;

-- ============================================================
-- 6. 新增：登录失败日志表（用于安全中心统计）
-- ============================================================
CREATE TABLE IF NOT EXISTS `log_login_failed` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_type` VARCHAR(32) NOT NULL COMMENT 'admin/tenant/agent',
  `username` VARCHAR(64) NOT NULL,
  `client_ip` VARCHAR(45) NOT NULL,
  `reason` VARCHAR(64) NOT NULL COMMENT 'wrong_password/disabled/locked/unknown',
  `user_agent` VARCHAR(255) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ip_created` (`client_ip`, `created_at`),
  KEY `idx_user_created` (`user_type`, `username`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='登录失败日志表';

-- ============================================================
-- 7. 新增：refresh token 设备会话表（用于 ListLoginDevices / KickDevice）
-- ============================================================
CREATE TABLE IF NOT EXISTS `refresh_token_device` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_role` VARCHAR(32) NOT NULL COMMENT 'admin/tenant/agent',
  `user_id` BIGINT UNSIGNED NOT NULL,
  `refresh_jti` VARCHAR(64) NOT NULL COMMENT 'refresh token JWT id',
  `device_name` VARCHAR(128) NULL COMMENT 'OS / Browser 简化名',
  `device_type` VARCHAR(32) NULL COMMENT 'pc/mobile/tablet',
  `client_ip` VARCHAR(45) NULL,
  `user_agent` VARCHAR(512) NULL,
  `last_active_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `expires_at` DATETIME NOT NULL COMMENT 'refresh token 过期时间',
  `revoked` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已撤销',
  `revoked_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_refresh_jti` (`refresh_jti`),
  KEY `idx_user_role_active` (`user_role`, `user_id`, `revoked`),
  KEY `idx_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='refresh token 设备会话表';

SET FOREIGN_KEY_CHECKS = 1;
