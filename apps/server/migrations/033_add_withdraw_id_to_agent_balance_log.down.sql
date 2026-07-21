-- 回滚 P1-01/02 修复：移除 agent_balance_log.related_withdraw_id 字段

DROP INDEX `idx_withdraw` ON `agent_balance_log`;

ALTER TABLE `agent_balance_log` DROP COLUMN `related_withdraw_id`;
