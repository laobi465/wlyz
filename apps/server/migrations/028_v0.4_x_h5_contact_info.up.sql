-- v0.4.x H5 联系客服配置（残留项 4：U-14）
-- 1. sys_config 新增 4 项 contact.* 配置（QQ 群 / 微信 / 邮箱 / 电话）
-- 2. 公开端点 GET /api/v1/public/contact 从 sys_config 读取并返回
--
-- 严格遵循铁律 04/05：
--   04 - 无硬编码：客服联系方式全部走 sys_config，可后台实时调整
--   05 - 配置走后端：4 项 contact.* 配置可通过「系统配置」实时调整
--
-- 配套代码：
--   apps/server/internal/handler/public.go PublicContact
--   apps/admin/src/api/enduser.ts getContactInfoApi
--   apps/admin/src/views/h5/Contact.vue

INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('contact.qq_group',  '', 'string', '客服 QQ 群号', 'contact', 'v0.4.x 残留项 4：H5 联系客服页展示的 QQ 群号（可空，留空则不展示）'),
('contact.wechat',    '', 'string', '客服微信号',   'contact', 'v0.4.x 残留项 4：H5 联系客服页展示的微信号（可空，留空则不展示）'),
('contact.email',     '', 'string', '客服邮箱',     'contact', 'v0.4.x 残留项 4：H5 联系客服页展示的邮箱（可空，留空则不展示）'),
('contact.phone',     '', 'string', '客服电话',     'contact', 'v0.4.x 残留项 4：H5 联系客服页展示的电话（可空，留空则不展示）') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
