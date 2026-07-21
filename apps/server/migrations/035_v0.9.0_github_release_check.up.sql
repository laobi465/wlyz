-- v0.9.0 GitHub Release 检查更新
-- 主动调用 GitHub API 查询最新 release，对比当前部署版本，提示管理员有新版本可更新
--
-- 严格遵循铁律：
--   04 - GitHub owner/repo/token/current_version 全部走 sys_config，禁止硬编码
--   05 - 4 项 update.github.* 配置可通过后台「系统配置」实时调整
--   06 - token 仅用于 GitHub API Authorization 头，不记录日志、不回显前端
--
-- 与 v0.4.0 update.webhook.* 的区别：
--   update.webhook.* 是被动接收 GitHub push event 触发自动更新
--   update.github.*  是主动调用 GitHub releases/latest API 检查是否有新版本

-- ============== sys_config 4 项 update.github.* ==============
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('update.github.owner',           '',           'string', 'GitHub 仓库 Owner',       'update', 'v0.9.0 GitHub 仓库 owner（如 laobi465），用于调用 releases/latest API 检查更新'),
('update.github.repo',            '',           'string', 'GitHub 仓库名',           'update', 'v0.9.0 GitHub 仓库名（如 wlyz），用于调用 releases/latest API 检查更新'),
('update.github.token',           '',           'string', 'GitHub API Token',        'update', 'v0.9.0 GitHub Personal Access Token（可选，避免匿名限流 60次/小时；配置后提升至 5000次/小时）。仅用于 Authorization 头，不回显前端'),
('update.github.current_version', '',           'string', '当前部署版本号',          'update', 'v0.9.0 当前系统部署的版本号（如 v0.9.0），用于与 GitHub release tag_name 对比判断是否有更新；空=未配置，默认认为有更新') ON DUPLICATE KEY UPDATE `remark` = VALUES(`remark`);
