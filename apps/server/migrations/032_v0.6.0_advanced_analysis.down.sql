-- v0.6.0 高级分析回滚
-- 顺序：先删配置，再删表

-- 1. 删除 sys_config 配置项（21 项）
DELETE FROM `sys_config` WHERE `config_group` = 'analysis';

-- 2. 删除 3 张分析表
DROP TABLE IF EXISTS `user_risk_score`;
DROP TABLE IF EXISTS `card_usage_profile`;
DROP TABLE IF EXISTS `user_behavior_profile`;
