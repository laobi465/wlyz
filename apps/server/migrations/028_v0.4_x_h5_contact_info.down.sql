-- v0.4.x H5 联系客服配置 - 回滚
-- 删除 sys_config 中 4 项 contact.* 配置

DELETE FROM `sys_config` WHERE `config_key` IN (
  'contact.qq_group',
  'contact.wechat',
  'contact.email',
  'contact.phone'
);
