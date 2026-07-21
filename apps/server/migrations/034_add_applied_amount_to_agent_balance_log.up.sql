-- 充值审核金额审计字段：agent_balance_log 新增 applied_amount 列
-- 问题：原 AgentBalanceLog.Amount 字段在审核通过时被 actualAmount 覆盖，丢失原始申请金额，
--       无法对账"用户申请金额" vs "实际到账金额"。
-- 修复：新增 applied_amount 字段，AgentRecharge 写入时填充申请金额；
--       TenantApproveRecharge 仅更新 amount=actualAmount，applied_amount 保持申请值不变。
--
-- 配套代码：
--   apps/server/internal/model/model.go                       AgentBalanceLog 新增 AppliedAmount 字段
--   apps/server/internal/handler/agent_business.go            AgentRecharge 写入 AppliedAmount = req.Amount
--   apps/server/internal/handler/tenant_finance.go            TenantApproveRecharge 仅更新 amount，不动 applied_amount

ALTER TABLE `agent_balance_log`
  ADD COLUMN `applied_amount` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '原始申请金额（充值审计用，审核调整 amount 时此字段保留申请值）' AFTER `amount`;

-- 历史数据回填：将现有 recharge 类型流水的 applied_amount 设为 amount（无原始申请值时退化为 amount）
UPDATE `agent_balance_log` SET `applied_amount` = `amount` WHERE `applied_amount` = 0 AND `type` = 'recharge';
