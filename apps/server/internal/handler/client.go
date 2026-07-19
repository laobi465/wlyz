// 客户端验证 API 处理器
// 对应路由：/api/v1/client/*
// 所有接口都经过 SignatureAuth 中间件，注入了 app_id / tenant_id / app 到上下文
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取
// 严格遵循铁律 06：不确定处标注「需验证」或「待核实」
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/heartbeat"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== DTO ==============

type ClientLoginReq struct {
	CardKey    string `json:"card_key" binding:"required"`
	HWID       string `json:"hwid" binding:"required"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"` // windows/macos/linux/android/ios/web
}

type ClientVerifyReq struct {
	CardKey string `json:"card_key" binding:"required"`
	HWID    string `json:"hwid" binding:"required"`
}

type ClientHeartbeatReq struct {
	CardKey string `json:"card_key" binding:"required"`
	HWID    string `json:"hwid" binding:"required"`
}

type ClientUnbindReq struct {
	CardKey string `json:"card_key" binding:"required"`
	HWID    string `json:"hwid" binding:"required"`
}

type ClientGetVarReq struct {
	CardKey string `json:"card_key" binding:"required"`
	VarKey  string `json:"var_key" binding:"required"`
}

// ============== 通用辅助 ==============

// clientCtx 客户端接口上下文（从中间件 + DB 加载）
type clientCtx struct {
	App    *model.App
	Card   *model.AppCard
	Device *model.AppDevice
}

// loadCardByHWID 根据卡密明文查找卡密
// 注：生产环境应按 hash 查询（防穷举），此处简化为按 card_key 直接查
// 待核实：是否需要建立 card_key_hash 唯一索引（已有，但 SDK 默认传明文，建议按 hash 查询）
func loadCardByCardKey(db *gorm.DB, tenantID uint64, cardKey string) (*model.AppCard, error) {
	var card model.AppCard
	// 优先按 hash 查询（性能 + 安全）
	hash := crypto.SHA512Hex(cardKey)
	err := db.Where("tenant_id = ? AND card_key_hash = ?", tenantID, hash).First(&card).Error
	if err == gorm.ErrRecordNotFound {
		// 兜底按明文查（兼容历史数据）
		err = db.Where("tenant_id = ? AND card_key = ?", tenantID, cardKey).First(&card).Error
	}
	if err != nil {
		return nil, err
	}
	return &card, nil
}

// loadDeviceByHWID 根据 HWID 查找设备
func loadDeviceByHWID(db *gorm.DB, tenantID, appID uint64, hwid string) (*model.AppDevice, error) {
	var dev model.AppDevice
	err := db.Where("tenant_id = ? AND app_id = ? AND hwid = ? AND status = ?",
		tenantID, appID, hwid, "active").First(&dev).Error
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

// cardToInfo 卡密转 CardInfo
func cardToInfo(card *model.AppCard, app *model.App, boundDevices int) gin.H {
	now := time.Now()
	var remaining int64 = 0
	if card.ExpiresAt != nil {
		remaining = card.ExpiresAt.Unix() - now.Unix()
		if remaining < 0 {
			remaining = 0
		}
	} else if card.DurationSeconds == -1 {
		remaining = -1 // 永久卡
	}
	var expiresAt *int64
	if card.ExpiresAt != nil {
		v := card.ExpiresAt.Unix()
		expiresAt = &v
	}
	return gin.H{
		"type":              cardType(card),
		"status":            card.Status,
		"expires_at":        expiresAt,
		"remaining_seconds": remaining,
		"bound_devices":     boundDevices,
		"max_devices":       app.MaxDevices,
		"used_count":        card.UsedCount,
		"max_uses":          card.MaxUses,
	}
}

// cardType 返回卡密类型描述（待核实：从 card_type 关联查询）
func cardType(card *model.AppCard) string {
	if card.DurationSeconds == -1 {
		return "permanent"
	}
	if card.DurationSeconds > 0 {
		return "duration"
	}
	if card.MaxUses > 1 {
		return "count"
	}
	return "unknown"
}

// isCardValid 校验卡密是否可用（未过期、未封禁、未禁用）
func isCardValid(card *model.AppCard) (bool, string) {
	switch card.Status {
	case "unused":
		return true, ""
	case "active":
		// 校验过期
		if card.ExpiresAt != nil && card.ExpiresAt.Before(time.Now()) {
			return false, "卡密已过期"
		}
		// 校验使用次数
		if card.MaxUses > 0 && card.UsedCount >= card.MaxUses {
			return false, "卡密使用次数已用尽"
		}
		return true, ""
	case "expired":
		return false, "卡密已过期"
	case "banned":
		return false, "卡密已被封禁"
	case "disabled":
		return false, "卡密已被禁用"
	default:
		return false, "卡密状态异常"
	}
}

// countBoundDevices 统计卡密已绑定的活跃设备数
func countBoundDevices(db *gorm.DB, cardID uint64) (int64, error) {
	var count int64
	err := db.Model(&model.AppDevice{}).
		Where("card_id = ? AND status = ?", cardID, "active").Count(&count).Error
	return count, err
}

// ============== ClientLogin 客户端登录 ==============

// ClientLogin 客户端登录（首次自动绑定设备）
// 流程：
// 1. 校验卡密有效性（状态、过期、使用次数）
// 2. 校验设备绑定（一机一卡 / 多卡）
// 3. 首次登录自动绑定，写入 app_device 表
// 4. 更新卡密状态为 active、设置过期时间
// 5. 写入验证日志
// 6. 返回 Token + 卡密信息
func ClientLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientLoginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		app := c.MustGet("app").(*model.App)
		tenantID := app.TenantID

		// 1. 查卡密
		card, err := loadCardByCardKey(deps.DB, tenantID, req.CardKey)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "login", "fail", "卡密不存在")
				middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询卡密失败")
			return
		}

		// 校验卡密属于当前应用
		if card.AppID != app.ID {
			writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "login", "fail", "卡密不属于该应用")
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}

		// 2. 校验卡密状态
		if valid, reason := isCardValid(card); !valid {
			writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "login", "fail", reason)
			middleware.Fail(c, http.StatusForbidden, 2002, reason)
			return
		}

		// 3. 校验设备绑定
		// 3.1 查询当前设备是否已绑定到该卡密
		var existingDev model.AppDevice
		err = deps.DB.Where("card_id = ? AND hwid = ? AND status = ?", card.ID, req.HWID, "active").
			First(&existingDev).Error
		if err == gorm.ErrRecordNotFound {
			// 3.2 新设备：校验绑定数量上限
			boundCount, _ := countBoundDevices(deps.DB, card.ID)
			if int(boundCount) >= app.MaxDevices {
				writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "login", "fail",
					fmt.Sprintf("设备数已达上限 %d", app.MaxDevices))
				middleware.Fail(c, http.StatusForbidden, 2003,
					fmt.Sprintf("设备绑定数已达上限 %d，请先解绑其他设备", app.MaxDevices))
				return
			}
		}

		// 4. 事务：绑定设备 + 激活卡密 + 写日志
		var device *model.AppDevice
		now := time.Now()
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 4.1 写入或更新设备绑定
			if err := tx.Where("card_id = ? AND hwid = ? AND status = ?", card.ID, req.HWID, "active").
				First(&existingDev).Error; err == gorm.ErrRecordNotFound {
				// 新建设备
				device = &model.AppDevice{
					TenantID:        tenantID,
					AppID:           app.ID,
					CardID:          card.ID,
					HWID:            req.HWID,
					DeviceName:      req.DeviceName,
					DeviceType:      req.DeviceType,
					IPAddress:       c.ClientIP(),
					Status:          "active",
					LastHeartbeatAt: &now,
					FirstBoundAt:    now,
				}
				if err := tx.Create(device).Error; err != nil {
					return fmt.Errorf("绑定设备失败: %w", err)
				}
			} else if err == nil {
				// 已存在，更新心跳
				device = &existingDev
				if err := tx.Model(device).Updates(map[string]interface{}{
					"last_heartbeat_at": now,
					"ip_address":        c.ClientIP(),
				}).Error; err != nil {
					return fmt.Errorf("更新设备信息失败: %w", err)
				}
			}

			// 4.2 激活卡密（首次登录时设置 activated_at 和 expires_at）
			updates := map[string]interface{}{
				"status":       "active",
				"used_count":   card.UsedCount + 1,
				"last_verify_at": now,
			}
			if card.ActivatedAt == nil {
				updates["activated_at"] = now
				// 设置过期时间（永久卡除外）
				if card.DurationSeconds > 0 {
					expiresAt := now.Add(time.Duration(card.DurationSeconds) * time.Second)
					updates["expires_at"] = expiresAt
				} else if card.DurationSeconds == -1 {
					updates["expires_at"] = nil // 永久卡
				}
			}
			if err := tx.Model(&model.AppCard{}).Where("id = ?", card.ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("激活卡密失败: %w", err)
			}

			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "登录事务失败: "+txErr.Error())
			return
		}

		// 5. Redis 记录心跳
		_ = heartbeat.Record(c.Request.Context(), deps.Redis, app.ID, device.ID, c.ClientIP(), c.Request.UserAgent())

		// 6. 重新查询卡密（获取更新后的字段）
		card, _ = loadCardByCardKey(deps.DB, tenantID, req.CardKey)

		// 7. 写日志
		writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "login", "success", "登录成功")

		// 8. 返回
		boundCount, _ := countBoundDevices(deps.DB, card.ID)
		middleware.Success(c, gin.H{
			"token":      "", // TODO(v0.3.0): 客户端 Token（目前用 HMAC 签名鉴权，可省略）
			"expires_at": card.ExpiresAt.Unix(),
			"card":       cardToInfo(card, app, int(boundCount)),
			"device": gin.H{
				"id":       device.ID,
				"hwid":     device.HWID,
				"name":     device.DeviceName,
				"bound_at": device.FirstBoundAt.Unix(),
			},
			"heartbeat_interval": app.HeartbeatInterval,
			"heartbeat_timeout":  app.HeartbeatTimeout,
		})
	}
}

// ============== ClientVerify 验证卡密 ==============

// ClientVerify 验证卡密有效性（不绑定设备，不增加使用次数）
// 用于客户端启动时检查登录态是否有效
func ClientVerify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientVerifyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		app := c.MustGet("app").(*model.App)

		// 1. 查卡密
		card, err := loadCardByCardKey(deps.DB, app.TenantID, req.CardKey)
		if err != nil {
			writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "verify", "fail", "卡密不存在")
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}
		if card.AppID != app.ID {
			writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "verify", "fail", "卡密不属于该应用")
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}

		// 2. 校验状态
		if valid, reason := isCardValid(card); !valid {
			writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "verify", "fail", reason)
			middleware.Fail(c, http.StatusForbidden, 2002, reason)
			return
		}

		// 3. 校验设备绑定（仅 active 卡密需要校验设备）
		var dev model.AppDevice
		err = deps.DB.Where("card_id = ? AND hwid = ? AND status = ?", card.ID, req.HWID, "active").
			First(&dev).Error
		if err == gorm.ErrRecordNotFound {
			writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "verify", "fail", "设备未绑定")
			middleware.Fail(c, http.StatusForbidden, 2004, "设备未绑定或已被解绑，请重新登录")
			return
		}

		// 4. 检查离线宽限期
		if app.OfflineGrace > 0 && dev.LastHeartbeatAt != nil {
			sinceLast := time.Since(*dev.LastHeartbeatAt)
			if sinceLast > time.Duration(app.OfflineGrace)*time.Second {
				writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "verify", "fail",
					"超过离线宽限期 "+itoa(app.OfflineGrace)+" 秒")
				middleware.Fail(c, http.StatusForbidden, 2005, "超过离线宽限期，请重新登录")
				return
			}
		}

		// 5. 更新最后验证时间
		now := time.Now()
		deps.DB.Model(&model.AppCard{}).Where("id = ?", card.ID).Update("last_verify_at", now)

		writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "verify", "success", "验证通过")

		boundCount, _ := countBoundDevices(deps.DB, card.ID)
		middleware.Success(c, gin.H{
			"card":               cardToInfo(card, app, int(boundCount)),
			"device":             gin.H{"id": dev.ID, "hwid": dev.HWID},
			"last_heartbeat_at":  dev.LastHeartbeatAt.Unix(),
			"heartbeat_interval": app.HeartbeatInterval,
			"heartbeat_timeout":  app.HeartbeatTimeout,
		})
	}
}

// ============== ClientHeartbeat 心跳保活 ==============

// ClientHeartbeat 心跳保活
// 流程：
// 1. 查卡密 + 设备
// 2. 校验状态
// 3. 更新 app_device.last_heartbeat_at + Redis Sorted Set
func ClientHeartbeat(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientHeartbeatReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误")
			return
		}

		app := c.MustGet("app").(*model.App)

		// 1. 查卡密 + 设备
		card, err := loadCardByCardKey(deps.DB, app.TenantID, req.CardKey)
		if err != nil {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}
		if card.AppID != app.ID {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}
		if valid, reason := isCardValid(card); !valid {
			middleware.Fail(c, http.StatusForbidden, 2002, reason)
			return
		}

		var dev model.AppDevice
		if err := deps.DB.Where("card_id = ? AND hwid = ? AND status = ?", card.ID, req.HWID, "active").
			First(&dev).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 2004, "设备未绑定")
			return
		}

		// 2. 更新 DB + Redis
		now := time.Now()
		deps.DB.Model(&dev).Update("last_heartbeat_at", now)
		_ = heartbeat.Record(c.Request.Context(), deps.Redis, app.ID, dev.ID, c.ClientIP(), c.Request.UserAgent())

		middleware.Success(c, gin.H{
			"next_heartbeat":    now.Add(time.Duration(app.HeartbeatInterval) * time.Second).Unix(),
			"heartbeat_timeout": app.HeartbeatTimeout,
			"server_time":       now.Unix(),
		})
	}
}

// ============== ClientBind 手动绑定设备 ==============

// ClientBind 手动绑定设备（在 MaxDevices > 1 的多机场景下使用）
// 单机应用（MaxDevices=1）登录时已自动绑定，无需调用此接口
func ClientBind(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientLoginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误")
			return
		}

		app := c.MustGet("app").(*model.App)
		card, err := loadCardByCardKey(deps.DB, app.TenantID, req.CardKey)
		if err != nil {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}
		if card.AppID != app.ID {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}
		if valid, reason := isCardValid(card); !valid {
			middleware.Fail(c, http.StatusForbidden, 2002, reason)
			return
		}

		// 校验设备数上限
		boundCount, _ := countBoundDevices(deps.DB, card.ID)
		if int(boundCount) >= app.MaxDevices {
			middleware.Fail(c, http.StatusForbidden, 2003,
				fmt.Sprintf("设备绑定数已达上限 %d", app.MaxDevices))
			return
		}

		// 校验是否已绑定
		var existing model.AppDevice
		err = deps.DB.Where("card_id = ? AND hwid = ? AND status = ?", card.ID, req.HWID, "active").
			First(&existing).Error
		if err == nil {
			middleware.Fail(c, http.StatusConflict, 2006, "设备已绑定该卡密")
			return
		}

		now := time.Now()
		dev := &model.AppDevice{
			TenantID:        app.TenantID,
			AppID:           app.ID,
			CardID:          card.ID,
			HWID:            req.HWID,
			DeviceName:      req.DeviceName,
			DeviceType:      req.DeviceType,
			IPAddress:       c.ClientIP(),
			Status:          "active",
			LastHeartbeatAt: &now,
			FirstBoundAt:    now,
		}
		if err := deps.DB.Create(dev).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "绑定失败: "+err.Error())
			return
		}

		_ = heartbeat.Record(c.Request.Context(), deps.Redis, app.ID, dev.ID, c.ClientIP(), c.Request.UserAgent())
		writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "bind", "success", "绑定成功")

		middleware.Success(c, gin.H{
			"device_id":  dev.ID,
			"bound_at":   now.Unix(),
			"bound_count": boundCount + 1,
			"max_devices": app.MaxDevices,
		})
	}
}

// ============== ClientUnbind 解绑设备 ==============

// ClientUnbind 解绑设备（扣时）
// 流程：
// 1. 校验卡密 + 设备
// 2. 标记设备 unbound
// 3. 卡密 expires_at -= app.UnbindDeductSeconds（扣时）
// 4. Redis 移除在线状态
// 5. 写日志
func ClientUnbind(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientUnbindReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误")
			return
		}

		app := c.MustGet("app").(*model.App)
		card, err := loadCardByCardKey(deps.DB, app.TenantID, req.CardKey)
		if err != nil {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}
		if card.AppID != app.ID {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}

		var dev model.AppDevice
		if err := deps.DB.Where("card_id = ? AND hwid = ? AND status = ?", card.ID, req.HWID, "active").
			First(&dev).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 2007, "设备未绑定")
			return
		}

		now := time.Now()
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 1. 标记设备 unbound
			if err := tx.Model(&dev).Updates(map[string]interface{}{
				"status":     "unbound",
				"unbound_at": now,
			}).Error; err != nil {
				return err
			}
			// 2. 卡密扣时（永久卡不扣）
			if card.DurationSeconds > 0 && card.ExpiresAt != nil && app.UnbindDeductSeconds > 0 {
				newExpiresAt := card.ExpiresAt.Add(-time.Duration(app.UnbindDeductSeconds) * time.Second)
				if err := tx.Model(&model.AppCard{}).Where("id = ?", card.ID).
					Update("expires_at", newExpiresAt).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5004, "解绑失败: "+txErr.Error())
			return
		}

		// 3. Redis 移除在线状态
		_ = heartbeat.Remove(c.Request.Context(), deps.Redis, app.ID, dev.ID)
		writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "unbind", "success", "解绑成功")

		middleware.Success(c, gin.H{
			"unbound":           true,
			"deducted_seconds":  app.UnbindDeductSeconds,
			"message":           fmt.Sprintf("已解绑，扣除 %d 秒有效期", app.UnbindDeductSeconds),
		})
	}
}

// ============== ClientLogout 退出登录 ==============

// ClientLogout 退出登录（仅记录日志，不影响设备绑定状态）
func ClientLogout(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientVerifyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Success(c, nil) // 即使参数缺失也返回成功
			return
		}
		app := c.MustGet("app").(*model.App)
		writeVerifyLogCtx(deps, c, app, req.HWID, req.CardKey, "logout", "success", "退出登录")
		middleware.Success(c, gin.H{"logged_out": true})
	}
}

// ============== ClientGetVar 获取云变量 ==============

// ClientGetVar 获取云变量（需校验卡密有效性）
// POST /api/v1/client/get_var
func ClientGetVar(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ClientGetVarReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误")
			return
		}

		app := c.MustGet("app").(*model.App)
		card, err := loadCardByCardKey(deps.DB, app.TenantID, req.CardKey)
		if err != nil {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "卡密不存在或已失效")
			return
		}
		if valid, reason := isCardValid(card); !valid {
			middleware.Fail(c, http.StatusForbidden, 2002, reason)
			return
		}

		// 查云变量
		var v model.AppCloudVar
		err = deps.DB.Where("app_id = ? AND var_key = ? AND status = ?", app.ID, req.VarKey, "active").First(&v).Error
		if err == gorm.ErrRecordNotFound {
			middleware.Fail(c, http.StatusNotFound, 2008, "云变量不存在")
			return
		}
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5005, "查询失败")
			return
		}

		// 云变量直接返回（model 无 Encrypted 字段，全部明文存储）
		value := v.VarValue

		middleware.Success(c, gin.H{
			"var_key":    v.VarKey,
			"var_value":  value,
			"var_type":   v.VarType,
			"updated_at": v.UpdatedAt.Unix(),
		})
	}
}

// ============== ClientNotice 获取公告 ==============

// ClientNotice 获取应用公告
// POST /api/v1/client/notice
func ClientNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		app := c.MustGet("app").(*model.App)
		// 查应用公告（type=app AND status=published）
		var notices []model.Notice
		deps.DB.Where("app_id = ? AND type = ? AND status = ?", app.ID, "app", "published").
			Order("is_pinned DESC, id DESC").Limit(5).Find(&notices)

		result := make([]gin.H, 0, len(notices))
		for _, n := range notices {
			result = append(result, gin.H{
				"id":         n.ID,
				"title":      n.Title,
				"content":    n.Content,
				"is_pinned":  n.IsPinned,
				"created_at": n.CreatedAt.Unix(),
			})
		}
		middleware.Success(c, gin.H{"notices": result})
	}
}

// ============== ClientVersion 检查版本 ==============

// ClientVersion 检查版本更新
// POST /api/v1/client/version
func ClientVersion(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			CurrentVersion string `json:"current_version"`
			Platform       string `json:"platform"` // windows/macos/linux/android/ios
		}
		_ = c.ShouldBindJSON(&req)
		app := c.MustGet("app").(*model.App)

		// 查最新版本
		var ver model.AppVersion
		err := deps.DB.Where("app_id = ? AND status = ?", app.ID, "active").
			Order("id DESC").First(&ver).Error
		if err == gorm.ErrRecordNotFound {
			middleware.Success(c, gin.H{
				"has_update":    false,
				"force_update":  false,
				"message":       "暂无版本信息",
			})
			return
		}
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5006, "查询版本失败")
			return
		}

		// 比较版本号（简化：字符串比较，待核实：建议改用 semver 比较）
		hasUpdate := req.CurrentVersion == "" || ver.Version != req.CurrentVersion
		middleware.Success(c, gin.H{
			"has_update":         hasUpdate,
			"force_update":       ver.ForceUpdate,
			"latest_version":     ver.Version,
			"current_version":    req.CurrentVersion,
			"download_url":       ver.DownloadURL,
			"backup_url":         ver.BackupURL,
			"update_description": ver.UpdateContent,
			"min_version":        ver.MinVersion,
			"released_at":        ver.CreatedAt.Unix(),
		})
	}
}

// ============== 辅助：写验证日志 ==============

// writeVerifyLog 写入验证日志（v0.3.3 起改为异步队列，不阻塞验证 API）
// LogVerify 表无 CardKey/HWID/Message 字段，相关信息通过 Action/Result/Extra 表达
// 注：调用方需通过 c 传递 IP/UA；本函数从 c.MustGet("app") 取 app 之外，从 c 读取客户端信息
func writeVerifyLog(deps *Deps, app *model.App, hwid, cardKey, action, result, message string) {
	writeVerifyLogCtx(deps, nil, app, hwid, cardKey, action, result, message)
}

// writeVerifyLogCtx 带 gin.Context 的验证日志写入（v0.3.3 推荐使用）
func writeVerifyLogCtx(deps *Deps, c *gin.Context, app *model.App, hwid, cardKey, action, result, message string) {
	ip, ua := "", ""
	if c != nil {
		ip = c.ClientIP()
		ua = c.Request.UserAgent()
	}
	extra := map[string]interface{}{
		"card_key": cardKey,
		"hwid":     hwid,
		"message":  message,
	}
	enqueueVerifyLog(deps, app.TenantID, app.ID, nil, nil, action, result, ip, ua, extra)
}

// 标记未使用导入
var _ = context.Background
var _ = json.Marshal
