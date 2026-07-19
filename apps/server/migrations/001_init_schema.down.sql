-- ============================================================
-- KeyAuth SaaS 回滚：初始 schema
-- 注意：生产环境慎用，会清空所有业务数据
-- ============================================================
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS `log_operation`;
DROP TABLE IF EXISTS `log_verify`;
DROP TABLE IF EXISTS `sec_ip_blacklist`;
DROP TABLE IF EXISTS `notice_read`;
DROP TABLE IF EXISTS `notice_target`;
DROP TABLE IF EXISTS `notice`;
DROP TABLE IF EXISTS `agent_registration_order`;
DROP TABLE IF EXISTS `agent_commission`;
DROP TABLE IF EXISTS `agent_withdraw`;
DROP TABLE IF EXISTS `agent_balance_log`;
DROP TABLE IF EXISTS `agent_invite_code`;
DROP TABLE IF EXISTS `agent`;
DROP TABLE IF EXISTS `app_version`;
DROP TABLE IF EXISTS `app_cloud_var`;
DROP TABLE IF EXISTS `app_order`;
DROP TABLE IF EXISTS `app_device`;
DROP TABLE IF EXISTS `app_card`;
DROP TABLE IF EXISTS `app_card_type`;
DROP TABLE IF EXISTS `app`;
DROP TABLE IF EXISTS `tenant_pay_config`;
DROP TABLE IF EXISTS `sys_tenant`;
DROP TABLE IF EXISTS `sys_package`;
DROP TABLE IF EXISTS `sys_admin`;
DROP TABLE IF EXISTS `sys_config`;

SET FOREIGN_KEY_CHECKS = 1;
