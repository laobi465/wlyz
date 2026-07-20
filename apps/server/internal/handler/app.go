// 应用管理 Handler
// 功能：创建/列表/详情/更新应用 + AppKey/AppSecret/SignSecret 生成与轮换
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/quota"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== DTO ==============

type createAppReq struct {
	Name                string `json:"name" binding:"required,min=2,max=128"`
	Description         string `json:"description" binding:"omitempty,max=2000"`
	Icon                string `json:"icon" binding:"omitempty,max=255"`
	MaxDevices          int    `json:"max_devices" binding:"omitempty,min=1,max=100"`
	HeartbeatInterval   int    `json:"heartbeat_interval" binding:"omitempty,min=10,max=3600"`
	HeartbeatTimeout    int    `json:"heartbeat_timeout" binding:"omitempty,min=30,max=86400"`
	OfflineGrace        int    `json:"offline_grace" binding:"omitempty,min=0,max=2592000"`
	UnbindDeductSeconds int    `json:"unbind_deduct_seconds" binding:"omitempty,min=0,max=2592000"`
	AgentCommissionMode string `json:"agent_commission_mode" binding:"omitempty,oneof=percentage diff"`
}

type updateAppReq struct {
	Name                *string `json:"name" binding:"omitempty,min=2,max=128"`
	Description         *string `json:"description" binding:"omitempty,max=2000"`
	Icon                *string `json:"icon" binding:"omitempty,max=255"`
	Status              *string `json:"status" binding:"omitempty,oneof=active disabled"`
	MaxDevices          *int    `json:"max_devices" binding:"omitempty,min=1,max=100"`
	HeartbeatInterval   *int    `json:"heartbeat_interval" binding:"omitempty,min=10,max=3600"`
	HeartbeatTimeout    *int    `json:"heartbeat_timeout" binding:"omitempty,min=30,max=86400"`
	OfflineGrace        *int    `json:"offline_grace" binding:"omitempty,min=0,max=2592000"`
	UnbindDeductSeconds *int    `json:"unbind_deduct_seconds" binding:"omitempty,min=0,max=2592000"`
	AgentCommissionMode *string `json:"agent_commission_mode" binding:"omitempty,oneof=percentage diff"`
}

// ============== 创建应用 ==============

// TenantCreateApp 开发者创建应用
// POST /api/v1/tenant/apps
func TenantCreateApp(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		var req createAppReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		ctx := c.Request.Context()

		// 1. 校验开发者账号状态
		var tenant model.SysTenant
		if err := deps.DB.Select("id, status, package_id, expires_at").First(&tenant, tenantID).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询开发者信息失败")
			return
		}
		if tenant.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "开发者账号已被禁用或待审核")
			return
		}
		if tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now()) {
			middleware.Fail(c, http.StatusForbidden, 1006, "开发者套餐已过期，请续费")
			return
		}

		// 2. 校验套餐配额（创建应用数上限）—— v0.3.5：抽到 middleware/quota.go 统一管理
		if err := quota.CheckMaxApps(deps.DB, tenantID); err != nil {
			var qErr *quota.ExceededError
			if errors.As(err, &qErr) {
				middleware.Fail(c, http.StatusForbidden, 1007,
					"已达套餐应用数上限 "+itoa(qErr.Limit)+" 个，请升级套餐")
			} else {
				middleware.Fail(c, http.StatusForbidden, 1007, err.Error())
			}
			return
		}

		// 3. 生成密钥（AppKey 明文 + AppSecret/SignSecret AES 加密入库）
		appKey, err := crypto.GenerateAppKey()
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5004, "生成 AppKey 失败")
			return
		}
		appSecretPlain, err := crypto.GenerateAppSecret()
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5005, "生成 AppSecret 失败")
			return
		}
		signSecretPlain, err := crypto.GenerateSignSecret()
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5006, "生成 SignSecret 失败")
			return
		}
		appSecretEnc, err := deps.Crypto.EncryptAES(appSecretPlain)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5007, "加密 AppSecret 失败")
			return
		}
		signSecretEnc, err := deps.Crypto.EncryptAES(signSecretPlain)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5008, "加密 SignSecret 失败")
			return
		}

		// 4. 填充默认值（从 sys_config 读取，铁律 05）
	maxDevices := req.MaxDevices
	if maxDevices == 0 {
		maxDevices = deps.CfgCache.GetInt(ctx, "app.default.max_devices", 1)
	}
	heartbeatInterval := req.HeartbeatInterval
	if heartbeatInterval == 0 {
		heartbeatInterval = deps.CfgCache.GetInt(ctx, "app.default.heartbeat_interval", 60)
	}
	heartbeatTimeout := req.HeartbeatTimeout
	if heartbeatTimeout == 0 {
		heartbeatTimeout = deps.CfgCache.GetInt(ctx, "app.default.heartbeat_timeout", 180)
	}
	offlineGrace := req.OfflineGrace
	if offlineGrace == 0 {
		offlineGrace = deps.CfgCache.GetInt(ctx, "app.default.offline_grace", 86400)
	}
	unbindDeduct := req.UnbindDeductSeconds
	if unbindDeduct == 0 {
		unbindDeduct = deps.CfgCache.GetInt(ctx, "app.default.unbind_deduct_seconds", 86400)
	}
	commissionMode := req.AgentCommissionMode
	if commissionMode == "" {
		commissionMode = "diff"
	}

	// v0.4.x S-04：审核状态受 sys_config app.audit.enabled 控制（铁律 05）
	//   app.audit.enabled=1 → 新应用初始 audit_status=pending，需 admin 审核通过后才能用于客户端验证
	//   app.audit.enabled=0 → 新应用直接 audit_status=approved（默认行为，向后兼容）
	auditStatus := "approved"
	if deps.CfgCache.GetBool(ctx, "app.audit.enabled", false) {
		auditStatus = "pending"
	}

	// 5. 入库
	app := &model.App{
		TenantID:            tenantID,
		AppKey:              appKey,
		AppSecret:           appSecretEnc,
		SignSecret:          signSecretEnc,
		Name:                req.Name,
		Description:         req.Description,
		Icon:                req.Icon,
		Status:              "active",
		MaxDevices:          maxDevices,
		HeartbeatInterval:   heartbeatInterval,
		HeartbeatTimeout:    heartbeatTimeout,
		OfflineGrace:        offlineGrace,
		UnbindDeductSeconds: unbindDeduct,
		AgentCommissionMode: commissionMode,
		AuditStatus:         auditStatus,
	}
		if err := deps.DB.Create(app).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5009, "创建应用失败: "+err.Error())
			return
		}

		// 6. 返回（含明文密钥，仅此一次返回）
		middleware.Success(c, gin.H{
			"app":          app,
			"app_secret":   appSecretPlain,   // 明文，仅本次返回，开发者必须立即保存
			"sign_secret":  signSecretPlain,  // 明文，仅本次返回
			"secret_warn":  "AppSecret 与 SignSecret 仅本次返回一次，请立即妥善保存，丢失需重置",
		})
	}
}

// ============== 应用列表 ==============

// TenantListApps 开发者应用列表
// GET /api/v1/tenant/apps?page=1&page_size=20&keyword=&status=
func TenantListApps(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}

		q := deps.DB.Model(&model.App{}).Where("tenant_id = ?", tenantID)
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("name LIKE ? OR app_key LIKE ?", "%"+kw+"%", "%"+kw+"%")
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}

		var total int64
		q.Count(&total)

		var apps []model.App
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&apps).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"list":      apps,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 应用详情 ==============

// TenantGetApp 应用详情
// GET /api/v1/tenant/apps/:id
func TenantGetApp(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "应用 ID 格式错误")
			return
		}

		var app model.App
		if err := deps.DB.Where("id = ? AND tenant_id = ?", appID, tenantID).First(&app).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "应用不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}
		middleware.Success(c, app)
	}
}

// ============== 更新应用 ==============

// TenantUpdateApp 更新应用
// PUT /api/v1/tenant/apps/:id
func TenantUpdateApp(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "应用 ID 格式错误")
			return
		}

		var req updateAppReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 校验归属
		var app model.App
		if err := deps.DB.Where("id = ? AND tenant_id = ?", appID, tenantID).First(&app).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "应用不存在或无权访问")
			return
		}

		// 构造更新 map
		updates := make(map[string]interface{})
		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.Description != nil {
			updates["description"] = *req.Description
		}
		if req.Icon != nil {
			updates["icon"] = *req.Icon
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.MaxDevices != nil {
			updates["max_devices"] = *req.MaxDevices
		}
		if req.HeartbeatInterval != nil {
			updates["heartbeat_interval"] = *req.HeartbeatInterval
		}
		if req.HeartbeatTimeout != nil {
			updates["heartbeat_timeout"] = *req.HeartbeatTimeout
		}
		if req.OfflineGrace != nil {
			updates["offline_grace"] = *req.OfflineGrace
		}
		if req.UnbindDeductSeconds != nil {
			updates["unbind_deduct_seconds"] = *req.UnbindDeductSeconds
		}
		if req.AgentCommissionMode != nil {
			updates["agent_commission_mode"] = *req.AgentCommissionMode
		}

		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "未提交任何更新字段")
			return
		}

		if err := deps.DB.Model(&model.App{}).Where("id = ? AND tenant_id = ?", appID, tenantID).
			Updates(updates).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{"id": appID, "updated": true})
	}
}

// ============== 重置密钥 ==============

// resetKeyReq 重置密钥请求
type resetKeyReq struct {
	KeyType string `json:"key_type" binding:"required,oneof=app_secret sign_secret"`
}

// TenantResetAppKey 重置 AppSecret 或 SignSecret
// POST /api/v1/tenant/apps/:id/reset_key
// 注：重置 SignSecret 时，旧密钥自动迁移到 SignSecretPrev 字段保留 7 天（密钥轮换期）
func TenantResetAppKey(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "应用 ID 格式错误")
			return
		}

		var req resetKeyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误")
			return
		}

		// 校验归属
		var app model.App
		if err := deps.DB.Where("id = ? AND tenant_id = ?", appID, tenantID).First(&app).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "应用不存在或无权访问")
			return
		}

		var plainValue string
		var encryptedValue string

		switch req.KeyType {
		case "app_secret":
			plainValue, err = crypto.GenerateAppSecret()
			if err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5003, "生成 AppSecret 失败")
				return
			}
			encryptedValue, err = deps.Crypto.EncryptAES(plainValue)
			if err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5004, "加密失败")
				return
			}
			if err := deps.DB.Model(&model.App{}).Where("id = ?", appID).
				Update("app_secret", encryptedValue).Error; err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5005, "更新失败")
				return
			}

		case "sign_secret":
			plainValue, err = crypto.GenerateSignSecret()
			if err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5003, "生成 SignSecret 失败")
				return
			}
			encryptedValue, err = deps.Crypto.EncryptAES(plainValue)
			if err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5004, "加密失败")
				return
			}
			// 旧密钥迁移到 SignSecretPrev（轮换期保留 7 天）
			if err := deps.DB.Model(&model.App{}).Where("id = ?", appID).
				Updates(map[string]interface{}{
					"sign_secret_prev": app.SignSecret,
					"sign_secret":      encryptedValue,
				}).Error; err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5005, "更新失败")
				return
			}
		}

		middleware.Success(c, gin.H{
			"app_id":      appID,
			"key_type":    req.KeyType,
			"new_value":   plainValue, // 明文，仅本次返回
			"secret_warn": "新密钥仅本次返回一次，请立即更新到客户端 SDK 配置",
		})
	}
}

// ============== 删除应用 ==============

// TenantDeleteApp 删除应用（软删除：状态置为 disabled）
// DELETE /api/v1/tenant/apps/:id
// 注：硬删除会级联影响卡密/设备/订单，故采用软删除
func TenantDeleteApp(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "应用 ID 格式错误")
			return
		}

		// 校验归属
		var app model.App
		if err := deps.DB.Where("id = ? AND tenant_id = ?", appID, tenantID).First(&app).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "应用不存在或无权访问")
			return
		}

		// 软删除：状态置为 disabled
		if err := deps.DB.Model(&model.App{}).Where("id = ?", appID).
			Update("status", "disabled").Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "删除失败")
			return
		}

		middleware.Success(c, gin.H{"id": appID, "deleted": true})
	}
}

// ============== 辅助函数 ==============

// getTenantID 从 gin.Context 获取 tenant_id
// 同时兼容 admin/tenant/agent 角色（admin 操作时 tenant_id 为 0）
func getTenantID(c *gin.Context) uint64 {
	if v, exists := c.Get("tenant_id"); exists {
		if id, ok := v.(uint64); ok {
			return id
		}
	}
	return 0
}

// getUserID 从 gin.Context 获取 user_id
func getUserID(c *gin.Context) uint64 {
	if v, exists := c.Get("user_id"); exists {
		if id, ok := v.(uint64); ok {
			return id
		}
	}
	return 0
}

// getRole 从 gin.Context 获取角色
func getRole(c *gin.Context) string {
	if v, exists := c.Get("role"); exists {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// parseUintParam 解析路径参数为 uint64
func parseUintParam(c *gin.Context, key string) (uint64, error) {
	v := strings.TrimSpace(c.Param(key))
	return strconv.ParseUint(v, 10, 64)
}

// 标记未使用导入（防编译报错）
var _ = context.Background
