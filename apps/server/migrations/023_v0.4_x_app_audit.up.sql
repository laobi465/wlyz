-- v0.4.x S-04 应用审核（应用上架审核、违规下架）
-- 1. app 表新增 4 个审核字段：audit_status / audit_remark / audited_at / audited_by
-- 2. sys_config 新增 1 项 app.audit.* 配置（审核总开关）
--
-- 严格遵循铁律 04/05：
--   04 - 无硬编码：审核开关走 sys_config，可后台实时调整
--   05 - 配置走后端：app.audit.enabled=0 时新应用直接 audit_status=approved
--
-- 配套代码：
--   apps/server/internal/handler/app.go TenantCreateApp 根据 sys_config 设置 audit_status
--   apps/server/internal/handler/admin_business.go AdminListPendingApps/AdminAuditApp/AdminOfflineApp/AdminOnlineApp
--   apps/server/internal/handler/client.go 校验 app.audit_status='approved'

-- ============== 1. app 表新增审核字段 ==============
ALTER TABLE `app` ADD COLUMN `audit_status` VARCHAR(16) NOT NULL DEFAULT 'approved' COMMENT '审核状态：approved/pending/rejected';
ALTER TABLE `app` ADD COLUMN `audit_remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '审核备注';
ALTER TABLE `app` ADD COLUMN `audited_at` DATETIME NULL COMMENT '审核时间';
ALTER TABLE `app` ADD COLUMN `audited_by` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '审核人 admin ID';

-- 为按审核状态筛选提供索引
CREATE INDEX `idx_app_audit_status` ON `app` (`audit_status`);

-- ============== 2. 新增 sys_config 配置 ==============
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('app.audit.enabled', '0', 'bool', '应用审核总开关', 'app',
 'v0.4.x S-04：1=开启应用审核（创建时 audit_status=pending，需 admin 审核后才能用于客户端验证）；0=关闭（创建时 audit_status=approved）')
ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
