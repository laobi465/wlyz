-- =====================================================================
-- Migration 005: 平台总支付（彩虹易支付）结算记录表 + 配置修正
-- 关联功能：v0.2.3 平台总支付下单/回调/自动发卡/抽成结算
-- 铁律 05：所有可变参数必须后台化（sys_config 表）
-- =====================================================================

-- ============================================================
-- 1. 平台抽成结算记录表 platform_settlement
-- ============================================================
CREATE TABLE IF NOT EXISTS `platform_settlement` (
    `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tenant_id`          BIGINT UNSIGNED NOT NULL COMMENT '开发者租户 ID',
    `order_id`           BIGINT UNSIGNED NOT NULL COMMENT '关联 app_order.id',
    `order_no`           VARCHAR(64)     NOT NULL COMMENT '订单号',
    `gross_amount`       DECIMAL(10,2)   NOT NULL COMMENT '订单总额',
    `commission_rate`    DECIMAL(5,2)    NOT NULL COMMENT '抽成比例(%)',
    `commission_amount`  DECIMAL(10,2)   NOT NULL COMMENT '平台抽成金额',
    `net_amount`         DECIMAL(10,2)   NOT NULL COMMENT '开发者应得金额',
    `status`             VARCHAR(32)     NOT NULL DEFAULT 'pending' COMMENT 'pending/settled/rejected',
    `settled_at`         DATETIME        DEFAULT NULL COMMENT '结算时间',
    `settle_batch_no`    VARCHAR(64)     DEFAULT NULL COMMENT '结算批次号',
    `settle_method`      VARCHAR(32)     DEFAULT NULL COMMENT '结算方式 manual/alipay/wechat/bank',
    `settle_remark`      VARCHAR(255)    DEFAULT NULL COMMENT '结算备注',
    `created_at`         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order` (`order_id`),
    KEY `idx_tenant_status` (`tenant_id`, `status`),
    KEY `idx_order_no` (`order_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台抽成结算记录';

-- ============================================================
-- 2. 修正 002 seed 中与实际路由不一致的配置项默认值
--    实际路由：/api/v1/pay/notify/epay 与 /api/v1/pay/return/epay
-- ============================================================
UPDATE `sys_config` SET `config_value` = '/api/v1/pay/notify/epay'
  WHERE `config_key` = 'pay.platform.notify_path' AND `config_value` = '/api/v1/pay/platform/notify';

UPDATE `sys_config` SET `config_value` = '/api/v1/pay/return/epay'
  WHERE `config_key` = 'pay.platform.return_path' AND `config_value` = '/pay/return';

-- ============================================================
-- 3. 新增支付配置项
-- ============================================================
DELETE FROM `sys_config` WHERE `config_key` IN (
  'pay.platform.sign_type',
  'pay.platform.return_front_url',
  'pay.platform.order_name_prefix',
  'pay.platform.callback_retry_max',
  'pay.settlement.min_amount',
  'pay.settlement.auto_enabled'
);

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('pay.platform.sign_type',          'MD5',                    'string', '平台易支付签名类型',     'pay', '彩虹易支付协议固定为 MD5，预留扩展'),
('pay.platform.return_front_url',   '/pay/result',            'string', '支付完成前端跳转地址',   'pay', '同步回调后 302 跳转到此前端地址，附加 ?order_no=xxx'),
('pay.platform.order_name_prefix',  'KeyAuth卡密',            'string', '易支付订单名称前缀',     'pay', '提交给易支付时显示的商品名前缀，后接卡类名'),
('pay.platform.callback_retry_max', '3',                      'number', '回调失败重试上限',       'pay', '平台主动查询订单状态的最大重试次数'),
('pay.settlement.min_amount',       '100.00',                 'number', '最低结算金额(元)',       'pay', '开发者申请结算的最低门槛'),
('pay.settlement.auto_enabled',     '0',                      'bool',   '是否启用自动结算',       'pay', '开启后每日自动结算已结算金额到开发者账户');
