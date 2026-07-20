-- ============================================================
-- KeyAuth SaaS v0.4.0 多级代理（二级 / 三级 + 跨级佣金）迁移
-- 说明：
--   1. agent 表新增 parent_id（上级代理 ID，0=一级代理）+ level（1/2/3）
--   2. agent_invite_code 表新增 creator_type（tenant/agent）+ creator_agent_id（代理创建时填）
--      用于支持代理创建下级邀请码
--   3. sys_config 新增 4 项多级代理配置：
--      - agent.commission.cross_level_2_rate：二级代理上线（一级代理）的跨级佣金比例
--      - agent.commission.cross_level_3_rate：三级代理上线（一级代理）的跨级佣金比例
--      - agent.commission.max_level：最大代理层级（1/2/3）
--      - agent.invite_code.agent_can_create：是否允许代理创建下级邀请码
--   4. 兼容策略：老代理 parent_id=0 / level=1；老邀请码 creator_type='tenant' / creator_agent_id=0
-- 严格遵循铁律 04/05：不引入硬编码（所有比例走 sys_config）；铁律 06：向前兼容（默认值不破坏老数据）
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ============================================================
-- 1. agent 表加 parent_id + level
-- ============================================================
ALTER TABLE `agent`
  ADD COLUMN `parent_id` BIGINT NOT NULL DEFAULT 0 COMMENT '上级代理 ID（0=一级代理，否则指向父级 agent.id）' AFTER `inviter_id`,
  ADD COLUMN `level` TINYINT NOT NULL DEFAULT 1 COMMENT '代理层级（1=一级 / 2=二级 / 3=三级，最大 3）' AFTER `parent_id`,
  ADD INDEX `idx_agent_parent` (`parent_id`),
  ADD INDEX `idx_agent_level` (`level`);

-- ============================================================
-- 2. agent_invite_code 表加 creator_type + creator_agent_id
-- ============================================================
ALTER TABLE `agent_invite_code`
  ADD COLUMN `creator_type` VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT '创建者类型（tenant=开发者 / agent=代理）' AFTER `created_by`,
  ADD COLUMN `creator_agent_id` BIGINT NOT NULL DEFAULT 0 COMMENT '创建者代理 ID（creator_type=agent 时填，否则 0）' AFTER `creator_type`,
  ADD INDEX `idx_invite_code_creator_agent` (`creator_agent_id`);

-- ============================================================
-- 3. sys_config 新增多级代理配置项
-- ============================================================
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('agent.commission.cross_level_2_rate', '50.00', 'number', '二级代理跨级佣金比例(%)', 'agent', '二级代理产生佣金时，其上级（一级代理）获得的跨级佣金比例（占二级代理佣金的百分比，0-100）'),
('agent.commission.cross_level_3_rate', '20.00', 'number', '三级代理跨级佣金比例(%)', 'agent', '三级代理产生佣金时，其上级的一级代理（祖父级）获得的跨级佣金比例（占三级代理佣金的百分比，0-100）'),
('agent.commission.max_level', '3', 'number', '代理最大层级', 'agent', '允许的代理层级（1=仅一级 / 2=支持二级 / 3=支持三级），超出则禁止注册下级'),
('agent.invite_code.agent_can_create', '1', 'bool', '允许代理创建下级邀请码', 'agent', '总开关，关闭后代理无法生成下级邀请码（即使 max_level > 1）');

SET FOREIGN_KEY_CHECKS = 1;
