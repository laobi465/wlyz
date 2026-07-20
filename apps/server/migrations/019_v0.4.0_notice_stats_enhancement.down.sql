-- v0.4.0 公告增强 + 数据统计看板 - 回滚
-- 1. 删除 9 项 notice.*/stats.* sys_config
-- 2. 删除 notice.content_format 字段

DELETE FROM `sys_config` WHERE `config_key` IN (
  'notice.popup.max_unread',
  'notice.popup.enabled',
  'notice.popup.dismiss_ttl_hours',
  'notice.richtext.enabled',
  'notice.richtext.max_length',
  'stats.verify_trend.default_days',
  'stats.verify_trend.max_days',
  'stats.agent_ranking.default_limit',
  'stats.agent_ranking.max_limit'
);

ALTER TABLE `notice` DROP COLUMN `content_format`;
