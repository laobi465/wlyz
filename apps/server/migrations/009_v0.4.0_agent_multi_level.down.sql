-- ============================================================
-- KeyAuth SaaS v0.4.0 多级代理迁移（回滚）
-- 回滚后：agent 表恢复无 parent_id/level；agent_invite_code 表恢复无 creator_type/creator_agent_id
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

ALTER TABLE `agent`
  DROP INDEX `idx_agent_parent`,
  DROP INDEX `idx_agent_level`,
  DROP COLUMN `parent_id`,
  DROP COLUMN `level`;

ALTER TABLE `agent_invite_code`
  DROP INDEX `idx_invite_code_creator_agent`,
  DROP COLUMN `creator_agent_id`,
  DROP COLUMN `creator_type`;

DELETE FROM `sys_config` WHERE `config_key` IN (
  'agent.commission.cross_level_2_rate',
  'agent.commission.cross_level_3_rate',
  'agent.commission.max_level',
  'agent.invite_code.agent_can_create'
);

SET FOREIGN_KEY_CHECKS = 1;
