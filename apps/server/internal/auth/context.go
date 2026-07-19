package auth

import "context"

// nilCtx 用于 Redis 操作的默认 context（refresh token 操作不需要请求级别 context）
// 后续如需 trace / 超时控制可替换为注入的 context
var nilCtx = context.Background()
