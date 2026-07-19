-- ============================================================
-- KeyAuth SaaS v0.3.4 开发者财务闭环迁移
-- 说明：
--   1. sys_tenant 增加 balance / frozen_balance 字段（开发者可提现余额）
--   2. 新增 tenant_balance_log 开发者余额流水表
--   3. 新增 tenant_withdraw 开发者提现申请表
-- 严格遵循铁律 04/05：字段命名与 agent_balance_log / agent_withdraw 对称
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ============================================================
-- 1. sys_tenant 增加 balance / frozen_balance
-- ============================================================
ALTER TABLE `sys_tenant`
  ADD COLUMN `balance` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '可提现余额（已结算净额累计 - 已提现）' AFTER `last_login_ip`;

ALTER TABLE `sys_tenant`
  ADD COLUMN `frozen_balance` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '冻结余额（提现申请中）' AFTER `balance`;

-- ============================================================
-- 2. 新增 tenant_balance_log 开发者余额流水表
--    type: settle（结算入账）/ withdraw（提现扣款）/ refund（提现驳回退回）/ adjust（人工调整）
--    status: pending / settled / rejected
-- ============================================================
CREATE TABLE IF NOT EXISTS `tenant_balance_log` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '所属开发者',
  `type` VARCHAR(32) NOT NULL COMMENT 'settle/withdraw/refund/adjust',
  `amount` DECIMAL(12,2) NOT NULL COMMENT '金额（settle/refund 为正，withdraw 为负）',
  `balance_after` DECIMAL(12,2) NOT NULL COMMENT '操作后可提现余额快照',
  `related_order_id` BIGINT UNSIGNED NULL COMMENT '关联订单 id（settle 类型）',
  `related_settlement_id` BIGINT UNSIGNED NULL COMMENT '关联 platform_settlement id',
  `related_withdraw_id` BIGINT UNSIGNED NULL COMMENT '关联 tenant_withdraw id',
  `pay_method` VARCHAR(32) NULL COMMENT '结算方式 manual/alipay/wechat/bank',
  `settle_batch_no` VARCHAR(64) NULL COMMENT '结算批次号',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/settled/rejected',
  `remark` VARCHAR(255) NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_tenant_status` (`tenant_id`, `status`),
  KEY `idx_tenant_created` (`tenant_id`, `created_at`),
  KEY `idx_withdraw` (`related_withdraw_id`),
  KEY `idx_settlement` (`related_settlement_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='开发者余额流水表';

-- ============================================================
-- 3. 新增 tenant_withdraw 开发者提现申请表
--    status: pending（待审核）/ paid（已打款）/ rejected（已驳回）/ failed（打款失败）
-- ============================================================
CREATE TABLE IF NOT EXISTS `tenant_withdraw` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '申请提现的开发者',
  `amount` DECIMAL(12,2) NOT NULL COMMENT '提现金额',
  `pay_method` VARCHAR(32) NOT NULL COMMENT 'wechat/alipay/bank',
  `pay_account` VARCHAR(128) NOT NULL COMMENT '收款账号',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/paid/rejected/failed',
  `audit_remark` VARCHAR(255) NULL COMMENT '审核备注',
  `pay_trade_no` VARCHAR(128) NULL COMMENT '打款流水号',
  `paid_at` DATETIME NULL COMMENT '打款时间',
  `audited_by` BIGINT UNSIGNED NULL COMMENT '审核人 admin id',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_tenant_status` (`tenant_id`, `status`),
  KEY `idx_status_created` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='开发者提现申请表';

SET FOREIGN_KEY_CHECKS = 1;
