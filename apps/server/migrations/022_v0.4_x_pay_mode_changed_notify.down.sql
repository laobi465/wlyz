-- v0.4.x 支付方式变更通知代理 - 回滚
-- 删除 pay_mode_changed 站内信模板 + 配置开关

DELETE FROM `notify_template` WHERE `code` = 'pay_mode_changed' AND `tenant_id` = 0;
DELETE FROM `sys_config` WHERE `config_key` = 'notify.pay_mode_changed.enabled';
