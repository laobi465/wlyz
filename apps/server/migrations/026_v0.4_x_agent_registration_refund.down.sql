-- v0.4.x S-17 代理注册管理 - 回滚
-- 删除 agent_registration_order 表的 5 个退款字段

DROP INDEX `idx_agent_registration_refund_status` ON `agent_registration_order`;
ALTER TABLE `agent_registration_order`
  DROP COLUMN `refund_status`,
  DROP COLUMN `refund_amount`,
  DROP COLUMN `refund_at`,
  DROP COLUMN `refund_by`,
  DROP COLUMN `refund_reason`;
