// 客户端验证 API 处理器（骨架）
// 对应路由：/api/v1/client/*
// 所有接口都经过 SignatureAuth 中间件，注入了 app_id / tenant_id / app 到上下文
package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 类型定义 ==============

// ClientLoginReq 登录请求
type ClientLoginReq struct {
	CardKey   string `json:"card_key" binding:"required"`
	HWID      string `json:"hwid" binding:"required"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"` // windows/macos/linux/android/ios/web
}

// ClientVerifyReq 验证请求
type ClientVerifyReq struct {
	CardKey string `json:"card_key" binding:"required"`
	HWID    string `json:"hwid" binding:"required"`
}

// ClientHeartbeatReq 心跳请求
type ClientHeartbeatReq struct {
	CardKey string `json:"card_key" binding:"required"`
	HWID    string `json:"hwid" binding:"required"`
}

// ClientBindReq 绑定设备请求
type ClientBindReq struct {
	CardKey string `json:"card_key" binding:"required"`
	HWID    string `json:"hwid" binding:"required"`
}

// ClientUnbindReq 解绑设备请求
type ClientUnbindReq struct {
	CardKey string `json:"card_key" binding:"required"`
	HWID    string `json:"hwid" binding:"required"`
}

// ClientGetVarReq 获取云变量请求
type ClientGetVarReq struct {
	CardKey string `json:"card_key" binding:"required"`
	VarKey  string `json:"var_key" binding:"required"`
}

// LoginResp 登录响应
type LoginResp struct {
	Token     string                 `json:"token"`
	ExpiresAt int64                  `json:"expires_at"`
	Card      CardInfo               `json:"card"`
	Features  map[string]interface{} `json:"features"`
}

// CardInfo 卡密信息
type CardInfo struct {
	Type              string `json:"type"`
	Status            string `json:"status"`
	ExpiresAt         *int64 `json:"expires_at"`
	RemainingSeconds  int64  `json:"remaining_seconds"`
	BoundDevices      int    `json:"bound_devices"`
	MaxDevices        int    `json:"max_devices"`
}

// ============== 处理器实现 ==============

// ClientLogin 客户端登录（首次自动绑定设备）
// 流程：
// 1. 校验卡密有效性（状态、过期）
// 2. 校验设备绑定（一机一卡）
// 3. 首次登录自动绑定，写入 app_device 表
// 4. 更新卡密状态为 active、设置过期时间
// 5. 写入验证日志
// 6. 返回 Token + 卡密信息 + RSA 签名
func ClientLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientLoginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, 400, 1001, "参数错误: "+err.Error())
			return
		}

		app := c.MustGet("app").(*model.App)
		tenantID := c.GetUint64("tenant_id")

		// TODO(v0.2.0): 实现完整登录逻辑
		// 1. 查卡密（按 card_key_hash）
		// 2. 校验卡密状态
		// 3. 校验设备绑定（app.MaxDevices）
		// 4. 首次登录写入 app_device
		// 5. 更新 app_card.status=active、activated_at、expires_at
		// 6. 写 log_verify
		// 7. 生成 JWT Token
		// 8. RSA 签名响应
		_ = app
		_ = tenantID

		// 占位：实际未实现，返回待开发
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// ClientVerify 验证卡密有效性（不绑定设备）
func ClientVerify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientVerifyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, 400, 1001, "参数错误")
			return
		}
		// TODO(v0.2.0): 查卡密、校验状态、返回信息
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// ClientHeartbeat 心跳保活
// 流程：
// 1. 查卡密 + 设备
// 2. 校验状态
// 3. 更新 app_device.last_heartbeat_at
// 4. Redis 维护在线状态（Sorted Set，score=timestamp）
func ClientHeartbeat(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientHeartbeatReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, 400, 1001, "参数错误")
			return
		}
		// TODO(v0.2.0): 实现 Redis Sorted Set 心跳
		middleware.Success(c, gin.H{"next_heartbeat": time.Now().Add(60 * time.Second).Unix()})
	}
}

// ClientBind 手动绑定设备
func ClientBind(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// ClientUnbind 解绑设备（扣时）
// 流程：
// 1. 校验卡密 + 设备
// 2. 删除 app_device 记录（或标记 unbound）
// 3. 卡密 expires_at -= app.UnbindDeductSeconds（扣时）
// 4. 写日志
func ClientUnbind(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// ClientGetVar 获取云变量
func ClientGetVar(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// ClientNotice 获取公告
func ClientNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// ClientVersion 检查版本更新
func ClientVersion(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// ClientLogout 退出登录
func ClientLogout(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Success(c, nil)
	}
}
