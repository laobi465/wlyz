-- v0.4.x 开发者自有支付附加月费订单
-- 1. tenant_monthly_fee_order 表：每个开发者月度服务费账单
-- 2. sys_config 新增 3 项 pay.tenant_monthly_fee.* 配置（开关 / 金额 / 免费天数）
--
-- 严格遵循铁律 04/05：
--   04 - 无硬编码：月费金额、免费天数从 sys_config 读取，可后台实时调整
--   05 - 配置走后端：3 项配置可由超管在「系统配置」编辑
--   06 - 反幻觉：订单号前缀 MFD 与 ORD/TOP/REG 区分，避免污染卡密/代理注册流程
--
-- 配套代码：
--   apps/server/internal/handler/admin_finance.go AdminListMonthlyFeeOrders/AdminMonthlyFeeStats/AdminMarkMonthlyFeePaid
--   apps/server/internal/handler/tenant_finance.go TenantGetMonthlyFeeCurrent/TenantPayMonthlyFee
--   apps/server/internal/handler/pay.go dispatchPaidOrder 新增 MFD 前缀分支

CREATE TABLE IF NOT EXISTS `tenant_monthly_fee_order` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id`     BIGINT UNSIGNED NOT NULL COMMENT '开发者 ID',
    `period_start`  DATETIME     NOT NULL COMMENT '账单周期开始时间',
    `period_end`    DATETIME     NOT NULL COMMENT '账单周期结束时间',
    `amount`        DECIMAL(10,2) NOT NULL COMMENT '账单金额（从 pay.tenant_monthly_fee.amount 读取）',
    `pay_status`    VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT 'pending/paid/closed',
    `pay_mode`      VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '支付方式：platform_epay/manual',
    `order_no`      VARCHAR(64)  NOT NULL COMMENT '订单号（MFD 前缀 + snowflake）',
    `paid_at`       DATETIME     NULL COMMENT '支付时间',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_tenant_id` (`tenant_id`),
    KEY `idx_pay_status` (`pay_status`),
    KEY `idx_period` (`period_start`, `period_end`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='v0.4.x 开发者月费订单';

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('pay.tenant_monthly_fee.enabled',   '0',     'bool',   '开发者月费开关',       'pay', 'v0.4.x：1=启用开发者月度服务费（开发者控制台展示月费账单）；0=关闭'),
('pay.tenant_monthly_fee.amount',    '50.00', 'number', '开发者月费金额',       'pay', 'v0.4.x：开发者每月服务费金额（单位：元），生效于下一次账单生成'),
('pay.tenant_monthly_fee.free_days', '30',    'number', '开发者月费免费试用天数','pay', 'v0.4.x：新开发者注册后免费天数，期间不生成月费账单；默认 30 天')
ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
