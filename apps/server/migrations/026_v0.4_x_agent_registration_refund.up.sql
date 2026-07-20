-- v0.4.x S-17 超管后台代理注册管理（退款 / 收入统计）
-- agent_registration_order 表新增 5 个退款字段
--
-- 严格遵循铁律 04/05：
--   04 - 无硬编码：退款金额从订单原值反算
--   05 - 配置走后端：无新增 sys_config（退款由 admin 手动触发，非自动规则）
--   06 - 反幻觉：退款事务内同步禁用代理账号，避免账号残留可用
--
-- 配套代码：
--   apps/server/internal/handler/admin_business.go
--     AdminListAgentRegistrations / AdminAgentRegistrationStats
--     AdminRefundAgentRegistration / AdminGetAgentRegistration

ALTER TABLE `agent_registration_order`
  ADD COLUMN `refund_status` VARCHAR(16) NOT NULL DEFAULT 'none' COMMENT '退款状态：none/refunded',
  ADD COLUMN `refund_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '退款金额',
  ADD COLUMN `refund_at`    DATETIME NULL COMMENT '退款时间',
  ADD COLUMN `refund_by`    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '退款操作人 admin ID',
  ADD COLUMN `refund_reason` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '退款原因';

CREATE INDEX `idx_agent_registration_refund_status` ON `agent_registration_order` (`refund_status`);
