// 会话与登录审计辅助
// v0.3.1：基于 refresh_token_device 表实现 ListLoginDevices / KickDevice
//        + log_login_failed 表实现安全统计
// 严格遵循铁律 04/05/06
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/your-org/keyauth-saas/apps/server/internal/auth"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/ua"
)

// ============== 会话记录 ==============

// recordLoginSession 登录成功后写入一条 refresh_token_device 记录
// refresh_jti 使用 uuid 作为会话标识（当前 JWT 不携带 jti，待核实 v0.4.x 将 jti 嵌入 claims 后即可精准单点踢出）
func recordLoginSession(deps *Deps, role string, userID uint64, ip, uaStr string, refreshTTL time.Duration) error {
	now := time.Now()
	expiresAt := now.Add(refreshTTL)
	// v0.4.x：使用 pkg/ua 统一解析（一次解析复用结果，避免重复扫描 UA）
	info := ua.Parse(uaStr)
	session := &model.RefreshTokenDevice{
		UserRole:     role,
		UserID:       userID,
		RefreshJTI:   uuid.NewString(),
		DeviceName:   info.DeviceName,
		DeviceType:   info.DeviceType,
		ClientIP:     ip,
		UserAgent:    truncateUA(uaStr),
		LastActiveAt: now,
		ExpiresAt:    expiresAt,
		Revoked:      false,
	}
	return deps.DB.Create(session).Error
}

// touchSessionActive 更新会话最近活跃时间（在 JWT 中间件中按需调用，v0.3.1 暂不接入以避免性能损耗）
func touchSessionActive(deps *Deps, role string, userID uint64) {
	now := time.Now()
	deps.DB.Model(&model.RefreshTokenDevice{}).
		Where("user_role = ? AND user_id = ? AND revoked = 0 AND expires_at > ?",
			role, userID, now).
		Update("last_active_at", now)
}

// markAllSessionsRevoked 将某用户的所有未过期会话标记为已撤销
// 用于 Logout / ChangePassword / Disable2FA 场景
func markAllSessionsRevoked(deps *Deps, role string, userID uint64) {
	now := time.Now()
	deps.DB.Model(&model.RefreshTokenDevice{}).
		Where("user_role = ? AND user_id = ? AND revoked = 0", role, userID).
		Updates(map[string]interface{}{
			"revoked":     true,
			"revoked_at":  now,
		})
}

// detectDeviceType 设备类型判定（v0.4.x 改为调用 pkg/ua）
// 保留此函数作为 handler 层包装，兼容既有调用点
func detectDeviceType(uaStr string) string {
	return ua.Parse(uaStr).DeviceType
}

// truncateUA 截断 User-Agent 到 512 字符（数据库字段长度限制）
func truncateUA(ua string) string {
	if len(ua) > 512 {
		return ua[:512]
	}
	return ua
}

// ============== 登录失败日志 ==============

// recordLoginFailureDB 登录失败时同步写入 log_login_failed 表
// 异步写入以避免阻塞登录响应（channel 容量 1024，超出后丢弃以保证登录可用性）
var loginFailureCh = make(chan *model.LogLoginFailed, 1024)

// StartLoginFailureWorker 启动后台 goroutine 消费登录失败日志
// 应在 main.go 启动时调用一次
func StartLoginFailureWorker(deps *Deps) {
	go func() {
		for log := range loginFailureCh {
			// 单条失败不影响主流程，仅记录错误
			if err := deps.DB.Create(log).Error; err != nil {
				// 待核实 v0.4.x：引入结构化日志记录此错误
				_ = err
			}
		}
	}()
}

// recordLoginFailureAsync 异步写入登录失败日志
func recordLoginFailureAsync(deps *Deps, role, username, ip, ua, reason string) {
	log := &model.LogLoginFailed{
		UserType:  role,
		Username:  username,
		ClientIP:  ip,
		Reason:    reason,
		UserAgent: truncateUA(ua),
		CreatedAt: time.Now(),
	}
	select {
	case loginFailureCh <- log:
	default:
		// 队列满则丢弃（保证登录主流程可用）
	}
}

// ============== 安全统计 ==============

// securityFailedLoginToday 今日登录失败次数（按 IP 维度或全量）
func securityFailedLoginToday(deps *Deps, ip string) int64 {
	todayStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(),
		0, 0, 0, 0, time.Now().Location())
	q := deps.DB.Model(&model.LogLoginFailed{}).Where("created_at >= ?", todayStart)
	if ip != "" {
		q = q.Where("client_ip = ?", ip)
	}
	var count int64
	q.Count(&count)
	return count
}

// securityBlockedIPsToday 今日被自动封禁 IP 数（基于 sec_ip_blacklist source=auto 且今日创建）
func securityBlockedIPsToday(deps *Deps) int64 {
	todayStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(),
		0, 0, 0, 0, time.Now().Location())
	var count int64
	deps.DB.Model(&model.SecIPBlacklist{}).
		Where("source = ? AND created_at >= ?", "auto", todayStart).
		Count(&count)
	return count
}

// ============== HTTP Handler（供 router 直接注册） ==============

// ListLoginDevicesFull 完整版登录设备列表（基于 refresh_token_device 表）
// GET /{role}/auth/devices
func ListLoginDevicesFull(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		now := time.Now()
		var sessions []model.RefreshTokenDevice
		if err := deps.DB.
			Where("user_role = ? AND user_id = ? AND revoked = 0 AND expires_at > ?",
				role, userID, now).
			Order("last_active_at DESC").
			Find(&sessions).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询登录设备失败: "+err.Error())
			return
		}

		// 标记当前请求所在设备（按 IP + User-Agent 匹配，简化判定）
		currentIP := c.ClientIP()
		currentUA := truncateUA(c.Request.Header.Get("User-Agent"))

		list := make([]gin.H, 0, len(sessions))
		for _, s := range sessions {
			isCurrent := s.ClientIP == currentIP && s.UserAgent == currentUA
			// v0.4.x：动态解析 UA 拆分字段（不改 DB schema，向前兼容）
			info := ua.Parse(s.UserAgent)
			list = append(list, gin.H{
				"id":             s.ID,
				"device_id":      s.RefreshJTI,
				"device_name":    s.DeviceName,
				"device_type":    s.DeviceType,
				"os":             info.OS,
				"os_version":     info.OSVersion,
				"browser":        info.Browser,
				"browser_version": info.Version,
				"is_bot":         info.DeviceType == ua.DeviceBot,
				"ip":             s.ClientIP,
				"location":       "", // 待核实 v0.4.x：接入 IP 地理库
				"user_agent":     s.UserAgent,
				"last_active_at": s.LastActiveAt,
				"created_at":     s.CreatedAt,
				"expires_at":     s.ExpiresAt,
				"current":        isCurrent,
			})
		}

		middleware.Success(c, gin.H{
			"list":  list,
			"total": len(list),
		})
	}
}

// KickDeviceFull 完整版踢设备下线
// POST /{role}/auth/devices/:id/kick
// 注：当前 JWT 不携带 jti，单点踢出会同时黑名单该用户所有 refresh token
//     已知限制 v0.4.x：将 jti 嵌入 JWT claims 后实现精准单点踢出
func KickDeviceFull(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		deviceID, err := parseUintParam(c, "id")
		if err != nil || deviceID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "设备 ID 参数错误")
			return
		}

		// 1. 校验会话归属当前用户
		var session model.RefreshTokenDevice
		if err := deps.DB.
			Where("id = ? AND user_role = ? AND user_id = ?", deviceID, role, userID).
			First(&session).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "设备记录不存在或无权操作")
			return
		}

		// 2. 标记会话为已撤销
		now := time.Now()
		if err := deps.DB.Model(&session).Updates(map[string]interface{}{
			"revoked":    true,
			"revoked_at": now,
		}).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "撤销会话失败: "+err.Error())
			return
		}

		// 3. 黑名单当前用户 refresh token（已知限制：会踢出该用户所有设备）
		//    待核实 v0.4.x：将 jti 嵌入 JWT 后改为只黑名单指定 jti
		params := loadAuthParams(deps, role)
		_ = auth.BlacklistRefreshToken(deps.Redis, userID, role, params.RefreshTTL)

		// 4. 异步清理该用户其他已过期会话标记（避免列表累积）
		go func(deps *Deps, role string, userID uint64) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = ctx
			deps.DB.Model(&model.RefreshTokenDevice{}).
				Where("user_role = ? AND user_id = ? AND (expires_at < ? OR revoked = 1)",
					role, userID, time.Now()).
				Update("revoked", true)
		}(deps, role, userID)

		middleware.Success(c, gin.H{
			"device_id":    deviceID,
			"kicked":       true,
			"note":         "当前实现会同时踢出该用户所有设备，待 v0.4.x 支持精准单点踢出",
		})
	}
}
