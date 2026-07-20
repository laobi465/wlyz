-- v0.4.0 公告增强 + 数据统计看板（第十六项迁移）
-- 1. notice 表新增 content_format 字段（text/html，富文本编辑支持）
-- 2. sys_config 新增 5 项 notice.* 配置（公告相关参数后台化）
-- 3. sys_config 新增 4 项 stats.* 配置（统计看板参数后台化）
--
-- 严格遵循铁律 04/05/06：
--   04 - 无硬编码：公告 / 统计相关阈值全部从 sys_config 读取
--   05 - 配置走后端：9 项 notice.*/stats.* 配置可通过后台实时调整
--   06 - 反幻觉：富文本字段允许 HTML 但前端需做 XSS 过滤；统计聚合基于真实数据

-- ============== 1. notice 表新增 content_format 字段 ==============
ALTER TABLE `notice` ADD COLUMN `content_format` VARCHAR(16) NOT NULL DEFAULT 'text' COMMENT '内容格式：text=纯文本 / html=富文本（v0.4.0 公告富文本编辑）' AFTER `content`;

-- ============== 2. sys_config 5 项 notice.* 配置 ==============
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('notice.popup.max_unread',       '5',     'number', '弹窗公告最大未读数', 'notice', 'v0.4.0 单次 popup 接口返回的最大未读弹窗数量，超过则按 is_pinned DESC + start_at DESC 取前 N 条'),
('notice.popup.enabled',          '1',     'bool',   '弹窗公告总开关',     'notice', 'v0.4.0 1=启用首次登录强制弹窗功能，前端轮询 /popup 接口；0=关闭弹窗，仅显示普通列表'),
('notice.popup.dismiss_ttl_hours','24',    'number', '弹窗关闭后再次提醒间隔（小时）', 'notice', 'v0.4.0 用户关闭弹窗后多少小时内不再提醒，0=每次登录都弹（前端 localStorage 配合）'),
('notice.richtext.enabled',       '1',     'bool',   '富文本编辑开关',     'notice', 'v0.4.0 1=管理员/开发者可用富文本编辑器（content_format=html）；0=仅纯文本'),
('notice.richtext.max_length',    '10000', 'number', '富文本最大长度',     'notice', 'v0.4.0 content 字段富文本最大字符数，防止超大内容影响列表查询性能') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);

-- ============== 3. sys_config 4 项 stats.* 配置 ==============
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('stats.verify_trend.default_days',  '30', 'number', '验证趋势图默认天数', 'stats', 'v0.4.0 验证趋势图默认查询近 N 天的数据，前端可调整范围 1-90'),
('stats.verify_trend.max_days',      '90', 'number', '验证趋势图最大天数', 'stats', 'v0.4.0 验证趋势图最大查询天数，防止超大范围聚合影响 DB 性能'),
('stats.agent_ranking.default_limit','10', 'number', '代理业绩排行默认条数', 'stats', 'v0.4.0 代理业绩排行默认返回前 N 名，前端可调整 1-100'),
('stats.agent_ranking.max_limit',    '100','number', '代理业绩排行最大条数', 'stats', 'v0.4.0 代理业绩排行最大返回条数，防止结果集过大') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
