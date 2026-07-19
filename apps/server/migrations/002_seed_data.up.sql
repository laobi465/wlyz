-- ============================================================
-- KeyAuth SaaS 种子数据初始化
-- 版本：0.2.0
-- 说明：默认 sys_config + 默认套餐 + 默认超管
-- 注意：本文件中所有金额、费率、阈值均为「默认值」，可在后台动态修改（铁律 05）
-- ============================================================

SET NAMES utf8mb4;

-- ============================================================
-- 1. sys_config 默认配置项
-- ============================================================
-- 分组约定：basic / security / verify / pay / agent / notice / email / limit

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
-- ---------- 基础配置 basic ----------
('basic.platform_name', 'KeyAuth SaaS', 'string', '平台名称', 'basic', '平台对外显示名称'),
('basic.platform_url', '', 'string', '平台官网地址', 'basic', '用于支付回调与对外链接（待填写）'),
('basic.icp_record', '', 'string', 'ICP 备案号', 'basic', '页脚备案信息（待填写）'),
('basic.contact_email', '', 'string', '客服邮箱', 'basic', '平台客服联系邮箱（待填写）'),
('basic.register_enabled', '1', 'bool', '是否开放开发者自助注册', 'basic', '关闭后只能由管理员后台手动开通'),
('basic.register_need_pay', '1', 'bool', '开发者注册是否需要支付', 'basic', '开启后开发者注册时需支付套餐费用'),
('basic.trial_days', '7', 'number', '试用天数', 'basic', '新注册开发者赠送的免费试用天数'),

-- ---------- 安全配置 security ----------
('security.sign.timestamp_tolerance', '300', 'number', '签名时间容差(秒)', 'security', '客户端请求时间戳与服务器时间的允许偏差'),
('security.sign.nonce_expire', '600', 'number', 'nonce 防重放保留时长(秒)', 'security', 'Redis 中 nonce 缓存过期时间'),
('security.rate.limit_global', '100', 'number', '全局接口限流(次/分钟/IP)', 'security', '所有客户端接口共享的速率上限'),
('security.rate.limit_sensitive', '20', 'number', '敏感接口限流(次/分钟/IP)', 'security', '登录/验证/绑定等接口单独限流'),
('security.ban.threshold', '50', 'number', 'IP 自动封禁阈值(失败次数)', 'security', '同一 IP 在统计窗口内失败次数达到该值即封禁'),
('security.ban.duration_seconds', '86400', 'number', 'IP 封禁时长(秒)', 'security', '默认 24 小时'),
('security.ban.window_seconds', '3600', 'number', '封禁统计窗口(秒)', 'security', '默认 1 小时统计窗口'),
('security.password_min_length', '8', 'number', '密码最小长度', 'security', '所有账号密码最小长度要求'),
('security.totp_enabled', '0', 'bool', '是否强制开启 2FA', 'security', '开启后所有管理员/开发者必须绑定 TOTP'),
('security.aes_key_rotation_days', '90', 'number', 'AES 密钥轮换提醒周期(天)', 'security', '仅提醒不影响业务，实际轮换需手动操作'),
('security.rsa_key_rotation_days', '365', 'number', 'RSA 密钥轮换提醒周期(天)', 'security', '响应签名 RSA-4096 密钥轮换周期'),

-- ---------- 验证配置 verify ----------
('verify.heartbeat.interval', '60', 'number', '心跳默认间隔(秒)', 'verify', '应用未单独配置时使用此默认值'),
('verify.heartbeat.timeout', '180', 'number', '心跳超时判定(秒)', 'verify', '超过该时间未心跳则设备标记为离线'),
('verify.offline_grace', '86400', 'number', '离线宽限期(秒)', 'verify', '设备断网后允许继续验证的宽限期，默认 24 小时'),
('verify.unbind_deduct_seconds', '86400', 'number', '换绑扣减时长(秒)', 'verify', '设备换绑时从卡密有效期中扣减的时长'),
('verify.rsa_sign_response', '1', 'bool', '是否对客户端响应做 RSA 签名', 'verify', 'fail-closed 模式：开启后无签名响应客户端拒绝接受'),
('verify.allow_unbound_get_var', '0', 'bool', '未绑定设备是否可读云变量', 'verify', '默认禁止，需绑定后才能读取'),
('verify.device_fingerprint_fields', '["cpu","motherboard","mac","disk"]', 'json', '硬件指纹组成字段', 'verify', '参与 HWID 计算的硬件字段组合'),

-- ---------- 支付配置 pay ----------
('pay.platform.enabled', '1', 'bool', '是否启用平台总支付', 'pay', '关闭后所有未配置自定义支付的开发者无法收款'),
('pay.platform.gateway_url', '', 'string', '平台易支付网关地址', 'pay', '彩虹易支付接口地址（待填写）'),
('pay.platform.pid', '', 'string', '平台易支付商户 PID', 'pay', '（待填写）'),
('pay.platform.key_encrypted', '', 'string', '平台易支付商户密钥(已加密)', 'pay', '由后端启动时通过 AES-256-GCM 加密写入（待填写）'),
('pay.platform.methods', '["alipay","wxpay","qqpay"]', 'json', '平台支付支持的支付方式', 'pay', '客户端可选支付方式列表'),
('pay.platform.notify_path', '/api/v1/pay/platform/notify', 'string', '平台支付异步通知路径', 'pay', '易支付异步回调路径'),
('pay.platform.return_path', '/pay/return', 'string', '平台支付同步跳转路径', 'pay', '支付完成后前端跳转路径'),
('pay.platform.commission_default', '5.00', 'number', '平台默认抽成比例(%)', 'pay', '使用平台总支付时默认平台抽成，套餐可单独覆盖'),
('pay.custom_pay.enabled_default', '0', 'bool', '开发者默认是否可使用自定义支付', 'pay', '需套餐 allow_custom_pay=1 才生效'),
('pay.custom_pay.fee_monthly', '30.00', 'number', '开通自定义支付附加月费(元)', 'pay', '套餐 custom_pay_fee 为 0 时使用此默认值'),
('pay.order_expire_seconds', '1800', 'number', '订单未支付自动关闭时长(秒)', 'pay', '默认 30 分钟'),

-- ---------- 代理配置 agent ----------
('agent.register.fee', '99.00', 'number', '代理注册费(元)', 'agent', '代理通过开发者邀请码注册时需支付的费用'),
('agent.register.enabled', '1', 'bool', '是否开放代理注册', 'agent', '总开关，关闭后所有代理注册入口禁用'),
('agent.invite_code_expire_days', '30', 'number', '邀请码有效期(天)', 'agent', '开发者生成的代理邀请码有效期'),
('agent.commission_default_mode', 'diff', 'string', '默认代理分成模式', 'agent', 'percentage=按比例 / diff=按差价'),
('agent.commission_default_rate', '20.00', 'number', '默认代理分成比例(%)', 'agent', 'percentage 模式下的默认比例'),
('agent.min_withdraw_amount', '100.00', 'number', '最低提现金额(元)', 'agent', '代理申请提现的最低金额门槛'),
('agent.withdraw_fee_rate', '0.00', 'number', '提现手续费率(%)', 'agent', '默认 0，可按需配置'),
('agent.balance_negative_limit', '0.00', 'number', '代理余额负数容忍上限(元)', 'agent', '默认 0 不允许透支'),
('agent.notify_on_pay_change', '1', 'bool', '开发者切换支付方式时是否通知代理', 'agent', '通过 notice 系统下发 agent_notify 类型公告'),

-- ---------- 公告配置 notice ----------
('notice.platform.banner_enabled', '1', 'bool', '是否显示平台公告横幅', 'notice', '在控制台顶部显示显眼的「平台公告」横幅'),
('notice.platform.banner_color', '#fff7e6', 'string', '平台公告横幅背景色', 'notice', '默认浅黄警示色'),
('notice.platform.banner_text_color', '#d46b08', 'string', '平台公告横幅文字色', 'notice', '默认深橙警示色'),
('notice.force_popup_seconds', '86400', 'number', '强制弹窗冷却时长(秒)', 'notice', '同一用户同一公告弹窗冷却，默认 24 小时'),
('notice.max_active_per_level', '5', 'number', '每层级最大活跃公告数', 'notice', '平台/开发者/应用各自同时生效的最大公告数'),

-- ---------- 邮件配置 email ----------
('email.enabled', '0', 'bool', '是否启用邮件发送', 'email', '总开关'),
('email.smtp_host', '', 'string', 'SMTP 服务器地址', 'email', '（待填写）'),
('email.smtp_port', '465', 'number', 'SMTP 端口', 'email', 'SSL 通常为 465，TLS 为 587'),
('email.smtp_username', '', 'string', 'SMTP 用户名', 'email', '（待填写）'),
('email.smtp_password_encrypted', '', 'string', 'SMTP 密码(已加密)', 'email', '由后端通过 AES 加密后写入（待填写）'),
('email.from_address', '', 'string', '发件人邮箱', 'email', '（待填写）'),
('email.from_name', 'KeyAuth SaaS', 'string', '发件人名称', 'email', '默认显示名'),

-- ---------- 数量限制 limit ----------
('limit.tenant_default_apps', '1', 'number', '开发者默认最大应用数', 'limit', '注册时未指定套餐时使用'),
('limit.tenant_default_cards', '1000', 'number', '开发者默认最大卡密数', 'limit', '注册时未指定套餐时使用'),
('limit.card_batch_max', '1000', 'number', '单次批量生成卡密上限', 'limit', '防止一次性生成过多卡密'),
('limit.export_max_rows', '10000', 'number', '单次导出最大行数', 'limit', '卡密/订单导出上限'),
('limit.agent_max_per_tenant', '10', 'number', '开发者默认最大代理数', 'limit', '套餐 max_agents 为 0 时使用');

-- ============================================================
-- 2. 默认套餐 sys_package
-- ============================================================
INSERT INTO `sys_package` (`id`, `name`, `monthly_price`, `yearly_price`, `max_apps`, `max_cards`, `max_agents`, `allow_custom_pay`, `custom_pay_fee`, `platform_commission_rate`, `features`, `status`) VALUES
(1, '免费版', 0.00, 0.00, 1, 100, 0, 0, 0.00, 10.00, '{"trial": true, "support": "community"}', 'active'),
(2, '专业版', 99.00, 999.00, 5, 10000, 5, 0, 0.00, 5.00, '{"trial": false, "support": "email", "export": true}', 'active'),
(3, '企业版', 399.00, 3999.00, 50, 100000, 50, 1, 30.00, 3.00, '{"trial": false, "support": "phone", "export": true, "priority": true, "white_label": true}', 'active');

-- ============================================================
-- 3. 默认超管 sys_admin
-- ============================================================
-- ⚠️ 待核实：bcrypt(cost=12) 哈希值需在首次部署后通过运维脚本重新生成
--    当前为占位哈希，对应明文密码：Admin@2026
--    部署后请务必执行：scripts/reset_admin_password.sh 修改默认密码
--    哈希生成命令（待核实）：
--    htpasswd -bnBC 12 "" 'Admin@2026' | tr -d ':\n' | sed 's/$2y/$2a/'
INSERT INTO `sys_admin` (`id`, `username`, `password_hash`, `email`, `phone`, `status`, `totp_secret`) VALUES
(1, 'admin', '$2a$12$PLACEHOLDER_BCRYPT_HASH_REPLACE_ON_DEPLOY_ADMIN_2026_DEFAULT_PASSWORD_MUST_BE_RESET_BEFORE_PRODUCTION_USE', NULL, NULL, 'active', NULL);

-- ============================================================
-- 4. 平台初始公告 notice（示例占位，待管理员后台编辑）
-- ============================================================
-- 平台层级公告：type=platform, tenant_id/app_id 均 NULL
INSERT INTO `notice` (`id`, `type`, `tenant_id`, `app_id`, `title`, `content`, `is_pinned`, `is_popup`, `show_badge`, `start_at`, `end_at`, `status`, `view_count`, `created_by`) VALUES
(1, 'platform', NULL, NULL, '欢迎使用 KeyAuth SaaS', '欢迎接入 KeyAuth SaaS 多租户卡密验证平台。请尽快在管理员后台修改默认密码并配置平台易支付参数。', 1, 1, 1, NOW(), DATE_ADD(NOW(), INTERVAL 30 DAY), 'active', 0, 1);

-- ============================================================
-- 5. IP 黑名单占位（默认空，由后台或自动封禁写入）
-- ============================================================
-- 不插入数据，仅作说明：sec_ip_blacklist 由管理员后台或 ratelimit 中间件自动写入

-- ============================================================
-- 完成提示
-- ============================================================
-- 执行完成后请：
-- 1. 运行 scripts/reset_admin_password.sh 重置超管密码
-- 2. 后台「系统配置 > 支付」中填写平台易支付网关地址、PID、商户密钥
-- 3. 后台「系统配置 > 邮件」中配置 SMTP（如需邮件通知）
-- 4. 在「公告管理」中编辑或删除默认欢迎公告
