// 账号设置 Handler：admin / tenant / agent 三角色统一的资料 / 密码 / 2FA / 设备管理
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取（CfgCache）
// 遵循铁律 06：不确定处标注「待核实」
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/auth"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== DTO ==============

// updateProfileReq 更新基本资料请求
// 注：avatar 字段当前三表均无对应列，仅在 DTO 层接收，待核实 v0.3.x 加列后落库
type updateProfileReq struct {
	RealName string `json:"real_name" binding:"omitempty,max=64"`
	Email    string `json:"email" binding:"omitempty,email,max=128"`
	Phone    string `json:"phone" binding:"omitempty,max=32"`
	Company  string `json:"company" binding:"omitempty,max=128"`
	Avatar   string `json:"avatar" binding:"omitempty,max=255"` // 待核实 v0.3.x：表结构加 avatar 字段
}

// changePasswordReq 修改密码请求
type changePasswordReq struct {
	OldPassword     string `json:"old_password" binding:"required,min=1,max=128"`
	NewPassword     string `json:"new_password" binding:"required,min=1,max=128"`
	ConfirmPassword string `json:"confirm_password" binding:"required,min=1,max=128"`
}

// verify2FAReq 启用 2FA 验证请求
type verify2FAReq struct {
	Code string `json:"code" binding:"required,len=6"`
}

// disable2FAReq 关闭 2FA 请求（code 与 password 二选一或同时提供）
type disable2FAReq struct {
	Code     string `json:"code" binding:"omitempty,len=6"`
	Password string `json:"password" binding:"omitempty,min=1,max=128"`
}

// twoFASetupData 2FA setup 临时数据（Redis 中转）
type twoFASetupData struct {
	Secret      string   `json:"secret"`
	OTPAUTHURL  string   `json:"otpauth_url"`
	BackupCodes []string `json:"backup_codes"`
}

// ============== 内部辅助函数 ==============

// loadUserProfile 按 role 查对应表，返回统一字段格式
// 各 role 字段映射 frontend CurrentUser 接口
func loadUserProfile(deps *Deps, role string, userID uint64) (gin.H, error) {
	switch role {
	case auth.RoleAdmin:
		var admin model.SysAdmin
		if err := deps.DB.Select("id, username, email, phone, status, totp_secret, last_login_at, last_login_ip, created_at").
			First(&admin, userID).Error; err != nil {
			return nil, err
		}
		// admin 表无 real_name/company 字段，real_name 备用 email
		return gin.H{
			"user_id":       admin.ID,
			"username":      admin.Username,
			"role":          role,
			"tenant_id":     uint64(0),
			"email":         admin.Email,
			"phone":         admin.Phone,
			"real_name":     admin.Email, // 待核实：admin 表无 real_name 字段，暂用 email 备用
			"company":       "",
			"status":        admin.Status,
			"created_at":    admin.CreatedAt,
			"last_login_at": admin.LastLoginAt,
			"last_login_ip": admin.LastLoginIP,
			"totp_enabled":  admin.TOTPSecret != "",
		}, nil

	case auth.RoleTenant:
		var tenant model.SysTenant
		if err := deps.DB.Select("id, username, email, phone, company, status, expires_at, totp_secret, last_login_at, last_login_ip, created_at").
			First(&tenant, userID).Error; err != nil {
			return nil, err
		}
		// tenant 表无 real_name 字段，real_name 备用 company
		return gin.H{
			"user_id":       tenant.ID,
			"username":      tenant.Username,
			"role":          role,
			"tenant_id":     tenant.ID,
			"email":         tenant.Email,
			"phone":         tenant.Phone,
			"real_name":     tenant.Company, // 待核实：tenant 表无 real_name 字段，暂用 company 备用
			"company":       tenant.Company,
			"status":        tenant.Status,
			"expires_at":    tenant.ExpiresAt, // tenant 专属字段
			"created_at":    tenant.CreatedAt,
			"last_login_at": tenant.LastLoginAt,
			"last_login_ip": tenant.LastLoginIP,
			"totp_enabled":  tenant.TOTPSecret != "",
		}, nil

	case auth.RoleAgent:
		var agent model.Agent
		if err := deps.DB.Select("id, tenant_id, username, real_name, phone, balance, status, last_login_at, created_at").
			First(&agent, userID).Error; err != nil {
			return nil, err
		}
		return gin.H{
			"user_id":       agent.ID,
			"username":      agent.Username,
			"role":          role,
			"tenant_id":     agent.TenantID,
			"email":         "", // 待核实 v0.3.x：agent 表无 email 字段
			"phone":         agent.Phone,
			"real_name":     agent.RealName,
			"company":       "",
			"status":        agent.Status,
			"balance":       agent.Balance, // agent 专属字段
			"created_at":    agent.CreatedAt,
			"last_login_at": agent.LastLoginAt,
			"last_login_ip": "", // 待核实 v0.3.x：agent 表无 last_login_ip 字段
			"totp_enabled":  false, // 待核实 v0.3.x：agent 表无 totp_secret 字段
		}, nil
	}

	return nil, errors.New("unsupported role: " + role)
}

// loadUserPasswordHash 按 role 加载密码哈希
func loadUserPasswordHash(deps *Deps, role string, userID uint64) (string, error) {
	switch role {
	case auth.RoleAdmin:
		var admin model.SysAdmin
		if err := deps.DB.Select("password_hash").First(&admin, userID).Error; err != nil {
			return "", err
		}
		return admin.PasswordHash, nil
	case auth.RoleTenant:
		var tenant model.SysTenant
		if err := deps.DB.Select("password_hash").First(&tenant, userID).Error; err != nil {
			return "", err
		}
		return tenant.PasswordHash, nil
	case auth.RoleAgent:
		var agent model.Agent
		if err := deps.DB.Select("password_hash").First(&agent, userID).Error; err != nil {
			return "", err
		}
		return agent.PasswordHash, nil
	}
	return "", errors.New("unsupported role: " + role)
}

// loadUserTOTPSecret 按 role 加载 TOTP 密钥（返回 AES 加密后的密文）
// 注：agent 表暂无 totp_secret 字段，返回空字符串（待核实 v0.3.x）
func loadUserTOTPSecret(deps *Deps, role string, userID uint64) (string, error) {
	switch role {
	case auth.RoleAdmin:
		var admin model.SysAdmin
		if err := deps.DB.Select("totp_secret").First(&admin, userID).Error; err != nil {
			return "", err
		}
		return admin.TOTPSecret, nil
	case auth.RoleTenant:
		var tenant model.SysTenant
		if err := deps.DB.Select("totp_secret").First(&tenant, userID).Error; err != nil {
			return "", err
		}
		return tenant.TOTPSecret, nil
	case auth.RoleAgent:
		// 待核实 v0.3.x：agent 表加 totp_secret 字段后启用
		return "", nil
	}
	return "", errors.New("unsupported role: " + role)
}

// updateUserTOTPSecret 按 role 更新 TOTP 密钥（密文入库）
// secretEnc 为空字符串表示清空 2FA
func updateUserTOTPSecret(deps *Deps, role string, userID uint64, secretEnc string) error {
	switch role {
	case auth.RoleAdmin:
		return deps.DB.Model(&model.SysAdmin{}).Where("id = ?", userID).
			Update("totp_secret", secretEnc).Error
	case auth.RoleTenant:
		return deps.DB.Model(&model.SysTenant{}).Where("id = ?", userID).
			Update("totp_secret", secretEnc).Error
	case auth.RoleAgent:
		// 待核实 v0.3.x：agent 表加 totp_secret 字段后启用
		return errors.New("agent 2FA 待核实 v0.3.x：表结构无 totp_secret 字段")
	}
	return errors.New("unsupported role: " + role)
}

// twoFASetupKey 2FA setup Redis Key（TTL 10min）
func twoFASetupKey(role string, userID uint64) string {
	return "2fa:setup:" + role + ":" + strconv.FormatUint(userID, 10)
}

// twoFABackupKey 2FA backup codes Redis Key（持久化，待核实 v0.3.x 加表字段后迁移）
func twoFABackupKey(role string, userID uint64) string {
	return "2fa:backup:" + role + ":" + strconv.FormatUint(userID, 10)
}

// parseDeviceName 从 User-Agent 简化解析设备名称（OS / Browser）
// 待核实 v0.3.x：引入更完整的 UA 解析库（如 mileusna/ua 或 oe ua-parser）
func parseDeviceName(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return "Unknown Device"
	}
	uaLower := strings.ToLower(ua)

	osName := "Unknown OS"
	switch {
	case strings.Contains(uaLower, "windows"):
		osName = "Windows"
	case strings.Contains(uaLower, "mac os") || strings.Contains(uaLower, "macintosh"):
		osName = "macOS"
	case strings.Contains(uaLower, "android"):
		osName = "Android"
	case strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad"):
		osName = "iOS"
	case strings.Contains(uaLower, "linux"):
		osName = "Linux"
	}

	browser := "Unknown Browser"
	switch {
	case strings.Contains(uaLower, "edg/"):
		browser = "Edge"
	case strings.Contains(uaLower, "chrome/"):
		browser = "Chrome"
	case strings.Contains(uaLower, "firefox/"):
		browser = "Firefox"
	case strings.Contains(uaLower, "safari/"):
		browser = "Safari"
	}

	return osName + " / " + browser
}

// ============== 1. ProfileMe 当前用户完整信息 ==============

// ProfileMe 返回当前登录用户的完整资料（覆盖 auth.go 中的 CurrentUser）
// GET /{role}/auth/me
func ProfileMe(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		profile, err := loadUserProfile(deps, role, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				middleware.Fail(c, http.StatusNotFound, 1008, "用户不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询用户信息失败: "+err.Error())
			return
		}

		middleware.Success(c, profile)
	}
}

// ============== 2. UpdateProfile 更新基本资料 ==============

// UpdateProfile 按 role 更新对应表的资料字段
// PUT /{role}/auth/profile
func UpdateProfile(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		var req updateProfileReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 构造更新 map（仅更新对应表支持的字段）
		updates := make(map[string]interface{})
		switch role {
		case auth.RoleAdmin:
			// admin：更新 email/phone
			if req.Email != "" {
				updates["email"] = req.Email
			}
			if req.Phone != "" {
				updates["phone"] = req.Phone
			}
		case auth.RoleTenant:
			// tenant：更新 email/phone/company
			if req.Email != "" {
				updates["email"] = req.Email
			}
			if req.Phone != "" {
				updates["phone"] = req.Phone
			}
			if req.Company != "" {
				updates["company"] = req.Company
			}
		case auth.RoleAgent:
			// agent：更新 real_name/phone
			// 待核实 v0.3.x：agent 表加 email 字段后启用 email 更新
			if req.RealName != "" {
				updates["real_name"] = req.RealName
			}
			if req.Phone != "" {
				updates["phone"] = req.Phone
			}
		default:
			middleware.Fail(c, http.StatusBadRequest, 1001, "不支持的角色: "+role)
			return
		}

		// avatar 字段当前三表均无对应列，忽略（待核实 v0.3.x）
		// 不参与 updates 构造

		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "未提交任何更新字段")
			return
		}

		// 执行更新（按 role 选择对应模型）
		var err error
		switch role {
		case auth.RoleAdmin:
			err = deps.DB.Model(&model.SysAdmin{}).Where("id = ?", userID).Updates(updates).Error
		case auth.RoleTenant:
			err = deps.DB.Model(&model.SysTenant{}).Where("id = ?", userID).Updates(updates).Error
		case auth.RoleAgent:
			err = deps.DB.Model(&model.Agent{}).Where("id = ?", userID).Updates(updates).Error
		}
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"user_id": userID,
			"updated": true,
		})
	}
}

// ============== 3. ChangePassword 修改密码 ==============

// ChangePassword 修改密码：校验旧密码 + 新密码规则 + 黑名单 refresh token
// POST /{role}/auth/change_password
func ChangePassword(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		var req changePasswordReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 1. 校验 new == confirm
		if req.NewPassword != req.ConfirmPassword {
			middleware.Fail(c, http.StatusBadRequest, 1010, "两次输入的新密码不一致")
			return
		}

		// 2. 校验新密码长度（从 sys_config 读取最小长度）
		ctx := c.Request.Context()
		minLen := deps.CfgCache.GetInt(ctx, "security.password_min_length", 8)
		if len(req.NewPassword) < minLen {
			middleware.Fail(c, http.StatusBadRequest, 1010, "新密码长度至少 "+itoa(minLen)+" 位")
			return
		}

		// 3. 校验旧密码正确
		oldHash, err := loadUserPasswordHash(deps, role, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				middleware.Fail(c, http.StatusNotFound, 1008, "用户不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询用户失败: "+err.Error())
			return
		}
		if !crypto.CheckPassword(oldHash, req.OldPassword) {
			middleware.Fail(c, http.StatusUnauthorized, 1004, "旧密码错误")
			return
		}

		// 4. 校验新密码不能与旧密码相同
		if crypto.CheckPassword(oldHash, req.NewPassword) {
			middleware.Fail(c, http.StatusBadRequest, 1011, "新密码不能与旧密码相同")
			return
		}

		// 5. bcrypt 加密新密码
		newHash, err := crypto.HashPassword(req.NewPassword)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5006, "密码加密失败: "+err.Error())
			return
		}

		// 6. 更新对应表 password_hash
		switch role {
		case auth.RoleAdmin:
			err = deps.DB.Model(&model.SysAdmin{}).Where("id = ?", userID).Update("password_hash", newHash).Error
		case auth.RoleTenant:
			err = deps.DB.Model(&model.SysTenant{}).Where("id = ?", userID).Update("password_hash", newHash).Error
		case auth.RoleAgent:
			err = deps.DB.Model(&model.Agent{}).Where("id = ?", userID).Update("password_hash", newHash).Error
		default:
			middleware.Fail(c, http.StatusBadRequest, 1001, "不支持的角色: "+role)
			return
		}
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新密码失败: "+err.Error())
			return
		}

		// 7. 将当前用户所有 refresh token 加入黑名单（强制其他设备重新登录）
		params := loadAuthParams(deps, role)
		_ = auth.BlacklistRefreshToken(deps.Redis, userID, role, params.RefreshTTL)

		middleware.Success(c, gin.H{
			"user_id": userID,
			"changed": true,
		})
	}
}

// ============== 4. Setup2FA 生成 2FA 密钥与二维码 ==============

// Setup2FA 生成 TOTP 密钥 + otpauth URL + 备用码，临时存 Redis（TTL 10min）
// POST /{role}/auth/2fa/setup
func Setup2FA(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		// agent 表暂无 totp_secret 字段
		if role == auth.RoleAgent {
			// 待核实 v0.3.x：agent 2FA 支持
			middleware.Fail(c, http.StatusNotImplemented, 1006, "待核实 v0.3.x：agent 2FA 支持")
			return
		}

		// 校验未绑定 2FA
		totpSecretEnc, err := loadUserTOTPSecret(deps, role, userID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询 2FA 状态失败: "+err.Error())
			return
		}
		if totpSecretEnc != "" {
			middleware.Fail(c, http.StatusConflict, 1012, "已绑定 2FA，如需重置请先关闭")
			return
		}

		// 加载 TOTP 参数（issuer / skew / period 均从 sys_config 读取）
		params := loadAuthParams(deps, role)
		ctx := c.Request.Context()

		// 账号标识：用 username（从 JWT 取）
		username, _ := c.Get("username")
		account, _ := username.(string)
		if account == "" {
			account = strconv.FormatUint(userID, 10)
		}

		// 生成 TOTP（实际 API：GenerateTOTP，返回 secret + otpauth URL + 10 个备用码）
		// 待核实：task 描述中的 auth.GenerateTOTPSecret / auth.GenerateBackupCodes 在 totp.go 中不存在
		totpResult, err := auth.GenerateTOTP(auth.TOTPOptions{
			Issuer:     params.TOTPIssuer,
			Account:    account,
			Period:     params.TOTPPeriod,
			SecretSize: 20,
			Skew:       params.TOTPSkew,
		})
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "生成 2FA 密钥失败: "+err.Error())
			return
		}

		// 备用码取前 5 个（task 要求 5 个；totp.go 默认生成 10 个）
		backupCount := 5
		if len(totpResult.BackupCodes) < backupCount {
			backupCount = len(totpResult.BackupCodes)
		}
		backupCodes := totpResult.BackupCodes[:backupCount]

		// 临时存 Redis（TTL 10min），verify 阶段再落库
		setupData := twoFASetupData{
			Secret:      totpResult.Secret,
			OTPAUTHURL:  totpResult.OTPAUTHURL,
			BackupCodes: backupCodes,
		}
		payload, err := json.Marshal(setupData)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5004, "序列化 2FA 数据失败: "+err.Error())
			return
		}
		setupTTL := time.Duration(deps.CfgCache.GetInt(ctx, "totp.setup_ttl_seconds", 600)) * time.Second
		if err := deps.Redis.Set(ctx, twoFASetupKey(role, userID), payload, setupTTL).Err(); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5005, "缓存 2FA 数据失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"secret":        totpResult.Secret,
			"qr_code_url":   totpResult.OTPAUTHURL,
			"backup_codes":  backupCodes,
			"expires_in":    int64(setupTTL.Seconds()),
		})
	}
}

// ============== 5. Verify2FA 启用 2FA 验证 ==============

// Verify2FA 校验 setup 阶段的 6 位验证码，成功后将密钥加密入库 + 备用码存 Redis
// POST /{role}/auth/2fa/verify
func Verify2FA(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		if role == auth.RoleAgent {
			// 待核实 v0.3.x：agent 2FA 支持
			middleware.Fail(c, http.StatusNotImplemented, 1006, "待核实 v0.3.x：agent 2FA 支持")
			return
		}

		var req verify2FAReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		ctx := c.Request.Context()

		// 1. 从 Redis 取 setup 数据
		raw, err := deps.Redis.Get(ctx, twoFASetupKey(role, userID)).Result()
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1013, "2FA 设置数据已过期，请重新发起 setup")
			return
		}
		var setupData twoFASetupData
		if err := json.Unmarshal([]byte(raw), &setupData); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "解析 2FA 数据失败: "+err.Error())
			return
		}

		// 2. 校验验证码
		params := loadAuthParams(deps, role)
		if !auth.ValidateTOTP(setupData.Secret, req.Code, params.TOTPSkew) {
			middleware.Fail(c, http.StatusUnauthorized, 1007, "动态验证码错误或已过期")
			return
		}

		// 3. AES 加密 secret 后入库
		secretEnc, err := auth.EncryptTOTPSecret(deps.Crypto, setupData.Secret)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "加密 2FA 密钥失败: "+err.Error())
			return
		}
		if err := updateUserTOTPSecret(deps, role, userID, secretEnc); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "保存 2FA 密钥失败: "+err.Error())
			return
		}

		// 4. 备用码持久化到 Redis（待核实 v0.3.x：表结构加 backup_codes 字段后迁移）
		// 注：备用码应 bcrypt 哈希存储，此处简化用 AES 加密后存 Redis
		backupEnc, err := deps.Crypto.EncryptAES(strings.Join(setupData.BackupCodes, ","))
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5004, "加密备用码失败: "+err.Error())
			return
		}
		// 持久化（无 TTL），待核实 v0.3.x：迁移到表字段后改为哈希入库
		if err := deps.Redis.Set(ctx, twoFABackupKey(role, userID), backupEnc, 0).Err(); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5005, "保存备用码失败: "+err.Error())
			return
		}

		// 5. 删除 setup key
		_ = deps.Redis.Del(ctx, twoFASetupKey(role, userID)).Err()

		middleware.Success(c, gin.H{
			"user_id": userID,
			"enabled": true,
		})
	}
}

// ============== 6. Disable2FA 关闭 2FA ==============

// Disable2FA 校验 password + code 后清空 totp_secret + 删除备用码 + 黑名单 refresh token
// POST /{role}/auth/2fa/disable
func Disable2FA(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		if role == auth.RoleAgent {
			// 待核实 v0.3.x：agent 2FA 支持
			middleware.Fail(c, http.StatusNotImplemented, 1006, "待核实 v0.3.x：agent 2FA 支持")
			return
		}

		var req disable2FAReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 1. 必须同时提供 password 与 code
		if req.Password == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "请输入登录密码以确认身份")
			return
		}
		if req.Code == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "请输入动态验证码")
			return
		}

		// 2. 校验密码
		oldHash, err := loadUserPasswordHash(deps, role, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				middleware.Fail(c, http.StatusNotFound, 1008, "用户不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询用户失败: "+err.Error())
			return
		}
		if !crypto.CheckPassword(oldHash, req.Password) {
			middleware.Fail(c, http.StatusUnauthorized, 1004, "登录密码错误")
			return
		}

		// 3. 校验 TOTP 验证码
		totpSecretEnc, err := loadUserTOTPSecret(deps, role, userID)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询 2FA 状态失败: "+err.Error())
			return
		}
		if totpSecretEnc == "" {
			middleware.Fail(c, http.StatusBadRequest, 1014, "未绑定 2FA，无需关闭")
			return
		}
		secretPlain, err := auth.DecryptTOTPSecret(deps.Crypto, totpSecretEnc)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "2FA 密钥解密失败: "+err.Error())
			return
		}
		params := loadAuthParams(deps, role)
		if !auth.ValidateTOTP(secretPlain, req.Code, params.TOTPSkew) {
			middleware.Fail(c, http.StatusUnauthorized, 1007, "动态验证码错误或已过期")
			return
		}

		// 4. 清空 totp_secret
		if err := updateUserTOTPSecret(deps, role, userID, ""); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5004, "关闭 2FA 失败: "+err.Error())
			return
		}

		// 5. 删除 Redis 备用码 key
		ctx := c.Request.Context()
		_ = deps.Redis.Del(ctx, twoFABackupKey(role, userID)).Err()

		// 6. 黑名单当前用户 refresh token（强制重新登录）
		_ = auth.BlacklistRefreshToken(deps.Redis, userID, role, params.RefreshTTL)

		middleware.Success(c, gin.H{
			"user_id":   userID,
			"disabled":  true,
		})
	}
}

// ============== 7. ListLoginDevices 登录设备列表 ==============

// ListLoginDevices 简化方案：仅返回当前会话信息
// GET /{role}/auth/devices
// 待核实 v0.3.x：维护 user_session 表或多端登录记录
func ListLoginDevices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := getRole(c)
		userID := getUserID(c)
		if role == "" || userID == 0 {
			middleware.Fail(c, http.StatusUnauthorized, 2001, "无法识别用户身份")
			return
		}

		ua := c.Request.Header.Get("User-Agent")
		ip := c.ClientIP()

		// 当前设备信息（id=0 表示无持久化设备 ID）
		currentDevice := gin.H{
			"id":              uint64(0),
			"device_name":     parseDeviceName(ua),
			"ip":              ip,
			"location":        "", // 待核实 v0.3.x：接入 IP 地理库
			"user_agent":      ua,
			"last_active_at":  time.Now(),
			"current":         true,
		}

		// 简化方案：暂仅返回当前会话，待 v0.3.x 维护 user_session 表后返回完整列表
		middleware.Success(c, gin.H{
			"list":       []gin.H{currentDevice},
			"total":      1,
			"pending_v03": "待核实 v0.3.x：多端登录管理",
		})
	}
}

// ============== 8. KickDevice 踢指定设备下线 ==============

// KickDevice 简化方案：暂返回 501
// DELETE /{role}/auth/devices/:id
// 待核实 v0.3.x：多端登录管理
func KickDevice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 简化方案：暂不支持，待 v0.3.x 引入 user_session 表后实现
		// 如需强制全部下线，可调用 ChangePassword 或 Disable2FA 触发黑名单
		_ = c.Param("id") // 占位，避免 lint 警告
		middleware.Fail(c, http.StatusNotImplemented, 1006, "待核实 v0.3.x：多端登录管理")
	}
}
