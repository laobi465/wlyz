-- P1-01/02 安全审计修复：agent_balance_log 新增 related_withdraw_id 字段
-- 问题：原审核提现时按 agent_id + type + status + created_at>=? 时间窗口模糊匹配 balance_log，
--       同一代理多笔相同金额提现会错配，导致流水状态被错误更新。
-- 修复：新增 related_withdraw_id 字段，写入流水时填充，查询时精确匹配。
--
-- 配套代码：
--   apps/server/internal/handler/agent_business.go      AgentWithdraw 提现申请时填充 related_withdraw_id
--   apps/server/internal/handler/tenant_finance.go      TenantPayWithdraw / TenantRejectWithdraw 改为按 related_withdraw_id 精确匹配
--   apps/server/internal/model/model.go                 AgentBalanceLog 新增 RelatedWithdrawID 字段

ALTER TABLE `agent_balance_log`
  ADD COLUMN `related_withdraw_id` BIGINT UNSIGNED NULL COMMENT '关联 agent_withdraw id（withdraw 类型流水使用，精确匹配避免时间窗口错配）' AFTER `related_order_id`;

CREATE INDEX `idx_withdraw` ON `agent_balance_log` (`related_withdraw_id`);
