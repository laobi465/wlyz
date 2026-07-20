-- v0.4.x D-15 开发者安全设置（IP 黑名单、频率限制）
-- 1. tenant_security_config 表：每个租户独立的安全配置（IP 黑名单 + 频率限制）
-- 2. 中间件 TenantSecurityMiddleware 在 client 验证 API 链路中校验
--
-- 严格遵循铁律 04/05：
--   04 - 无硬编码：黑名单 IP 与频率阈值均存表，租户可后台实时调整
--   05 - 配置走后端：tenant_security_config 表与 sys_config 配合
--
-- 配套代码：
--   apps/server/internal/handler/tenant_business.go TenantGetSecurity/TenantUpdateSecurity
--   apps/server/internal/middleware/tenant_security.go TenantSecurityMiddleware

CREATE TABLE IF NOT EXISTS `tenant_security_config` (
    `id`                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id`                BIGINT UNSIGNED NOT NULL COMMENT '租户 ID',
    `ip_blacklist`             TEXT         NOT NULL COMMENT 'IP 黑名单 JSON 数组，如 ["1.2.3.4","10.0.0.0/8"]',
    `verify_rate_limit_per_min` INT         NOT NULL DEFAULT 0 COMMENT '客户端验证 API 限速（每分钟，0=不限）',
    `login_rate_limit_per_min`  INT         NOT NULL DEFAULT 0 COMMENT '客户端登录 API 限速（每分钟，0=不限）',
    `updated_at`               DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_tenant_id` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.x D-15 开发者安全配置';
