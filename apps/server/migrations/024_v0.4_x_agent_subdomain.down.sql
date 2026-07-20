-- v0.4.x 代理子域名绑定 - 回滚
-- 1. 删除 sys_config 中 2 项配置
-- 2. 删除 agent 表 subdomain_status 字段 + 索引

DELETE FROM `sys_config` WHERE `config_key` IN ('agent.subdomain.enabled', 'agent.subdomain.pattern');

DROP INDEX `idx_agent_subdomain_status` ON `agent`;
ALTER TABLE `agent` DROP COLUMN `subdomain_status`;
