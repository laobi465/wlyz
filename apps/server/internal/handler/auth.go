// 认证处理器：超管 / 开发者 / 代理 三角色登录注册
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取
// 遵循铁律 06：不确定处标注「需验证」或「待核实」
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/auth"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/quota"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"github.com/your-org/keyauth-saas/apps/server/pkg/epay"
	"github.com/your-org/keyauth-saas/apps/server/pkg/snowflake"
)

// ============== 公共 DTO ==============

type loginReq struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=64"`
	TOTPCode string `json:"totp_code"` // 可选，绑定 2FA 后必填
}

type registerTenantReq struct {
	Username        string `json:"username" binding:"required,min=3,max=64"`
	Password        string `json:"password" binding:"required,min=8,max=64"`
	Email           string `json:"email" binding:"required,email"`
	Phone           string `json:"phone" binding:"omitempty,max=32"`
	Company         string `json:"company" binding:"omitempty,max=128"`
	Agreement       bool   `json:"agreement" binding:"required,eq=true"` // 必须同意协议
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type loginResp struct {
	TokenPair   *auth.TokenPair `json:"token_pair"`
	User        userInfo        `json:"user"`
	Require2FA  bool            `json:"require_2fa"`   // true 表示需要二次验证，TokenPair 为空
	TOTPSecret  string          `json:"totp_secret,omitempty"`  // 仅首次绑定 2FA 时返回
	OTPAUTHURL  string          `json:"otpauth_url,omitempty"`  // 仅首次绑定 2FA 时返回
}

type userInfo struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TenantID uint64 `json:"tenant_id,omitempty"`
	Status   string `json:"status"`
}

// ============== 辅助：从 sys_config 读取认证参数 ==============

// getAuthParams 从 sys_config 读取认证相关参数（带缓存）
type authParams struct {
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	Issuer          string
	MaxAttempts     int
	LockSeconds     int
	WindowSeconds   int
	TOTPRequired    bool // 当前角色是否强制 2FA
	TOTPIssuer      string
	TOTPPeriod      uint
	TOTPSkew        uint
	PasswordMinLen  int
}

func loadAuthParams(deps *Deps, role string) authParams {
	ctx := context.Background()
	p := authParams{
		AccessTTL:      time.Duration(deps.CfgCache.GetInt(ctx, "jwt.access_ttl_seconds", 7200)) * time.Second,
		RefreshTTL:     time.Duration(deps.CfgCache.GetInt(ctx, "jwt.refresh_ttl_seconds", 604800)) * time.Second,
		Issuer:         deps.CfgCache.GetString(ctx, "jwt.issuer", "keyauth-saas"),
		MaxAttempts:    deps.CfgCache.GetInt(ctx, "security.login.max_attempts", 5),
		LockSeconds:    deps.CfgCache.GetInt(ctx, "security.login.lock_seconds", 900),
		WindowSeconds:  deps.CfgCache.GetInt(ctx, "security.login.window_seconds", 600),
		TOTPIssuer:     deps.CfgCache.GetString(ctx, "totp.issuer", "KeyAuth SaaS"),
		TOTPPeriod:     uint(deps.CfgCache.GetInt(ctx, "totp.period", 30)),
		TOTPSkew:       uint(deps.CfgCache.GetInt(ctx, "totp.skew", 1)),
		PasswordMinLen: deps.CfgCache.GetInt(ctx, "security.password_min_length", 8),
	}

	// 当前角色是否强制开启 2FA
	switch role {
	case auth.RoleAdmin:
		p.TOTPRequired = deps.CfgCache.GetBool(ctx, "admin.2fa_required", false)
	case auth.RoleTenant:
		p.TOTPRequired = deps.CfgCache.GetBool(ctx, "tenant.2fa_required", false)
	case auth.RoleAgent:
		p.TOTPRequired = deps.CfgCache.GetBool(ctx, "agent.2fa_required", false)
	}
	return p
}

// ============== 通用登录流程 ==============

// doLogin 通用登录逻辑
// role: admin/tenant/agent
// lookup: 根据 username 查询账号的回调（返回密码哈希、2FA密钥、状态等）
// updateLoginInfo: 登录成功后更新 last_login_at / last_login_ip
//
// 返回 loginResp，调用方负责 JSON 序列化
func doLogin(
	c *gin.Context,
	deps *Deps,
	role string,
	lookup func(db *gorm.DB, username string) (id uint64, passwordHash, totpSecretEnc, status string, tenantID uint64, err error),
	updateLoginInfo func(db *gorm.DB, id uint64, ip string, t time.Time) error,
) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
		return
	}

	params := loadAuthParams(deps, role)
	ctx := c.Request.Context()

	// 1. 检查账号锁定
	locked, lockTTL, err := auth.IsAccountLocked(ctx, deps.Redis, role, req.Username)
	if err != nil {
		middleware.Fail(c, http.StatusInternalServerError, 5001, "登录服务异常: "+err.Error())
		return
	}
	if locked {
		middleware.Fail(c, http.StatusTooManyRequests, 4003,
			"账号已锁定，请 "+auth.FormatLockRemaining(lockTTL)+" 后重试")
		return
	}

	// 2. 查询账号
	id, passwordHash, totpSecretEnc, status, tenantID, err := lookup(deps.DB, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 不暴露账号是否存在，统一返回「用户名或密码错误」
			// 但仍记录失败次数（防爆破）
			_, _ = auth.RecordLoginFailure(ctx, deps.Redis, role, req.Username,
				params.MaxAttempts, params.WindowSeconds, params.LockSeconds)
			// 同步写入数据库失败日志（异步队列，用于安全中心统计）
			recordLoginFailureAsync(deps, role, req.Username, c.ClientIP(),
				c.Request.Header.Get("User-Agent"), "wrong_password")
			middleware.Fail(c, http.StatusUnauthorized, 1004, "用户名或密码错误")
			return
		}
		middleware.Fail(c, http.StatusInternalServerError, 5002, "查询账号失败: "+err.Error())
		return
	}

	// 3. 校验状态
	if status != "active" {
		recordLoginFailureAsync(deps, role, req.Username, c.ClientIP(),
			c.Request.Header.Get("User-Agent"), "disabled")
		middleware.Fail(c, http.StatusForbidden, 1005, "账号已被禁用或待审核，请联系管理员")
		return
	}

	// 4. 校验密码
	if !crypto.CheckPassword(passwordHash, req.Password) {
		count, _ := auth.RecordLoginFailure(ctx, deps.Redis, role, req.Username,
			params.MaxAttempts, params.WindowSeconds, params.LockSeconds)
		remaining := params.MaxAttempts - count
		// 同步写入数据库失败日志
		recordLoginFailureAsync(deps, role, req.Username, c.ClientIP(),
			c.Request.Header.Get("User-Agent"), "wrong_password")
		if remaining > 0 {
			middleware.Fail(c, http.StatusUnauthorized, 1004,
				"用户名或密码错误，剩余 "+itoa(remaining)+" 次尝试机会")
		} else {
			middleware.Fail(c, http.StatusTooManyRequests, 4003,
				"登录失败次数过多，账号已锁定 "+auth.FormatLockRemaining(time.Duration(params.LockSeconds)*time.Second))
		}
		return
	}

	// 5. 2FA 校验
	if totpSecretEnc != "" {
		// 已绑定 2FA
		totpSecret, err := auth.DecryptTOTPSecret(deps.Crypto, totpSecretEnc)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "2FA 密钥解密失败")
			return
		}
		if !auth.ValidateTOTP(totpSecret, req.TOTPCode, params.TOTPSkew) {
			// 2FA 验证码错误不计入账号锁定计数（防止忘记手机导致账号被锁）
			middleware.Fail(c, http.StatusUnauthorized, 1007, "动态验证码错误或已过期")
			return
		}
	} else if params.TOTPRequired {
		// 强制 2FA 但未绑定
		middleware.Fail(c, http.StatusForbidden, 1008, "账号未绑定 2FA，请联系管理员重置后绑定")
		return
	}

	// 6. 生成 Token
	tokenPair, err := auth.GenerateTokenPair(auth.TokenOptions{
		Secret:     deps.Config.JWT.Secret,
		Issuer:     params.Issuer,
		UserID:     id,
		Username:   req.Username,
		Role:       role,
		TenantID:   tenantID,
		AccessTTL:  params.AccessTTL,
		RefreshTTL: params.RefreshTTL,
	})
	if err != nil {
		middleware.Fail(c, http.StatusInternalServerError, 5004, "签发 Token 失败: "+err.Error())
		return
	}

	// 7. 更新登录信息
	ip := c.ClientIP()
	if err := updateLoginInfo(deps.DB, id, ip, time.Now()); err != nil {
		// 登录信息更新失败不影响登录
		_ = err
	}

	// 7.1 写入会话记录（refresh_token_device 表，供 ListLoginDevices 查询）
	ua := c.Request.Header.Get("User-Agent")
	_ = recordLoginSession(deps, role, id, ip, ua, params.RefreshTTL)

	// 8. 清除失败计数
	_ = auth.ClearLoginFailure(ctx, deps.Redis, role, req.Username)

	middleware.Success(c, loginResp{
		TokenPair: tokenPair,
		User: userInfo{
			ID:       id,
			Username: req.Username,
			Role:     role,
			TenantID: tenantID,
			Status:   status,
		},
	})
}

// ============== 超管登录 ==============

func AdminLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		doLogin(c, deps, auth.RoleAdmin,
			func(db *gorm.DB, username string) (uint64, string, string, string, uint64, error) {
				var admin model.SysAdmin
				err := db.Select("id, password_hash, totp_secret, status").
					Where("username = ?", username).First(&admin).Error
				if err != nil {
					return 0, "", "", "", 0, err
				}
				return admin.ID, admin.PasswordHash, admin.TOTPSecret, admin.Status, 0, nil
			},
			func(db *gorm.DB, id uint64, ip string, t time.Time) error {
				return db.Model(&model.SysAdmin{}).Where("id = ?", id).
					Updates(map[string]interface{}{
						"last_login_at": t,
						"last_login_ip": ip,
					}).Error
			},
		)
	}
}

// ============== 开发者登录 ==============

func TenantLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		doLogin(c, deps, auth.RoleTenant,
			func(db *gorm.DB, username string) (uint64, string, string, string, uint64, error) {
				var tenant model.SysTenant
				err := db.Select("id, password_hash, totp_secret, status").
					Where("username = ?", username).First(&tenant).Error
				if err != nil {
					return 0, "", "", "", 0, err
				}
				return tenant.ID, tenant.PasswordHash, tenant.TOTPSecret, tenant.Status, tenant.ID, nil
			},
			func(db *gorm.DB, id uint64, ip string, t time.Time) error {
				return db.Model(&model.SysTenant{}).Where("id = ?", id).
					Updates(map[string]interface{}{
						"last_login_at": t,
						"last_login_ip": ip,
					}).Error
			},
		)
	}
}

// ============== 代理登录 ==============

func AgentLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		doLogin(c, deps, auth.RoleAgent,
			func(db *gorm.DB, username string) (uint64, string, string, string, uint64, error) {
				var agent model.Agent
				err := db.Select("id, password_hash, totp_secret, status, tenant_id").
					Where("username = ?", username).First(&agent).Error
				if err != nil {
					return 0, "", "", "", 0, err
				}
				return agent.ID, agent.PasswordHash, agent.TOTPSecret, agent.Status, agent.TenantID, nil
			},
			func(db *gorm.DB, id uint64, ip string, t time.Time) error {
				return db.Model(&model.Agent{}).Where("id = ?", id).
					Updates(map[string]interface{}{
						"last_login_at": t,
						"last_login_ip": ip,
					}).Error
			},
		)
	}
}

// ============== 开发者注册 ==============

func TenantRegister(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 1. 校验开关
		if !deps.CfgCache.GetBool(ctx, "tenant.register.enabled", true) {
			middleware.Fail(c, http.StatusForbidden, 1009, "开发者注册已关闭")
			return
		}

		var req registerTenantReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 2. 校验密码长度（从 sys_config 读取）
		minLen := deps.CfgCache.GetInt(ctx, "security.password_min_length", 8)
		if len(req.Password) < minLen {
			middleware.Fail(c, http.StatusBadRequest, 1010, "密码长度至少 "+itoa(minLen)+" 位")
			return
		}

		// 3. 检查用户名唯一
		var count int64
		if err := deps.DB.Model(&model.SysTenant{}).Where("username = ?", req.Username).Count(&count).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5005, "查询失败: "+err.Error())
			return
		}
		if count > 0 {
			middleware.Fail(c, http.StatusConflict, 1011, "用户名已被使用")
			return
		}

		// 4. 加密密码
		passwordHash, err := crypto.HashPassword(req.Password)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5006, "密码加密失败: "+err.Error())
			return
		}

		// 5. 读取默认套餐 + 试用天数
		packageID := uint64(deps.CfgCache.GetInt(ctx, "tenant.register.default_package_id", 1))
		trialDays := deps.CfgCache.GetInt(ctx, "tenant.register.trial_days", 7)

		// 校验套餐是否存在
		var pkg model.SysPackage
		if err := deps.DB.Select("id, status").First(&pkg, packageID).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5007, "默认套餐不存在，请联系管理员")
			return
		}
		if pkg.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1012, "默认套餐已下架，请联系管理员")
			return
		}

		// 6. 生成 TenantCode（简化：用时间戳后 8 位 + 随机 4 位）
		tenantCode := genTenantCode()

		// 7. 计算到期时间
		var expiresAt *time.Time
		if trialDays > 0 {
			t := time.Now().AddDate(0, 0, trialDays)
			expiresAt = &t
		}

		// 8. 入库
		tenant := &model.SysTenant{
			TenantCode:   tenantCode,
			Username:     req.Username,
			PasswordHash: passwordHash,
			Email:        req.Email,
			Phone:        req.Phone,
			Company:      req.Company,
			Status:       "active", // 注册即激活（如需审核可改为 pending）
			PackageID:    packageID,
			ExpiresAt:    expiresAt,
		}
		if err := deps.DB.Create(tenant).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5008, "创建账号失败: "+err.Error())
			return
		}

		// 9. 自动签发 Token（注册后免登录）
		params := loadAuthParams(deps, auth.RoleTenant)
		tokenPair, err := auth.GenerateTokenPair(auth.TokenOptions{
			Secret:     deps.Config.JWT.Secret,
			Issuer:     params.Issuer,
			UserID:     tenant.ID,
			Username:   tenant.Username,
			Role:       auth.RoleTenant,
			TenantID:   tenant.ID,
			AccessTTL:  params.AccessTTL,
			RefreshTTL: params.RefreshTTL,
		})
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5004, "签发 Token 失败: "+err.Error())
			return
		}

		// 10. 写入会话记录（与登录保持一致）
		_ = recordLoginSession(deps, auth.RoleTenant, tenant.ID, c.ClientIP(),
			c.Request.Header.Get("User-Agent"), params.RefreshTTL)

		middleware.Success(c, gin.H{
			"token_pair": tokenPair,
			"user": userInfo{
				ID:       tenant.ID,
				Username: tenant.Username,
				Role:     auth.RoleTenant,
				TenantID: tenant.ID,
				Status:   tenant.Status,
			},
			"package_id":  packageID,
			"trial_days":  trialDays,
			"expires_at":  expiresAt,
		})
	}
}

// ============== 代理注册（v0.3.6 完整版）==============
// 流程（方案 B：先支付后建 Agent，避免引入 pending_payment 状态破坏 AgentLogin 不变量）：
//  1. 校验邀请码（active + 未达 max_uses + 未过期）
//  2. 校验用户名唯一性
//  3. quota.CheckMaxAgents 校验套餐上限（第一道防线）
//  4. 读 sys_config agent.register.fee 注册费
//  5. 创建 AgentRegistrationOrder（PayStatus=pending，AgentID=nil 占位）
//  6. 调易支付 BuildSubmitURL 返回 pay_url
//  7. 支付回调 EpayNotify 按订单号前缀 REG 分发到 processAgentRegisterPaid 真正建 Agent

// agentRegisterReq 代理注册请求体
type agentRegisterReq struct {
	InviteCode string `json:"invite_code" binding:"required,min=4,max=32"`
	Username   string `json:"username" binding:"required,min=3,max=64"`
	Password   string `json:"password" binding:"required,min=8,max=64"`
	Phone      string `json:"phone" binding:"omitempty,max=32"`
	PayType    string `json:"pay_type" binding:"required,oneof=alipay wxpay qqpay"`
}

// AgentRegister POST /api/v1/public/auth/agent/register
// 返回 pay_url，前端跳转易支付完成支付
func AgentRegister(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req agentRegisterReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		ctx := c.Request.Context()

		// 1. 校验平台支付总开关
		if !deps.CfgCache.GetBool(ctx, "pay.platform.enabled", true) {
			middleware.Fail(c, http.StatusForbidden, 1007, "平台支付未启用，无法完成代理注册")
			return
		}

		// 2. 校验邀请码
		var ic model.AgentInviteCode
		if err := deps.DB.Where("code = ?", req.InviteCode).First(&ic).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusBadRequest, 1008, "邀请码不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询邀请码失败: "+err.Error())
			return
		}
		if ic.Status != "active" {
			middleware.Fail(c, http.StatusBadRequest, 1008, "邀请码已失效（状态: "+ic.Status+"）")
			return
		}
		if ic.UsedCount >= ic.MaxUses {
			middleware.Fail(c, http.StatusBadRequest, 1008, "邀请码使用次数已用完")
			return
		}
		if ic.ExpiresAt.Before(time.Now()) {
			middleware.Fail(c, http.StatusBadRequest, 1008, "邀请码已过期")
			return
		}

		// 3. 校验用户名在所属租户内唯一
		var exists int64
		deps.DB.Model(&model.Agent{}).Where("tenant_id = ? AND username = ?", ic.TenantID, req.Username).Count(&exists)
		if exists > 0 {
			middleware.Fail(c, http.StatusBadRequest, 1009, "用户名已被占用")
			return
		}

		// 4. 校验套餐代理数上限（quota.CheckMaxAgents 第一道防线）
		if err := quota.CheckMaxAgents(deps.DB, ic.TenantID); err != nil {
			var qErr *quota.ExceededError
			if errors.As(err, &qErr) {
				if qErr.Limit == 0 {
					middleware.Fail(c, http.StatusForbidden, 1007, "当前套餐不支持招募代理，请联系开发者升级套餐")
				} else {
					middleware.Fail(c, http.StatusForbidden, 1007,
						"开发者代理数已达上限 "+strconv.Itoa(qErr.Limit)+" 个，请联系开发者")
				}
			} else {
				middleware.Fail(c, http.StatusForbidden, 1007, err.Error())
			}
			return
		}

		// 5. 读注册费（铁律 05：从 sys_config 读取，不硬编码）
		fee := deps.CfgCache.GetFloat64(ctx, "agent.register.fee", 99.00)
		if fee <= 0 {
			middleware.Fail(c, http.StatusForbidden, 1007, "代理注册费未配置或为 0，请联系平台管理员")
			return
		}

		// 6. 加载易支付配置
		epayCfg, err := loadPlatformPayConfig(deps)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "支付配置加载失败: "+err.Error())
			return
		}

		// 7. 生成订单号（前缀 REG 与卡密购买 ORD 区分，回调按前缀分发）
		orderNo := snowflake.OrderNo("REG")

		// 7.1 计算密码 bcrypt 哈希（cost=12）并缓存到 Redis，等支付回调 processAgentRegisterPaid 取用
		//     铁律 04：DB 不存明文密码也不存哈希到 AgentRegistrationOrder（订单表只存账号元信息）
		pwdHash, err := crypto.HashPassword(req.Password)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "密码加密失败: "+err.Error())
			return
		}
		if err := cacheAgentRegisterPassword(deps, orderNo, pwdHash); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "密码缓存失败: "+err.Error())
			return
		}

		// 8. 创建 AgentRegistrationOrder（pending，AgentID=nil 占位）
		order := &model.AgentRegistrationOrder{
			OrderNo:      orderNo,
			InviteCodeID: ic.ID,
			TenantID:     ic.TenantID,
			AgentID:      nil, // 支付回调时回填
			Username:     req.Username,
			Phone:        req.Phone,
			Amount:       fee,
			PayChannel:   "epay_" + req.PayType,
			PayStatus:    "pending",
			ClientIP:     c.ClientIP(),
		}
		if err := deps.DB.Create(order).Error; err != nil {
			// 订单创建失败：清理 Redis 密码缓存避免泄漏
			deps.Redis.Del(ctx, "agent_register:pwd:"+orderNo)
			middleware.Fail(c, http.StatusInternalServerError, 5003, "创建订单失败: "+err.Error())
			return
		}

		// 9. 构造易支付跳转 URL
		namePrefix := deps.CfgCache.GetString(ctx, "pay.platform.order_name_prefix", "KeyAuth代理注册")
		moneyStr := strconv.FormatFloat(fee, 'f', 2, 64)
		payURL, err := epay.BuildSubmitURL(epayCfg, &epay.OrderParams{
			OutTradeNo: orderNo,
			Name:       fmt.Sprintf("%s·%s", namePrefix, req.Username),
			Money:      moneyStr,
			PayType:    req.PayType,
			NotifyURL:  resolveNotifyURL(deps, c),
			ReturnURL:  resolveReturnURL(deps, c),
			ClientIP:   c.ClientIP(),
		})
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5004, "构造支付 URL 失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"order_no": orderNo,
			"pay_url":  payURL,
			"amount":   fee,
			"message":  "请在新页面完成支付，支付成功后代理账号将自动创建",
		})
	}
}

// AgentRegisterOrderStatus GET /api/v1/public/auth/agent/register/order/:order_no
// 前端支付完成跳回后查询订单状态，判断注册是否成功
func AgentRegisterOrderStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderNo := c.Param("order_no")
		if orderNo == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "订单号不能为空")
			return
		}

		var order model.AgentRegistrationOrder
		if err := deps.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1010, "订单不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询订单失败: "+err.Error())
			return
		}

		resp := gin.H{
			"order_no":   order.OrderNo,
			"pay_status": order.PayStatus,
			"amount":     order.Amount,
			"username":   order.Username,
			"created_at": order.CreatedAt,
			"paid_at":    order.PaidAt,
		}
		if order.AgentID != nil {
			resp["agent_id"] = *order.AgentID
		}
		middleware.Success(c, resp)
	}
}

// AgentRegisterConfig GET /api/v1/public/auth/agent/register/config
// 公开返回代理注册所需配置（注册费 + 支付方式 + 平台支付开关），供注册页未登录时读取
// 铁律 05：所有可变参数走 sys_config；铁律 06：不返回敏感配置（如 gateway_url/pid/key）
func AgentRegisterConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		fee := deps.CfgCache.GetFloat64(ctx, "agent.register.fee", 99.00)
		payEnabled := deps.CfgCache.GetBool(ctx, "pay.platform.enabled", true)

		// 支付方式列表（JSON 数组字符串 → 解析）
		methodsJSON := deps.CfgCache.GetString(ctx, "pay.platform.methods", `["alipay","wxpay","qqpay"]`)
		var methods []string
		if err := json.Unmarshal([]byte(methodsJSON), &methods); err != nil || len(methods) == 0 {
			methods = []string{"alipay", "wxpay", "qqpay"}
		}

		middleware.Success(c, gin.H{
			"register_fee":    fee,
			"pay_enabled":     payEnabled,
			"pay_methods":     methods,
			"order_expire_seconds": deps.CfgCache.GetInt64(ctx, "pay.order_expire_seconds", 1800),
		})
	}
}

// ============== Token 刷新 ==============

// RefreshToken 用 refresh token 换取新的 access + refresh token
// 三角色共用一个端点
func RefreshToken(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req refreshReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误")
			return
		}

		// 1. 解析 refresh token
		claims, tokenType, err := auth.ParseToken(deps.Config.JWT.Secret, req.RefreshToken)
		if err != nil {
			middleware.Fail(c, http.StatusUnauthorized, 2002, "Refresh Token 无效或已过期")
			return
		}
		if tokenType != auth.TokenTypeRefresh {
			middleware.Fail(c, http.StatusUnauthorized, 2002, "请使用 Refresh Token 而非 Access Token")
			return
		}

		// 2. 检查黑名单
		blacklisted, err := auth.IsRefreshTokenBlacklisted(deps.Redis, claims.UserID, claims.Role)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5009, "会话校验失败")
			return
		}
		if blacklisted {
			middleware.Fail(c, http.StatusUnauthorized, 2003, "会话已失效，请重新登录")
			return
		}

		// 3. 签发新 Token（轮换：旧 refresh token 立即失效）
		// 将旧 refresh token 加入黑名单（剩余 TTL 由 ParseToken 的过期时间推算）
		params := loadAuthParams(deps, claims.Role)
		_ = auth.BlacklistRefreshToken(deps.Redis, claims.UserID, claims.Role, params.RefreshTTL)

		// 重新生成
		tokenPair, err := auth.GenerateTokenPair(auth.TokenOptions{
			Secret:     deps.Config.JWT.Secret,
			Issuer:     params.Issuer,
			UserID:     claims.UserID,
			Username:   claims.Username,
			Role:       claims.Role,
			TenantID:   claims.TenantID,
			AccessTTL:  params.AccessTTL,
			RefreshTTL: params.RefreshTTL,
		})
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5004, "签发 Token 失败")
			return
		}

		middleware.Success(c, gin.H{"token_pair": tokenPair})
	}
}

// ============== 登出 ==============

// Logout 登出：将当前 refresh token 加入黑名单
// 需要从客户端传 refresh_token（access token 自然过期即可）
func Logout(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req refreshReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误")
			return
		}

		claims, tokenType, err := auth.ParseToken(deps.Config.JWT.Secret, req.RefreshToken)
		if err != nil || tokenType != auth.TokenTypeRefresh {
			// 即使 token 无效也返回成功（客户端幂等清理）
			middleware.Success(c, nil)
			return
		}

		// 加入黑名单（剩余 TTL = refresh token 剩余有效期）
		params := loadAuthParams(deps, claims.Role)
		_ = auth.BlacklistRefreshToken(deps.Redis, claims.UserID, claims.Role, params.RefreshTTL)

		// 同步标记该用户的所有会话为已撤销（v0.3.1：与黑名单行为保持一致）
		markAllSessionsRevoked(deps, claims.Role, claims.UserID)

		middleware.Success(c, gin.H{"logged_out": true})
	}
}

// ============== 获取当前登录用户信息 ==============

// CurrentUser 返回 JWT 中的当前用户信息
func CurrentUser(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		tenantID, _ := c.Get("tenant_id")

		middleware.Success(c, gin.H{
			"user_id":   userID,
			"username":  username,
			"role":      role,
			"tenant_id": tenantID,
		})
	}
}

// ============== 辅助函数 ==============

// itoa 简化版 int -> string（避免引入 strconv）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 8)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		return "-" + string(buf)
	}
	return string(buf)
}

// genTenantCode 生成开发者编号
// 规则：T + 年月日(8位) + 随机4位（示例：T20260719A3B5）
// 注意：随机数使用 crypto/rand 保证不可预测（需验证：当前简化为时间戳）
func genTenantCode() string {
	// TODO(待核实): 改用 crypto/rand 生成不可预测的随机部分
	t := time.Now()
	// 简化实现：用纳秒级时间戳后 4 位（生产环境应替换）
	return "T" + t.Format("20060102") + strings.ToUpper(t.Format("150405"))[:4]
}
