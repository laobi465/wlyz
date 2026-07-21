-- 回滚 034：删除 agent_balance_log.applied_amount 字段
ALTER TABLE `agent_balance_log` DROP COLUMN `applied_amount`;
