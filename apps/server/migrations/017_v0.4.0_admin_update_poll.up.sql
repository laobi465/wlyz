-- v0.4.0 管理员弹窗通知（前端轮询 /admin/update/poll 检测新 commit）
-- 1. sys_config 2 项：弹窗通知开关 + 建议轮询间隔
--
-- 严格遵循铁律 04/05/06：
--   04 - 无硬编码：轮询开关 / 间隔 全部从 sys_config 读取
--   05 - 配置走后端：2 项 update.poll.* 配置可通过后台实时调整
--   06 - 反幻觉：轻量轮询接口仅返回 commit + 状态（不含 log_text 重字段），降低高频轮询带宽

-- ============== sys_config 2 项 ==============
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('update.poll.enabled',          '1',  'bool',   '管理员更新弹窗通知开关', 'update', 'v0.4.0 1=前端 AdminLayout 启用 30s 轮询 /admin/update/poll，检测到新 commit 弹窗提示管理员刷新；0=关闭弹窗通知'),
('update.poll.interval_seconds', '30', 'number', '弹窗通知轮询间隔（秒）', 'update', 'v0.4.0 前端轮询 /admin/update/poll 的建议间隔，默认 30 秒，最小 10 秒（前端会强制下限）') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
