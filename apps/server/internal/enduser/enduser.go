// Package enduser v0.4.0 终端用户体系核心包
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：注册/登录/密码/验证码/Token TTL/绑定上限/IP 限流 全部从 sys_config 读取
//   05 - 配置走后端：10 项 enduser.* 配置可通过后台实时调整
//   06 - 反幻觉：密码 bcrypt(cost=12)；refresh token SHA-512 哈希存储；jti 单点踢出；测试覆盖正/负/边界
//
// 核心能力：
//   1. Manager.Register - 终端用户注册（用户名 + 密码 + 可选邮箱/手机）
//   2. Manager.Login - 用户名密码登录（返回 access + refresh token）
//   3. Manager.RefreshToken - refresh token 轮换
//   4. Manager.Logout - 注销（撤销当前 jti）
//   5. Manager.RevokeSession - 管理员/用户踢出指定 jti
//   6. Manager.BindCard - 卡密绑定到用户
//   7. Manager.UnbindCard - 解绑卡密
//   8. Manager.ListMyCards - 列出用户绑定的卡密
//   9. Manager.GetCardDetail - 卡密详情（含绑定状态）
//  10. Manager.ChangePassword / Manager.ResetPassword / Manager.UpdateProfile
//  11. Manager.VerifyRefreshToken - 验证 refresh token 有效性
package enduser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 常量 ==============

// 配置键常量（铁律 04）
const (
	CfgKeyRegisterEnabled      = "enduser.register_enabled"
	CfgKeyLoginMethod          = "enduser.login_method"
	CfgKeyPasswordMinLength    = "enduser.password_min_length"
	CfgKeyVerifyCodeTTL        = "enduser.verify_code_ttl"
	CfgKeyVerifyCodeLength     = "enduser.verify_code_length"
	CfgKeyAccessTokenTTL       = "enduser.access_token_ttl"
	CfgKeyRefreshTokenTTL      = "enduser.refresh_token_ttl"
	CfgKeyBindCardPerUserMax   = "enduser.bind_card_per_user_max"
	CfgKeyAllowAnonymousQuery  = "enduser.allow_anonymous_query"
	CfgKeyIPRateLimitPerMinute = "enduser.ip_rate_limit_per_minute"
)

// UserStatus 用户状态
const (
	UserStatusActive  = "active"
	UserStatusBanned  = "banned"
	UserStatusDeleted = "deleted"
)

// BindStatus 绑定状态
const (
	BindStatusActive  = "active"
	BindStatusUnbound = "unbound"
)

// 错误
var (
	ErrUserNotFound          = errors.New("enduser: user not found")
	ErrUserBanned            = errors.New("enduser: user banned")
	ErrUserExists            = errors.New("enduser: username already exists")
	ErrPasswordTooShort      = errors.New("enduser: password too short")
	ErrPasswordIncorrect     = errors.New("enduser: password incorrect")
	ErrRegisterDisabled      = errors.New("enduser: register disabled")
	ErrCardNotFound          = errors.New("enduser: card not found")
	ErrCardAlreadyBound      = errors.New("enduser: card already bound to another user")
	ErrCardBoundToOther      = errors.New("enduser: card bound to another user")
	ErrBindLimitExceeded     = errors.New("enduser: bind card limit exceeded")
	ErrTokenInvalid          = errors.New("enduser: refresh token invalid")
	ErrTokenExpired          = errors.New("enduser: refresh token expired")
	ErrTokenRevoked          = errors.New("enduser: refresh token revoked")
	ErrNotAllowed            = errors.New("enduser: not allowed")
	ErrCardStatusInvalid     = errors.New("enduser: card status invalid for binding")
)

// ============== 类型 ==============

// RegisterRequest 注册请求
type RegisterRequest struct {
	TenantID uint64
	AppID    uint64
	Username string
	Password string
	Phone    string
	Email    string
	Nickname string
}

// LoginRequest 登录请求
type LoginRequest struct {
	TenantID  uint64
	AppID     uint64
	Username  string
	Password  string
	IP        string
	UserAgent string
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // access token 有效期秒数
	TokenType    string `json:"token_type"` // Bearer
}

// Manager 终端用户管理器
type Manager struct {
	db    *gorm.DB
	cache *config.ConfigCache
}

// NewManager 创建管理器
func NewManager(db *gorm.DB, cache *config.ConfigCache) *Manager {
	return &Manager{db: db, cache: cache}
}

// ============== 1. 注册 ==============

// Register 终端用户注册
func (m *Manager) Register(ctx context.Context, req RegisterRequest) (*model.EndUser, error) {
	// 1. 检查注册开关
	if !m.cache.GetBool(ctx, CfgKeyRegisterEnabled, true) {
		return nil, ErrRegisterDisabled
	}
	// 2. 用户名校验
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		return nil, errors.New("enduser: username required")
	}
	// 3. 密码长度校验
	minLen := m.cache.GetInt(ctx, CfgKeyPasswordMinLength, 8)
	if len(req.Password) < minLen {
		return nil, ErrPasswordTooShort
	}
	// 4. 重复用户名检查（tenant_id + app_id + username 唯一）
	var count int64
	m.db.Model(&model.EndUser{}).Where("tenant_id = ? AND app_id = ? AND username = ? AND status != ?",
		req.TenantID, req.AppID, req.Username, UserStatusDeleted).Count(&count)
	if count > 0 {
		return nil, ErrUserExists
	}
	// 5. bcrypt 密码哈希
	hash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("enduser: hash password failed: %w", err)
	}
	// 6. 写库
	user := &model.EndUser{
		TenantID:     req.TenantID,
		AppID:        req.AppID,
		Username:     req.Username,
		Phone:        strings.TrimSpace(req.Phone),
		Email:        strings.TrimSpace(req.Email),
		PasswordHash: hash,
		Nickname:     req.Nickname,
		Status:       UserStatusActive,
	}
	if err := m.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("enduser: create user failed: %w", err)
	}
	return user, nil
}

// ============== 2. 登录 ==============

// Login 用户名密码登录
// 返回 TokenPair（含 access + refresh）+ 用户实体
func (m *Manager) Login(ctx context.Context, req LoginRequest, jwtSecret string) (*TokenPair, *model.EndUser, error) {
	// 1. 查用户
	var user model.EndUser
	err := m.db.Where("tenant_id = ? AND app_id = ? AND username = ? AND status != ?",
		req.TenantID, req.AppID, req.Username, UserStatusDeleted).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrUserNotFound
		}
		return nil, nil, err
	}
	// 2. 状态检查
	if user.Status == UserStatusBanned {
		return nil, nil, ErrUserBanned
	}
	// 3. 密码校验
	if !crypto.CheckPassword(user.PasswordHash, req.Password) {
		return nil, nil, ErrPasswordIncorrect
	}
	// 4. 生成 access token（JWT）
	accessTTL := m.cache.GetInt(ctx, CfgKeyAccessTokenTTL, 2) // 默认 2 小时
	accessToken, err := generateAccessToken(&user, jwtSecret, accessTTL)
	if err != nil {
		return nil, nil, fmt.Errorf("enduser: generate access token failed: %w", err)
	}
	// 5. 生成 refresh token（随机 + SHA-512 哈希存储）
	refreshTTL := m.cache.GetInt(ctx, CfgKeyRefreshTokenTTL, 30) // 默认 30 天
	refreshToken, tokenEntry, err := m.issueRefreshToken(ctx, &user, req.IP, req.UserAgent, refreshTTL)
	if err != nil {
		return nil, nil, err
	}
	// 6. 更新登录信息
	now := time.Now()
	m.db.Model(&user).Updates(map[string]interface{}{
		"last_login_at": &now,
		"last_login_ip": req.IP,
		"last_login_ua": req.UserAgent,
		"login_count":   user.LoginCount + 1,
	})
	_ = tokenEntry
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTTL) * 3600,
		TokenType:    "Bearer",
	}, &user, nil
}

// generateAccessToken 生成 access token（简化版：HMAC-SHA256(secret|user_id|exp)）
// 铁律 06：实际生产应使用 jwt 库；此处用简化签名避免引入 jwt 依赖（jwt 已在 middleware 中）
func generateAccessToken(user *model.EndUser, secret string, ttlHours int) (string, error) {
	exp := time.Now().Add(time.Duration(ttlHours) * time.Hour).Unix()
	payload := fmt.Sprintf("%d|%d|%d", user.ID, user.AppID, exp)
	sig := crypto.HMACSHA256(secret, []byte(payload))
	return fmt.Sprintf("%s.%s", payload, sig), nil
}

// issueRefreshToken 生成 refresh token 并入库
func (m *Manager) issueRefreshToken(ctx context.Context, user *model.EndUser, ip, ua string, ttlDays int) (string, *model.EndUserToken, error) {
	// 1. 生成随机 token
	rawToken := uuid.New().String() + uuid.New().String()
	// 2. SHA-512 哈希存储
	hashed := crypto.SHA512Hex(rawToken)
	// 3. jti（uuid 唯一）
	jti := uuid.New().String()
	// 4. 设备名/类型从 UA 简单提取
	deviceName, deviceType := parseUA(ua)
	// 5. 入库
	exp := time.Now().Add(time.Duration(ttlDays) * 24 * time.Hour)
	entry := &model.EndUserToken{
		UserID:       user.ID,
		JTI:          jti,
		DeviceName:   deviceName,
		DeviceType:   deviceType,
		IP:           ip,
		UserAgent:    ua,
		RefreshToken: hashed,
		ExpiresAt:    exp,
	}
	if err := m.db.Create(entry).Error; err != nil {
		return "", nil, fmt.Errorf("enduser: save refresh token failed: %w", err)
	}
	return rawToken, entry, nil
}

// parseUA 简单解析 User-Agent
func parseUA(ua string) (name, dtype string) {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") {
		dtype = "mobile"
	} else if strings.Contains(ua, "bot") || strings.Contains(ua, "spider") {
		dtype = "bot"
	} else {
		dtype = "pc"
	}
	// 设备名取 UA 前 128 字符（避免过长）
	if len(ua) > 128 {
		name = ua[:128]
	} else {
		name = ua
	}
	return
}

// ============== 3. Refresh Token 验证 + 轮换 ==============

// VerifyRefreshToken 验证 refresh token 有效性，返回 token entry + user
func (m *Manager) VerifyRefreshToken(ctx context.Context, refreshToken string) (*model.EndUserToken, *model.EndUser, error) {
	if refreshToken == "" {
		return nil, nil, ErrTokenInvalid
	}
	hashed := crypto.SHA512Hex(refreshToken)
	var entry model.EndUserToken
	err := m.db.Where("refresh_token = ?", hashed).First(&entry).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrTokenInvalid
		}
		return nil, nil, err
	}
	// 1. 已撤销
	if entry.RevokedAt != nil {
		return nil, nil, ErrTokenRevoked
	}
	// 2. 已过期
	if time.Now().After(entry.ExpiresAt) {
		return nil, nil, ErrTokenExpired
	}
	// 3. 查用户
	var user model.EndUser
	if err := m.db.First(&user, entry.UserID).Error; err != nil {
		return nil, nil, ErrUserNotFound
	}
	if user.Status != UserStatusActive {
		return nil, nil, ErrUserBanned
	}
	return &entry, &user, nil
}

// RefreshToken 轮换 refresh token（旧 token 撤销 + 发新 token）
func (m *Manager) RefreshToken(ctx context.Context, oldRefreshToken, jwtSecret, ip, ua string) (*TokenPair, error) {
	entry, user, err := m.VerifyRefreshToken(ctx, oldRefreshToken)
	if err != nil {
		return nil, err
	}
	// 1. 撤销旧 token
	now := time.Now()
	m.db.Model(&model.EndUserToken{}).Where("id = ?", entry.ID).Updates(map[string]interface{}{
		"revoked_at": &now,
	})
	// 2. 生成新 token
	accessTTL := m.cache.GetInt(ctx, CfgKeyAccessTokenTTL, 2)
	accessToken, err := generateAccessToken(user, jwtSecret, accessTTL)
	if err != nil {
		return nil, err
	}
	refreshTTL := m.cache.GetInt(ctx, CfgKeyRefreshTokenTTL, 30)
	newRefresh, _, err := m.issueRefreshToken(ctx, user, ip, ua, refreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresIn:    int64(accessTTL) * 3600,
		TokenType:    "Bearer",
	}, nil
}

// ============== 4. 注销 / 撤销 ==============

// Logout 注销当前 jti 对应的 refresh token
func (m *Manager) Logout(ctx context.Context, refreshToken string) error {
	entry, _, err := m.VerifyRefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}
	now := time.Now()
	return m.db.Model(&model.EndUserToken{}).Where("id = ?", entry.ID).Updates(map[string]interface{}{
		"revoked_at": &now,
	}).Error
}

// RevokeSession 撤销指定 jti（管理员踢出）
func (m *Manager) RevokeSession(ctx context.Context, userID uint64, jti string) error {
	now := time.Now()
	return m.db.Model(&model.EndUserToken{}).
		Where("user_id = ? AND jti = ?", userID, jti).
		Updates(map[string]interface{}{"revoked_at": &now}).Error
}

// RevokeAllSessions 撤销用户所有有效 refresh token（修改密码时调用）
func (m *Manager) RevokeAllSessions(ctx context.Context, userID uint64) error {
	now := time.Now()
	return m.db.Model(&model.EndUserToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]interface{}{"revoked_at": &now}).Error
}

// ListSessions 列出用户的有效会话
func (m *Manager) ListSessions(ctx context.Context, userID uint64) ([]model.EndUserToken, error) {
	var items []model.EndUserToken
	err := m.db.Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?",
		userID, time.Now()).Order("created_at DESC").Find(&items).Error
	return items, err
}

// ============== 5. 卡密绑定 ==============

// BindCard 卡密绑定到用户
// 流程：① 校验卡密存在且未绑定 ② 校验用户绑定数上限 ③ 事务写绑定关系 + 更新卡密 end_user_id
func (m *Manager) BindCard(ctx context.Context, userID, cardID uint64) (*model.EndUserCard, error) {
	// 1. 查卡密
	var card model.AppCard
	if err := m.db.First(&card, cardID).Error; err != nil {
		return nil, ErrCardNotFound
	}
	// 2. 卡密状态校验（封禁/禁用不允许绑定）
	if card.Status == "banned" || card.Status == "disabled" {
		return nil, ErrCardStatusInvalid
	}
	// 3. 卡密已绑定其他用户
	if card.EndUserID != nil && *card.EndUserID != userID {
		return nil, ErrCardBoundToOther
	}
	// 4. 查用户
	var user model.EndUser
	if err := m.db.First(&user, userID).Error; err != nil {
		return nil, ErrUserNotFound
	}
	if user.Status != UserStatusActive {
		return nil, ErrUserBanned
	}
	// 5. 用户绑定数上限
	maxBind := m.cache.GetInt(ctx, CfgKeyBindCardPerUserMax, 10)
	if maxBind > 0 {
		var count int64
		m.db.Model(&model.EndUserCard{}).
			Where("user_id = ? AND status = ?", userID, BindStatusActive).Count(&count)
		if count >= int64(maxBind) {
			return nil, ErrBindLimitExceeded
		}
	}
	// 6. 是否已绑定（含 unbound 的旧记录，复用）
	var existing model.EndUserCard
	err := m.db.Where("card_id = ?", cardID).First(&existing).Error
	if err == nil {
		// 已有记录
		if existing.UserID != userID {
			return nil, ErrCardAlreadyBound
		}
		if existing.Status == BindStatusActive {
			return &existing, nil // 已绑定，幂等返回
		}
		// 旧记录解绑过，重新激活
		now := time.Now()
		m.db.Model(&existing).Updates(map[string]interface{}{
			"status":     BindStatusActive,
			"bound_at":   &now,
			"unbound_at": nil,
		})
		m.db.Model(&card).Updates(map[string]interface{}{"end_user_id": &userID})
		return &existing, nil
	}
	// 7. 事务：写绑定 + 更新卡密
	entry := &model.EndUserCard{
		UserID:   userID,
		CardID:   cardID,
		TenantID: card.TenantID,
		AppID:    card.AppID,
		Status:   BindStatusActive,
	}
	tx := m.db.Begin()
	if err := tx.Create(entry).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("enduser: create bind failed: %w", err)
	}
	if err := tx.Model(&model.AppCard{}).Where("id = ?", cardID).
		Updates(map[string]interface{}{"end_user_id": &userID}).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("enduser: update card failed: %w", err)
	}
	tx.Commit()
	return entry, nil
}

// UnbindCard 解绑卡密
func (m *Manager) UnbindCard(ctx context.Context, userID, cardID uint64) error {
	// 1. 校验归属
	var entry model.EndUserCard
	err := m.db.Where("user_id = ? AND card_id = ? AND status = ?", userID, cardID, BindStatusActive).First(&entry).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCardNotFound
		}
		return err
	}
	// 2. 事务：标记解绑 + 清空卡密 end_user_id
	now := time.Now()
	tx := m.db.Begin()
	if err := tx.Model(&entry).Updates(map[string]interface{}{
		"status":     BindStatusUnbound,
		"unbound_at": &now,
	}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&model.AppCard{}).Where("id = ?", cardID).
		Updates(map[string]interface{}{"end_user_id": nil}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// ListMyCards 列出用户绑定的卡密
func (m *Manager) ListMyCards(ctx context.Context, userID uint64, page, pageSize int) ([]model.AppCard, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	// 通过 end_user_card 关联查询 app_card
	var cardIDs []uint64
	m.db.Model(&model.EndUserCard{}).
		Where("user_id = ? AND status = ?", userID, BindStatusActive).
		Pluck("card_id", &cardIDs)
	if len(cardIDs) == 0 {
		return []model.AppCard{}, 0, nil
	}
	var total int64
	m.db.Model(&model.AppCard{}).Where("id IN ?", cardIDs).Count(&total)
	var cards []model.AppCard
	if err := m.db.Where("id IN ?", cardIDs).Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&cards).Error; err != nil {
		return nil, 0, err
	}
	return cards, total, nil
}

// GetCardDetail 查询单张卡密详情（含绑定状态）
func (m *Manager) GetCardDetail(ctx context.Context, userID, cardID uint64) (*model.AppCard, error) {
	// 校验归属
	var entry model.EndUserCard
	err := m.db.Where("user_id = ? AND card_id = ? AND status = ?", userID, cardID, BindStatusActive).First(&entry).Error
	if err != nil {
		return nil, ErrCardNotFound
	}
	var card model.AppCard
	if err := m.db.First(&card, cardID).Error; err != nil {
		return nil, ErrCardNotFound
	}
	return &card, nil
}

// ============== 6. 个人信息 ==============

// GetProfile 获取用户信息
func (m *Manager) GetProfile(ctx context.Context, userID uint64) (*model.EndUser, error) {
	var user model.EndUser
	if err := m.db.First(&user, userID).Error; err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// UpdateProfile 更新用户信息（仅昵称 / 头像 / 邮箱 / 手机）
func (m *Manager) UpdateProfile(ctx context.Context, userID uint64, updates map[string]interface{}) error {
	allowed := map[string]bool{"nickname": true, "avatar_url": true, "email": true, "phone": true}
	filtered := map[string]interface{}{}
	for k, v := range updates {
		if allowed[k] {
			filtered[k] = v
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return m.db.Model(&model.EndUser{}).Where("id = ?", userID).Updates(filtered).Error
}

// ChangePassword 修改密码（需校验旧密码）
func (m *Manager) ChangePassword(ctx context.Context, userID uint64, oldPassword, newPassword string) error {
	minLen := m.cache.GetInt(ctx, CfgKeyPasswordMinLength, 8)
	if len(newPassword) < minLen {
		return ErrPasswordTooShort
	}
	var user model.EndUser
	if err := m.db.First(&user, userID).Error; err != nil {
		return ErrUserNotFound
	}
	if !crypto.CheckPassword(user.PasswordHash, oldPassword) {
		return ErrPasswordIncorrect
	}
	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	// 事务：更新密码 + 撤销所有会话
	tx := m.db.Begin()
	if err := tx.Model(&model.EndUser{}).Where("id = ?", userID).
		Update("password_hash", hash).Error; err != nil {
		tx.Rollback()
		return err
	}
	now := time.Now()
	if err := tx.Model(&model.EndUserToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]interface{}{"revoked_at": &now}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// ResetPassword 重置密码（管理员/验证码流程使用，不需旧密码）
func (m *Manager) ResetPassword(ctx context.Context, userID uint64, newPassword string) error {
	minLen := m.cache.GetInt(ctx, CfgKeyPasswordMinLength, 8)
	if len(newPassword) < minLen {
		return ErrPasswordTooShort
	}
	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	tx := m.db.Begin()
	if err := tx.Model(&model.EndUser{}).Where("id = ?", userID).
		Update("password_hash", hash).Error; err != nil {
		tx.Rollback()
		return err
	}
	now := time.Now()
	if err := tx.Model(&model.EndUserToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]interface{}{"revoked_at": &now}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// ============== 7. 辅助 ==============

// IsRegisterEnabled 注册开关
func (m *Manager) IsRegisterEnabled(ctx context.Context) bool {
	return m.cache.GetBool(ctx, CfgKeyRegisterEnabled, true)
}

// IsAnonymousQueryAllowed 是否允许匿名查卡
func (m *Manager) IsAnonymousQueryAllowed(ctx context.Context) bool {
	return m.cache.GetBool(ctx, CfgKeyAllowAnonymousQuery, true)
}

// ValidateAccessToken 校验 access token 签名 + 过期
// 返回 userID, appID, error
func ValidateAccessToken(token, secret string) (uint64, uint64, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return 0, 0, ErrTokenInvalid
	}
	payload := parts[0]
	sig := parts[1]
	// 重新计算签名
	expectedSig := crypto.HMACSHA256(secret, []byte(payload))
	if sig != expectedSig {
		return 0, 0, ErrTokenInvalid
	}
	// 解析 payload
	var userID, appID uint64
	var exp int64
	if _, err := fmt.Sscanf(payload, "%d|%d|%d", &userID, &appID, &exp); err != nil {
		return 0, 0, ErrTokenInvalid
	}
	if time.Now().Unix() > exp {
		return 0, 0, ErrTokenExpired
	}
	return userID, appID, nil
}
