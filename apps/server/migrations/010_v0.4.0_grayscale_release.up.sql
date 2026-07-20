-- ============================================================
-- KeyAuth SaaS v0.4.0 灰度发布（应用版本灰度推送 + 规则配置）迁移
-- 说明：
--   1. app_version 表新增 5 个灰度字段：
--      - release_strategy：发布策略（full=全量 / grayscale=灰度 / canary=金丝雀）
--      - grayscale_rate：灰度比例（0.00-100.00，grayscale 策略下生效）
--      - grayscale_platforms：灰度平台限制（逗号分隔 windows/macos/linux/android/ios，空=不限）
--      - grayscale_regions：灰度地区限制（逗号分隔省/州代码，空=不限）
--      - grayscale_channels：灰度渠道限制（逗号分隔 stable/beta/dev，空=不限）
--   2. 补 app_version.tenant_id 字段（001 建表遗漏，仅 model 有，此迁移补齐 DB schema）
--   3. 新增复合索引 idx_app_status_strategy 支持 ClientVersion 查询
--   4. sys_config 新增 3 项灰度配置：
--      - app.version.grayscale.enabled：总开关
--      - app.version.grayscale.default_rate：默认灰度比例
--      - app.version.grayscale.hash_salt：用户哈希盐值（保证灰度分桶稳定）
--   5. 兼容策略：老版本 release_strategy='full' / grayscale_rate=0，行为等同原全量发布
-- 严格遵循铁律 04/05：不引入硬编码（所有可调参数走 sys_config）；铁律 06：向前兼容（默认值不破坏老数据）
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ============================================================
-- 1. app_version 表补 tenant_id（修复 001 建表遗漏，model 已有该字段）
-- ============================================================
-- 注意：使用 information_schema 检测避免重复执行报错（兼容已通过 ORM 写入数据的环境）
SET @col_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_version' AND COLUMN_NAME = 'tenant_id');
SET @sql := IF(@col_exists = 0,
  'ALTER TABLE `app_version` ADD COLUMN `tenant_id` BIGINT NOT NULL DEFAULT 0 COMMENT ''租户ID（多租户隔离）'' AFTER `id`',
  'SELECT ''app_version.tenant_id 已存在，跳过'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ============================================================
-- 2. app_version 表加 5 个灰度字段 + 复合索引
-- ============================================================
ALTER TABLE `app_version`
  ADD COLUMN `release_strategy` VARCHAR(32) NOT NULL DEFAULT 'full' COMMENT '发布策略（full=全量 / grayscale=灰度 / canary=金丝雀）' AFTER `channel`,
  ADD COLUMN `grayscale_rate` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT '灰度比例（0.00-100.00，grayscale 策略下生效）' AFTER `release_strategy`,
  ADD COLUMN `grayscale_platforms` VARCHAR(200) NOT NULL DEFAULT '' COMMENT '灰度平台限制（逗号分隔 windows/macos/linux/android/ios，空=不限）' AFTER `grayscale_rate`,
  ADD COLUMN `grayscale_regions` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '灰度地区限制（逗号分隔省/州代码，空=不限）' AFTER `grayscale_platforms`,
  ADD COLUMN `grayscale_channels` VARCHAR(200) NOT NULL DEFAULT '' COMMENT '灰度渠道限制（逗号分隔 stable/beta/dev，空=不限）' AFTER `grayscale_regions`,
  ADD INDEX `idx_app_status_strategy` (`app_id`, `status`, `release_strategy`);

-- ============================================================
-- 3. sys_config 新增灰度发布配置项
-- ============================================================
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('app.version.grayscale.enabled', '1', 'bool', '启用灰度发布', 'app', '总开关。关闭后所有版本按全量发布处理（release_strategy=grayscale 也走全量）'),
('app.version.grayscale.default_rate', '10.00', 'number', '默认灰度比例(%)', 'app', '新建灰度版本时的默认比例（0-100），可在版本编辑时单独覆盖'),
('app.version.grayscale.hash_salt', 'keyauth-grayscale-v040', 'string', '灰度分桶哈希盐值', 'app', '用于保证同一客户端在多次版本检查中得到稳定的灰度命中结果。修改盐值会导致全量用户重新分桶');

SET FOREIGN_KEY_CHECKS = 1;
