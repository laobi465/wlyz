-- v0.5.0 集成扩展批次 3：海外支付通道（USDT / PayPal / Stripe）
-- 1. USDT-TRC20：固定收款地址 + 金额唯一后缀匹配 + TronGrid 轮询 / 外部监控 webhook
-- 2. PayPal Orders API v2：OAuth2 + 创建订单 + webhook 验签
-- 3. Stripe Payment Intents API：创建 PaymentIntent + webhook HMAC-SHA256 验签
--
-- 铁律 04/05：15 项 pay.{usdt,paypal,stripe}.* sys_config 全部走后台「系统配置」可调

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
-- ============== USDT-TRC20 ==============
('pay.usdt.enabled',                       '0',                                'bool',   'USDT 支付开关',                'pay', 'v0.5.0 1=启用 USDT-TRC20 通道；0=关闭'),
('pay.usdt.trc20_address',                 '',                                 'string', 'USDT-TRC20 收款地址',           'pay', 'v0.5.0 T 开头 34 位 TRON 地址'),
('pay.usdt.contract_address',              'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t', 'string', 'USDT 合约地址',              'pay', 'v0.5.0 默认主网 USDT-TRC20 合约 TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t'),
('pay.usdt.hmac_secret_enc',               '',                                 'string', 'USDT webhook HMAC 密钥(AES加密)', 'pay', 'v0.5.0 外部监控 webhook 签名密钥，AES-256-GCM 加密'),
('pay.usdt.exchange_rate',                 '0.14',                             'number', 'CNY→USDT 汇率',                'pay', 'v0.5.0 1 CNY = X USDT，默认 0.14'),
('pay.usdt.polling_enabled',               '0',                                'bool',   '启用 TronGrid 轮询',           'pay', 'v0.5.0 1=内置轮询 worker；0=仅外部监控 webhook'),
('pay.usdt.polling_interval_seconds',      '60',                               'number', 'TronGrid 轮询间隔(秒)',         'pay', 'v0.5.0 默认 60 秒'),
('pay.usdt.trongrid_api_key',              '',                                 'string', 'TronGrid API Key',             'pay', 'v0.5.0 可选，提高速率限制'),
('pay.usdt.expire_seconds',                '1800',                             'number', 'USDT 订单超时(秒)',            'pay', 'v0.5.0 默认 1800 秒'),
-- ============== PayPal Orders v2 ==============
('pay.paypal.enabled',                     '0',                                'bool',   'PayPal 支付开关',              'pay', 'v0.5.0 1=启用 PayPal 通道；0=关闭'),
('pay.paypal.client_id',                   '',                                 'string', 'PayPal Client ID',             'pay', 'v0.5.0 PayPal 应用 Client ID'),
('pay.paypal.client_secret_enc',           '',                                 'string', 'PayPal Client Secret(AES加密)','pay', 'v0.5.0 PayPal 应用 Client Secret，AES-256-GCM 加密'),
('pay.paypal.webhook_id',                  '',                                 'string', 'PayPal Webhook ID',            'pay', 'v0.5.0 WH- 开头的 webhook ID，用于验签'),
('pay.paypal.sandbox',                     '1',                                'bool',   'PayPal 沙盒模式',              'pay', 'v0.5.0 1=sandbox 沙盒；0=live 生产'),
('pay.paypal.exchange_rate',               '0.14',                             'number', 'CNY→USD 汇率(PayPal)',         'pay', 'v0.5.0 1 CNY = X USD，默认 0.14'),
('pay.paypal.expire_seconds',              '1800',                             'number', 'PayPal 订单超时(秒)',          'pay', 'v0.5.0 默认 1800 秒'),
-- ============== Stripe Payment Intents ==============
('pay.stripe.enabled',                     '0',                                'bool',   'Stripe 支付开关',              'pay', 'v0.5.0 1=启用 Stripe 通道；0=关闭'),
('pay.stripe.secret_key_enc',              '',                                 'string', 'Stripe Secret Key(AES加密)',  'pay', 'v0.5.0 sk_live_xxx 或 sk_test_xxx，AES-256-GCM 加密'),
('pay.stripe.webhook_secret_enc',          '',                                 'string', 'Stripe Webhook Secret(AES加密)', 'pay', 'v0.5.0 whsec_xxx，AES-256-GCM 加密'),
('pay.stripe.exchange_rate',               '0.14',                             'number', 'CNY→USD 汇率(Stripe)',         'pay', 'v0.5.0 1 CNY = X USD，默认 0.14'),
('pay.stripe.expire_seconds',              '1800',                             'number', 'Stripe 订单超时(秒)',          'pay', 'v0.5.0 默认 1800 秒');
