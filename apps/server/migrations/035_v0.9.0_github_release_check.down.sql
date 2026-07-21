-- v0.9.0 GitHub Release 检查更新 - 回滚
-- 删除 4 项 update.github.* 配置
DELETE FROM `sys_config` WHERE `config_key` IN (
    'update.github.owner',
    'update.github.repo',
    'update.github.token',
    'update.github.current_version'
);
