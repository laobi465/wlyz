-- v0.4.x 代理子域名绑定（残留项 1）
-- 1. agent 表新增 subdomain_status 字段（pending/approved/rejected/none）
-- 2. sys_config 新增 2 项：
--    agent.subdomain.enabled  - 子域名功能总开关（默认 0 关闭）
--    agent.subdomain.pattern  - 子域名格式正则（默认 ^[a-z0-9-]{3,32}$）
--
-- 严格遵循铁律 04/05：
--   04 - 无硬编码：所有可变参数走 sys_config，可后台实时调整
--   05 - 配置走后端：subdomain.enabled=0 时所有代理端点拒绝申请
--
-- 配套代码：
--   apps/server/internal/handler/agent_business.go AgentSubdomainStatus/ApplySubdomain/UnbindSubdomain
--   apps/server/internal/handler/admin_business.go AdminListSubdomains/AdminApproveSubdomain/AdminRejectSubdomain
--   apps/server/internal/handler/agent_business.go AgentPortalQrCode
--   apps/server/internal/handler/public.go PublicPortal/PublicPortalOrder

-- ============== 1. agent 表新增 subdomain_status 字段 ==============
ALTER TABLE `agent` ADD COLUMN `subdomain_status` VARCHAR(16) NOT NULL DEFAULT 'none' COMMENT '子域名绑定状态：none/pending/approved/rejected';

-- 为按状态筛选提供索引（admin 列表筛选用）
CREATE INDEX `idx_agent_subdomain_status` ON `agent` (`subdomain_status`);

-- ============== 2. 新增 sys_config 配置 ==============
INSERT IGNORE INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('agent.subdomain.enabled', '0', 'bool', '代理子域名绑定开关', 'agent',
 'v0.4.x 残留项 1：代理子域名绑定总开关；1=启用，0=关闭（默认关闭，需平台运维配置泛域名解析后再开启）'),
('agent.subdomain.pattern', '^[a-z0-9-]{3,32}$', 'string', '代理子域名格式正则', 'agent',
 'v0.4.x 残留项 1：子域名格式校验正则；默认仅允许小写字母/数字/连字符，长度 3-32');
