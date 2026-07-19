-- ============================================================
-- KeyAuth SaaS 回滚：种子数据
-- ============================================================
SET NAMES utf8mb4;

DELETE FROM `notice` WHERE `id` = 1;
DELETE FROM `sys_admin` WHERE `id` = 1;
DELETE FROM `sys_package` WHERE `id` IN (1, 2, 3);
DELETE FROM `sys_config` WHERE `config_group` IN ('basic', 'security', 'verify', 'pay', 'agent', 'notice', 'email', 'limit');
