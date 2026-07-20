-- v0.4.0 API 开放平台 回滚（migration 016 down）

DROP TABLE IF EXISTS `webhook_delivery`;
DROP TABLE IF EXISTS `webhook_endpoint`;
DROP TABLE IF EXISTS `developer_api_token`;

DELETE FROM `sys_config` WHERE `config_key` IN (
    'openapi.token.prefix',
    'openapi.token.length',
    'openapi.token.max_per_tenant',
    'openapi.token.default_ttl_days',
    'openapi.scope.available',
    'webhook.timeout_seconds',
    'webhook.max_retry',
    'webhook.failure_threshold'
);
