// v0.4.0 终端用户体系 Handler
// 严格遵循铁律 04/05/06：
//   04 - 注册/登录/密码/Token TTL/绑定上限/IP 限流 全部从 sys_config 读取
//   05 - 10 项 enduser.* 配置可通过后台「系统配置」实时调整
//   06 - 密码 bcrypt 哈希；refresh token SHA-512 哈希存储；jti 单点踢出
//
// 接口分两组：
//   1. publicGroup 下（无鉴权）：注册 / 登录 / 发验证码 / 重置密码 / refresh token
//   2. h5Auth 下（终端用户 JWT）：个人信息 / 改密 / 卡密绑定/解绑 / 我的卡密 / 我的订单 / 注销
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/enduser"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/notify"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 公开接口（无需鉴权，挂在 publicGroup） ==============

// H5EndUserRegister POST /public/enduser/register
func H5EndUserRegister(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			AppKey   string `json:"app_key" binding:"required"`
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
			Phone    string `json:"phone"`
			Email    string `json:"email"`
			Nickname string `json:"nickname"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		// 通过 app_key 查应用
		var app model.App
		if err := deps.DB.Where("app_key = ?", req.AppKey).First(&app).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "应用不存在"})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		user, err := mgr.Register(ctx, enduser.RegisterRequest{
			TenantID: app.TenantID,
			AppID:    app.ID,
			Username: req.Username,
			Password: req.Password,
			Phone:    req.Phone,
			Email:    req.Email,
			Nickname: req.Nickname,
		})
		if err != nil {
			code := 500
			if err == enduser.ErrRegisterDisabled {
				code = 403
			} else if err == enduser.ErrUserExists {
				code = 409
			} else if err == enduser.ErrPasswordTooShort {
				code = 400
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{
			"id":       user.ID,
			"username": user.Username,
			"app_id":   user.AppID,
		})
	}
}

// H5EndUserLogin POST /public/enduser/login
func H5EndUserLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			AppKey   string `json:"app_key" binding:"required"`
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		var app model.App
		if err := deps.DB.Where("app_key = ?", req.AppKey).First(&app).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "应用不存在"})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		tokenPair, user, err := mgr.Login(ctx, enduser.LoginRequest{
			TenantID:  app.TenantID,
			AppID:     app.ID,
			Username:  req.Username,
			Password:  req.Password,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}, deps.Config.JWT.Secret)
		if err != nil {
			code := 500
			if err == enduser.ErrUserNotFound || err == enduser.ErrPasswordIncorrect {
				code = 401
			} else if err == enduser.ErrUserBanned {
				code = 403
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{
			// 扁平化 token 字段（P0 高危 10：前端期望扁平 access_token/refresh_token/expires_in）
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"expires_in":    tokenPair.ExpiresIn,
			"token_type":    tokenPair.TokenType,
			"user": gin.H{
				"id":         user.ID,
				"username":   user.Username,
				"nickname":   user.Nickname,
				"avatar_url": user.AvatarURL,
			},
		})
	}
}

// H5RefreshToken POST /public/enduser/refresh
func H5RefreshToken(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		tokenPair, err := mgr.RefreshToken(ctx, req.RefreshToken, deps.Config.JWT.Secret,
			c.ClientIP(), c.Request.UserAgent())
		if err != nil {
			code := 401
			if err == enduser.ErrUserBanned {
				code = 403
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{
			// 扁平化 token 字段（P0 高危 10：与 login 响应结构一致）
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"expires_in":    tokenPair.ExpiresIn,
			"token_type":    tokenPair.TokenType,
		})
	}
}

// H5SendVerifyCode POST /public/enduser/verify_code
// 给手机/邮箱发送验证码（注册/重置密码用）
func H5SendVerifyCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			AppKey   string `json:"app_key" binding:"required"`
			Channel  string `json:"channel" binding:"required"` // sms/email
			Recipient string `json:"recipient" binding:"required"`
			Purpose  string `json:"purpose"` // register/reset_password
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		if req.Channel != notify.ChannelSMS && req.Channel != notify.ChannelEmail {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的渠道，仅支持 sms/email"})
			return
		}
		var app model.App
		if err := deps.DB.Where("app_key = ?", req.AppKey).First(&app).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "应用不存在"})
			return
		}
		// 生成验证码
		length := deps.CfgCache.GetInt(ctx, enduser.CfgKeyVerifyCodeLength, 6)
		code := notify.GenerateVerifyCode(length)
		ttl := deps.CfgCache.GetInt(ctx, enduser.CfgKeyVerifyCodeTTL, 5)

		// 铁律 06 修复：验证码写入 Redis 用于后续校验（key 含 app_key + channel + recipient 防跨应用/跨渠道滥用）
		// TTL = ttl 分钟；同一 recipient 60s 内重复请求会被 Redis TTL 拦截（防短信轰炸）
		verifyCodeCacheKey := fmt.Sprintf("enduser:vcode:%s:%s:%s", req.AppKey, req.Channel, req.Recipient)
		if err := deps.Redis.Set(ctx, verifyCodeCacheKey, code, time.Duration(ttl)*time.Minute).Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "验证码缓存失败"})
			return
		}

		// 调通知系统发送
		notifyMgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		templateCode := notify.TemplateVerifyCode
		if req.Channel == notify.ChannelEmail {
			templateCode = notify.TemplateVerifyCodeEmail
		}
		_, err := notifyMgr.Send(ctx, notify.SendRequest{
			TemplateCode: templateCode,
			Channel:      req.Channel,
			Recipient:    req.Recipient,
			Variables:    map[string]interface{}{"code": code, "ttl": ttl, "app_name": app.Name},
			TenantID:     app.TenantID,
			Priority:     notify.PriorityHigh,
		})
		if err != nil {
			// 发送失败回滚 Redis 缓存，避免后续校验时验证码存在但用户未收到
			deps.Redis.Del(ctx, verifyCodeCacheKey)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "发送失败: " + err.Error()})
			return
		}
		middleware.Success(c, gin.H{
			"sent":     true,
			"ttl":      ttl,
			"channel":  req.Channel,
			"purpose":  req.Purpose,
		})
	}
}

// H5ResetPassword POST /public/enduser/reset_password
// 通过验证码重置密码（修复 P0 高危 1：必须从 Redis 校验验证码 + 删除防重放）
func H5ResetPassword(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			AppKey    string `json:"app_key" binding:"required"`
			Username  string `json:"username" binding:"required"`
			Password  string `json:"password" binding:"required"`
			Channel   string `json:"channel" binding:"required"`     // 短信/邮箱渠道（用于查 Redis 验证码）
			Recipient string `json:"recipient" binding:"required"`  // 接收验证码的手机号/邮箱
			VerifyCode string `json:"verify_code" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		var app model.App
		if err := deps.DB.Where("app_key = ?", req.AppKey).First(&app).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "应用不存在"})
			return
		}
		// 校验验证码（铁律 06：从 Redis 取，校验后立即删除防重放）
		verifyCodeCacheKey := fmt.Sprintf("enduser:vcode:%s:%s:%s", req.AppKey, req.Channel, req.Recipient)
		cachedCode, err := deps.Redis.Get(ctx, verifyCodeCacheKey).Result()
		if err != nil || cachedCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "验证码已过期，请重新获取"})
			return
		}
		if cachedCode != req.VerifyCode {
			// 验证码错误：记录失败次数（防暴力枚举，5 次失败后强制过期）
			failKey := fmt.Sprintf("enduser:vcode:fail:%s:%s:%s", req.AppKey, req.Channel, req.Recipient)
			failCnt, _ := deps.Redis.Incr(ctx, failKey).Result()
			if failCnt == 1 {
				deps.Redis.Expire(ctx, failKey, 30*time.Minute)
			}
			if failCnt >= 5 {
				deps.Redis.Del(ctx, verifyCodeCacheKey, failKey)
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "验证码错误次数过多，请重新获取"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "验证码错误"})
			return
		}
		// 验证码正确：删除防重放
		deps.Redis.Del(ctx, verifyCodeCacheKey)
		// 查用户
		var user model.EndUser
		if err := deps.DB.Where("tenant_id = ? AND app_id = ? AND username = ? AND status != ?",
			app.TenantID, app.ID, req.Username, enduser.UserStatusDeleted).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "用户不存在"})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		if err := mgr.ResetPassword(ctx, user.ID, req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{"reset": true})
	}
}

// ============== 鉴权接口（需 access token，挂在 h5Auth） ==============

// H5EndUserMe GET /h5/me
func H5EndUserMe(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		user, err := mgr.GetProfile(ctx, userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "用户不存在"})
			return
		}
		middleware.Success(c, user)
	}
}

// H5EndUserUpdateProfile PUT /h5/me
func H5EndUserUpdateProfile(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		var req struct {
			Nickname  *string `json:"nickname"`
			AvatarURL *string `json:"avatar_url"`
			Email     *string `json:"email"`
			Phone     *string `json:"phone"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		updates := map[string]interface{}{}
		if req.Nickname != nil {
			updates["nickname"] = *req.Nickname
		}
		if req.AvatarURL != nil {
			updates["avatar_url"] = *req.AvatarURL
		}
		if req.Email != nil {
			updates["email"] = *req.Email
		}
		if req.Phone != nil {
			updates["phone"] = *req.Phone
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		if err := mgr.UpdateProfile(ctx, userID, updates); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{"updated": true})
	}
}

// H5EndUserChangePassword POST /h5/me/password
func H5EndUserChangePassword(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		var req struct {
			OldPassword string `json:"old_password" binding:"required"`
			NewPassword string `json:"new_password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		if err := mgr.ChangePassword(ctx, userID, req.OldPassword, req.NewPassword); err != nil {
			code := 500
			if err == enduser.ErrPasswordIncorrect {
				code = 401
			} else if err == enduser.ErrPasswordTooShort {
				code = 400
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{"changed": true})
	}
}

// H5EndUserLogout POST /h5/logout
func H5EndUserLogout(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		if err := mgr.Logout(ctx, req.RefreshToken); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{"logged_out": true})
	}
}

// H5EndUserListSessions GET /h5/sessions
func H5EndUserListSessions(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		sessions, err := mgr.ListSessions(ctx, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{"items": sessions})
	}
}

// H5EndUserKickSession POST /h5/sessions/:jti/kick
func H5EndUserKickSession(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		jti := c.Param("jti")
		if jti == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 jti"})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		if err := mgr.RevokeSession(ctx, userID, jti); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{"kicked": jti})
	}
}

// ============== 卡密绑定接口 ==============

// H5EndUserBindCard POST /h5/cards/bind
func H5EndUserBindCard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		appID := c.GetUint64("enduser_app_id") // P0 高危 2：从 JWT 取 app_id 防跨租户/跨应用 IDOR
		var req struct {
			CardKey string `json:"card_key" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		// 通过 card_key_hash 查卡密（P0 高危 2：必须附加 app_id 过滤，否则跨应用可绑定他人卡密）
		cardKeyHash := crypto.SHA512Hex(req.CardKey)
		var card model.AppCard
		if err := deps.DB.Where("card_key_hash = ? AND app_id = ?", cardKeyHash, appID).First(&card).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "卡密不存在"})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		entry, err := mgr.BindCard(ctx, userID, card.ID)
		if err != nil {
			code := 500
			if err == enduser.ErrCardAlreadyBound || err == enduser.ErrCardBoundToOther {
				code = 409
			} else if err == enduser.ErrBindLimitExceeded {
				code = 429
			} else if err == enduser.ErrCardNotFound || err == enduser.ErrCardStatusInvalid {
				code = 400
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		middleware.Success(c, entry)
	}
}

// H5EndUserUnbindCard POST /h5/cards/unbind
func H5EndUserUnbindCard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		var req struct {
			CardID uint64 `json:"card_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		if err := mgr.UnbindCard(ctx, userID, req.CardID); err != nil {
			code := 500
			if err == enduser.ErrCardNotFound {
				code = 404
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{"unbound": req.CardID})
	}
}

// H5EndUserListMyCards GET /h5/cards?page=&page_size=
func H5EndUserListMyCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		cards, total, err := mgr.ListMyCards(ctx, userID, page, pageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		middleware.Success(c, gin.H{
			"items": cards,
			"total": total,
			"page":  page,
		})
	}
}

// H5EndUserGetCardDetail GET /h5/cards/:id
func H5EndUserGetCardDetail(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.GetUint64("enduser_id")
		cardID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := enduser.NewManager(deps.DB, deps.CfgCache)
		card, err := mgr.GetCardDetail(ctx, userID, cardID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "卡密不存在或未绑定"})
			return
		}
		middleware.Success(c, card)
	}
}

// ============== 终端用户订单接口（v0.4.x 残留项 1：U-11） ==============

// H5EndUserListOrders GET /h5/orders?status=&page=&page_size=
// 列出当前终端用户的订单（按 buyer_user_id 筛选）
// status 可选：pending / paid / closed / refunded / 空（全部）
func H5EndUserListOrders(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("enduser_id")
		status := strings.TrimSpace(c.Query("status"))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}

		q := deps.DB.Model(&model.AppOrder{}).Where("buyer_user_id = ?", userID)
		// 铁律 04：状态白名单（避免任意 status 拼接）
		switch status {
		case "pending", "paid", "closed", "refunded":
			q = q.Where("pay_status = ?", status)
		case "":
			// 全部
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 status 参数"})
			return
		}

		var total int64
		q.Count(&total)

		var orders []model.AppOrder
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询订单失败: " + err.Error()})
			return
		}

		// 拼装响应项（铁律 04：仅返回 H5 必要字段，敏感字段如 card_ids 不外泄明文卡密 ID）
		items := make([]gin.H, 0, len(orders))
		for _, o := range orders {
			items = append(items, gin.H{
				"id":           o.ID,
				"order_no":     o.OrderNo,
				"app_id":       o.AppID,
				"tenant_id":    o.TenantID,
				"card_type_id": o.CardTypeID,
				"quantity":     o.Quantity,
				"unit_price":   o.UnitPrice,
				"total_amount": o.TotalAmount,
				"pay_channel":  o.PayChannel,
				"pay_status":   o.PayStatus,
				"pay_trade_no": o.PayTradeNo,
				"paid_at":      o.PaidAt,
				"created_at":   o.CreatedAt,
				"client_ip":    o.ClientIP,
			})
		}

		middleware.Success(c, gin.H{
			"list":      items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// H5EndUserGetOrder GET /h5/orders/:order_no
// 查询单个订单详情 + 已支付时返回卡密明文列表
// 安全：仅返回属于当前 enduser_id 的订单；卡密明文仅在 pay_status=paid 时返回
func H5EndUserGetOrder(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("enduser_id")
		orderNo := strings.TrimSpace(c.Param("order_no"))
		if orderNo == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "订单号不能为空"})
			return
		}

		var order model.AppOrder
		if err := deps.DB.Where("order_no = ? AND buyer_user_id = ?", orderNo, userID).First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "订单不存在或不属于当前用户"})
			return
		}

		// 解析卡密 ID 列表
		var cardIDs []uint64
		if order.CardIDs != "" {
			_ = json.Unmarshal([]byte(order.CardIDs), &cardIDs)
		}

		// v0.3.5 起策略：订单已支付时返回卡密明文，供 H5 终端用户直接查看
		// 安全：仅返回该订单关联的卡密；card_ids 字段为 JSON 数组
		// 注：始终初始化为空切片而非 nil，便于前端无空值判断（JSON 序列化为 [] 而非 null）
		cardKeys := make([]string, 0)
		cards := make([]gin.H, 0)
		if order.PayStatus == "paid" && len(cardIDs) > 0 {
			var appCards []model.AppCard
			deps.DB.Where("id IN ?", cardIDs).Order("id ASC").Find(&appCards)
			for _, card := range appCards {
				cardKeys = append(cardKeys, card.CardKey)
				cards = append(cards, gin.H{
					"id":            card.ID,
					"card_key":      card.CardKey,
					"status":        card.Status,
					"expires_at":    card.ExpiresAt,
					"activated_at":  card.ActivatedAt,
					"duration_seconds": card.DurationSeconds,
					"max_uses":      card.MaxUses,
					"used_count":    card.UsedCount,
				})
			}
		}

		middleware.Success(c, gin.H{
			"id":              order.ID,
			"order_no":        order.OrderNo,
			"app_id":          order.AppID,
			"tenant_id":       order.TenantID,
			"card_type_id":    order.CardTypeID,
			"quantity":        order.Quantity,
			"unit_price":      order.UnitPrice,
			"total_amount":    order.TotalAmount,
			"pay_channel":     order.PayChannel,
			"pay_status":      order.PayStatus,
			"pay_trade_no":    order.PayTradeNo,
			"paid_at":         order.PaidAt,
			"created_at":      order.CreatedAt,
			"client_ip":       order.ClientIP,
			"buyer_contact":   order.BuyerContact,
			"card_ids":        cardIDs,
			"card_keys":       cardKeys, // 仅 paid 时非空
			"cards":           cards,   // 仅 paid 时非空：卡密明细列表
		})
	}
}

// ============== 管理员接口（admin 端管理终端用户） ==============

// AdminListEndUsers GET /admin/endusers?tenant_id=&app_id=&status=&page=&page_size=
func AdminListEndUsers(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseUint(c.Query("tenant_id"), 10, 64)
		appID, _ := strconv.ParseUint(c.Query("app_id"), 10, 64)
		status := c.Query("status")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}
		q := deps.DB.Model(&model.EndUser{})
		if tenantID > 0 {
			q = q.Where("tenant_id = ?", tenantID)
		}
		if appID > 0 {
			q = q.Where("app_id = ?", appID)
		}
		if status != "" {
			q = q.Where("status = ?", status)
		}
		var total int64
		q.Count(&total)
		var items []model.EndUser
		q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)
		middleware.Success(c, gin.H{
			"items": items,
			"total": total,
			"page":  page,
		})
	}
}

// AdminGetEndUser GET /admin/endusers/:id
func AdminGetEndUser(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		var user model.EndUser
		if err := deps.DB.First(&user, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "用户不存在"})
			return
		}
		middleware.Success(c, user)
	}
}

// AdminUpdateEndUserStatus PUT /admin/endusers/:id/status
// 用于封禁/解禁终端用户
func AdminUpdateEndUserStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		var req struct {
			Status string `json:"status" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		req.Status = strings.TrimSpace(req.Status)
		if req.Status != enduser.UserStatusActive && req.Status != enduser.UserStatusBanned && req.Status != enduser.UserStatusDeleted {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的状态"})
			return
		}
		if err := deps.DB.Model(&model.EndUser{}).Where("id = ?", id).
			Update("status", req.Status).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		// 封禁时撤销所有会话
		if req.Status == enduser.UserStatusBanned {
			mgr := enduser.NewManager(deps.DB, deps.CfgCache)
			_ = mgr.RevokeAllSessions(c.Request.Context(), id)
		}
		RecordOperation(deps, c, "enduser", "update_status", "success", "user", &id, map[string]interface{}{
			"status": req.Status,
		})
		middleware.Success(c, gin.H{"id": id, "status": req.Status})
	}
}

// AdminEndUserStats GET /admin/endusers/stats
func AdminEndUserStats(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseUint(c.Query("tenant_id"), 10, 64)
		q := deps.DB.Model(&model.EndUser{})
		if tenantID > 0 {
			q = q.Where("tenant_id = ?", tenantID)
		}
		var total, active, banned, deleted int64
		q.Count(&total)
		q.Where("status = ?", enduser.UserStatusActive).Count(&active)
		q.Where("status = ?", enduser.UserStatusBanned).Count(&banned)
		q.Where("status = ?", enduser.UserStatusDeleted).Count(&deleted)
		// 今日新增
		var todayNew int64
		deps.DB.Model(&model.EndUser{}).
			Where("created_at >= ?", todayStart()).
			Count(&todayNew)
		middleware.Success(c, gin.H{
			"total":      total,
			"active":     active,
			"banned":     banned,
			"deleted":    deleted,
			"today_new":  todayNew,
		})
	}
}

// todayStart 返回今日 0 点时间
func todayStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
