// 公开 API Handler（无需鉴权）
// 对应路由：/api/v1/public/*
// 用途：H5 终端用户购卡流程所需的应用信息 + 卡类列表查询
// 严格遵循铁律 04/05/06：
//   - 仅返回公开字段，敏感字段（app_secret/sign_secret/agent_base_price 等）绝不外泄
//   - 仅返回 active 状态的 App / SysTenant / AppCardType
//   - 卡类列表不返回代理成本价等内部字段
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== DTO ==============

// publicAppInfo H5 应用信息响应（仅公开字段）
type publicAppInfo struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Status      string `json:"status"`
	TenantName  string `json:"tenant_name,omitempty"` // 开发者展示名（公司名优先）
}

// publicCardType H5 卡类列表项（仅公开字段，不含代理成本价等）
type publicCardType struct {
	ID              uint64  `json:"id"`
	AppID           uint64  `json:"app_id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`              // duration/count/permanent/trial/feature
	DurationSeconds int64   `json:"duration_seconds"`  // 永久卡=-1
	MaxUses         int     `json:"max_uses"`
	Price           float64 `json:"price"`
	Features        string  `json:"features,omitempty"`
	Status          string  `json:"status"`
}

// ============== PublicAppInfo 应用公开信息 ==============

// PublicAppInfo GET /api/v1/public/apps/info?app_key=xxx
// 用于 H5 购卡首页：用户输入 AppKey 后查询应用信息
// 安全：
//  1. 仅返回 active 状态的 App
//  2. 仅返回所属 SysTenant 为 active 状态的应用
//  3. 不返回 app_secret / sign_secret 等敏感字段
//  4. 应用过期不算 active（tenant.ExpiresAt 已过期则拒绝）
func PublicAppInfo(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		appKey := c.Query("app_key")
		if appKey == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "app_key 参数不能为空")
			return
		}

		// 联表 sys_tenant 校验状态
		var app model.App
		err := deps.DB.Where("app_key = ? AND status = ?", appKey, "active").First(&app).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 4001, "应用不存在或已禁用")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询应用失败")
			return
		}

		// 校验开发者状态
		var tenant model.SysTenant
		if err := deps.DB.First(&tenant, app.TenantID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4001, "应用所属开发者不存在")
			return
		}
		if tenant.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 4002, "应用所属开发者已被禁用")
			return
		}

		// 拼装响应
		resp := publicAppInfo{
			ID:          app.ID,
			Name:        app.Name,
			Description: app.Description,
			Icon:        app.Icon,
			Status:      app.Status,
			TenantName:  tenant.Company,
		}
		if resp.TenantName == "" {
			resp.TenantName = tenant.Username
		}

		middleware.Success(c, resp)
	}
}

// ============== PublicCardTypes 卡类公开列表 ==============

// PublicCardTypes GET /api/v1/public/card_types?app_id=xxx
// 用于 H5 购卡首页：展示用户可购买的卡类列表
// 安全：
//  1. 校验 app_id 对应的 App 为 active 状态
//  2. 校验 App 所属 SysTenant 为 active 状态
//  3. 仅返回 status=active 的卡类
//  4. 不返回 agent_base_price / agent_commission_rate 等内部字段
func PublicCardTypes(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		appIDStr := c.Query("app_id")
		if appIDStr == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "app_id 参数不能为空")
			return
		}
		appID, err := strconv.ParseUint(appIDStr, 10, 64)
		if err != nil || appID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "app_id 参数格式错误")
			return
		}

		// 校验 App + Tenant 状态（与 PublicAppInfo 一致）
		var app model.App
		if err := deps.DB.Where("id = ? AND status = ?", appID, "active").First(&app).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4001, "应用不存在或已禁用")
			return
		}
		var tenant model.SysTenant
		if err := deps.DB.Select("id, status").First(&tenant, app.TenantID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4001, "应用所属开发者不存在")
			return
		}
		if tenant.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 4002, "应用所属开发者已被禁用")
			return
		}

		// 查询 active 卡类（按价格升序，便于 H5 展示）
		var types []model.AppCardType
		if err := deps.DB.Where("app_id = ? AND status = ?", appID, "active").
			Order("price ASC, id ASC").
			Find(&types).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询卡类失败")
			return
		}

		// 转换为公开 DTO（过滤内部字段）
		list := make([]publicCardType, 0, len(types))
		for _, t := range types {
			list = append(list, publicCardType{
				ID:              t.ID,
				AppID:           t.AppID,
				Name:            t.Name,
				Type:            t.Type,
				DurationSeconds: t.DurationSeconds,
				MaxUses:         t.MaxUses,
				Price:           t.Price,
				Features:        t.Features,
				Status:          t.Status,
			})
		}

		middleware.Success(c, gin.H{
			"list":  list,
			"total": len(list),
		})
	}
}
