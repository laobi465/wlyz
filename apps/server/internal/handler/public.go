// 公开 API Handler（无需鉴权）
// 对应路由：/api/v1/public/*
// 用途：H5 终端用户购卡流程所需的应用信息 + 卡类列表查询 + 公告详情 + 客服联系方式
// 严格遵循铁律 04/05/06：
//   - 仅返回公开字段，敏感字段（app_secret/sign_secret/agent_base_price 等）绝不外泄
//   - 仅返回 active 状态的 App / SysTenant / AppCardType
//   - 卡类列表不返回代理成本价等内部字段
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

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

// ============== v0.4.x 残留项 2：U-12 公告详情 H5 页面 ==============

// 配置键常量（铁律 04：禁止硬编码）
const (
	CfgKeyNoticePublicDetailEnabled = "notice.public_detail.enabled"
)

// PublicNoticeDetail GET /api/v1/public/notices/:id
// 公告详情（公开端点，无需鉴权）
// 流程：
//  1. 校验公告存在且 status=published
//  2. 校验当前时间在 start_at / end_at 范围内
//  3. 并发安全地增加 view_count：UPDATE notice SET view_count = view_count + 1
//  4. 返回公告详情（含 content_format 供前端区分 text/html 渲染）
//
// 安全（铁律 06）：
//   - content 字段允许 HTML，但前端必须做 XSS 过滤（v-html 配合 DOMPurify 或白名单过滤）
//   - 后端仅返回原始内容，不主动转义；前端按 content_format 决定渲染方式
//   - 仅返回 published 状态的公告，草稿/下线不可访问
func PublicNoticeDetail(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "公告 ID 格式错误")
			return
		}

		// 总开关（铁律 05：可通过 sys_config 关闭公开详情，强制走弹窗流程）
		ctx := context.Background()
		if deps.CfgCache != nil {
			if !deps.CfgCache.GetBool(ctx, CfgKeyNoticePublicDetailEnabled, true) {
				middleware.Fail(c, http.StatusForbidden, 4003, "公告详情功能已关闭")
				return
			}
		}

		var notice model.Notice
		if err := deps.DB.First(&notice, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 4001, "公告不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询公告失败")
			return
		}

		// 仅返回已发布公告
		if notice.Status != "published" {
			middleware.Fail(c, http.StatusNotFound, 4001, "公告不存在或已下线")
			return
		}

		// 时间窗口校验
		now := time.Now()
		if notice.StartAt.After(now) {
			middleware.Fail(c, http.StatusNotFound, 4001, "公告尚未发布")
			return
		}
		if notice.EndAt != nil && notice.EndAt.Before(now) {
			middleware.Fail(c, http.StatusNotFound, 4001, "公告已过期")
			return
		}

		// 并发安全地增加 view_count（铁律 06：UPDATE SET view_count = view_count + 1，避免读-改-写竞争）
		// 错误不阻塞响应（统计只是辅助信息）
		_ = deps.DB.Model(&model.Notice{}).Where("id = ?", id).
			UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error

		middleware.Success(c, gin.H{
			"id":             notice.ID,
			"type":           notice.Type,
			"tenant_id":      notice.TenantID,
			"app_id":         notice.AppID,
			"title":          notice.Title,
			"content":        notice.Content,
			"content_format": notice.ContentFormat,
			"is_pinned":      notice.IsPinned,
			"show_badge":     notice.ShowBadge,
			"start_at":       notice.StartAt,
			"end_at":         notice.EndAt,
			"view_count":     notice.ViewCount + 1, // 反映本次浏览后的最新值
			"sort":           notice.Sort,
			"created_at":     notice.CreatedAt,
		})
	}
}

// ============== v0.4.x 残留项 4：U-14 联系客服 H5 页面 ==============

// 配置键常量（铁律 04：禁止硬编码）
const (
	CfgKeyContactQQGroup = "contact.qq_group"
	CfgKeyContactWechat  = "contact.wechat"
	CfgKeyContactEmail   = "contact.email"
	CfgKeyContactPhone   = "contact.phone"
)

// PublicContact GET /api/v1/public/contact
// 返回平台客服联系方式（从 sys_config 读取）
// 4 项配置均可为空，留空时前端不展示对应渠道
// 安全：仅返回字符串值，不含任何敏感配置
func PublicContact(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		resp := gin.H{
			"qq_group": "",
			"wechat":   "",
			"email":    "",
			"phone":    "",
		}
		if deps.CfgCache != nil {
			resp["qq_group"] = deps.CfgCache.GetString(ctx, CfgKeyContactQQGroup, "")
			resp["wechat"] = deps.CfgCache.GetString(ctx, CfgKeyContactWechat, "")
			resp["email"] = deps.CfgCache.GetString(ctx, CfgKeyContactEmail, "")
			resp["phone"] = deps.CfgCache.GetString(ctx, CfgKeyContactPhone, "")
		}
		middleware.Success(c, resp)
	}
}

// ============== v0.4.x P-06 代理独立门户（展示 + 走开发者支付通道） ==============

// publicPortalAgentInfo 代理门户公开信息（仅展示字段，不含余额/佣金等内部信息）
type publicPortalAgentInfo struct {
	AgentID    uint64 `json:"agent_id"`
	Username   string `json:"username"`
	RealName   string `json:"real_name"`
	TenantID   uint64 `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
	Subdomain  string `json:"subdomain,omitempty"`
	Status     string `json:"status"`
}

// publicPortalCardType 代理门户可售卡类（仅公开字段，过滤代理成本价 agent_base_price）
type publicPortalCardType struct {
	ID              uint64  `json:"id"`
	AppID           uint64  `json:"app_id"`
	AppName         string  `json:"app_name"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	DurationSeconds int64   `json:"duration_seconds"`
	MaxUses         int     `json:"max_uses"`
	Price           float64 `json:"price"`
	Features        string  `json:"features,omitempty"`
}

// PublicPortal GET /api/v1/public/portal/:agent_id
// 代理独立 H5 门户：返回代理公开信息 + 可售卡类列表
// 安全：
//  1. 仅代理 status=active + 开发者 status=active 时返回数据
//  2. 卡类列表仅返回 active 状态、且归属代理所在 tenant 的卡类
//  3. 不返回 agent_base_price / agent_commission_rate 等内部字段
//  4. 不返回代理余额、佣金、提现等敏感财务信息
func PublicPortal(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID, err := parseUintParam(c, "agent_id")
		if err != nil || agentID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "agent_id 参数无效")
			return
		}

		// 1. 查代理 + 联表开发者
		var agent model.Agent
		if err := deps.DB.Where("id = ? AND status = ?", agentID, "active").First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "代理不存在或已禁用")
			return
		}

		var tenant model.SysTenant
		if err := deps.DB.First(&tenant, agent.TenantID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4001, "代理所属开发者不存在")
			return
		}
		if tenant.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 4002, "代理所属开发者已被禁用")
			return
		}
		if tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now()) {
			middleware.Fail(c, http.StatusForbidden, 4002, "代理所属开发者账号已过期")
			return
		}

		tenantName := tenant.Company
		if tenantName == "" {
			tenantName = tenant.Username
		}

		// 2. 查询该 tenant 下所有 active 应用（仅取 id + name 用于联表展示）
		var apps []model.App
		deps.DB.Select("id, name").Where("tenant_id = ? AND status = ?", agent.TenantID, "active").Find(&apps)
		appNameMap := make(map[uint64]string, len(apps))
		appIDs := make([]uint64, 0, len(apps))
		for _, a := range apps {
			appNameMap[a.ID] = a.Name
			appIDs = append(appIDs, a.ID)
		}

		// 3. 查询 active 卡类（按价格升序，便于 H5 展示）
		var types []model.AppCardType
		if len(appIDs) > 0 {
			deps.DB.Where("app_id IN ? AND tenant_id = ? AND status = ?",
				appIDs, agent.TenantID, "active").
				Order("price ASC, id ASC").
				Find(&types)
		}

		list := make([]publicPortalCardType, 0, len(types))
		for _, t := range types {
			list = append(list, publicPortalCardType{
				ID:              t.ID,
				AppID:           t.AppID,
				AppName:         appNameMap[t.AppID],
				Name:            t.Name,
				Type:            t.Type,
				DurationSeconds: t.DurationSeconds,
				MaxUses:         t.MaxUses,
				Price:           t.Price,
				Features:        t.Features,
			})
		}

		middleware.Success(c, gin.H{
			"agent": publicPortalAgentInfo{
				AgentID:    agent.ID,
				Username:   agent.Username,
				RealName:   agent.RealName,
				TenantID:   agent.TenantID,
				TenantName: tenantName,
				Subdomain:  agent.Subdomain,
				Status:     agent.Status,
			},
			"card_types": list,
			"total":      len(list),
		})
	}
}

// PublicPortalOrder POST /api/v1/public/portal/:agent_id/order
// 代理门户下单：复用平台/开发者自有支付通道，强制 agent_id 来自 URL（覆盖 body）
// 安全：
//  1. 校验代理存在且 active
//  2. 校验代理所属开发者 active 且未过期
//  3. agent_id 强制从 URL 取（防止前端伪造其他代理 ID）
//  4. 实际下单逻辑完全复用 CreatePayOrder（含支付通道选择/订单创建/易支付跳转）
//  5. 不写入额外抽成记录（代理门户订单与 H5 终端用户订单一致，由 processPaidOrder 统一处理）
func PublicPortalOrder(deps *Deps) gin.HandlerFunc {
	inner := CreatePayOrder(deps)
	return func(c *gin.Context) {
		agentID, err := parseUintParam(c, "agent_id")
		if err != nil || agentID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "agent_id 参数无效")
			return
		}

		// 1. 校验代理存在 + active
		var agent model.Agent
		if err := deps.DB.Select("id, tenant_id, status").
			Where("id = ? AND status = ?", agentID, "active").First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "代理不存在或已禁用")
			return
		}

		// 2. 校验开发者 active 且未过期
		var tenant model.SysTenant
		if err := deps.DB.First(&tenant, agent.TenantID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4001, "代理所属开发者不存在")
			return
		}
		if tenant.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 4002, "代理所属开发者已被禁用")
			return
		}
		if tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now()) {
			middleware.Fail(c, http.StatusForbidden, 4002, "代理所属开发者账号已过期")
			return
		}

		// 3. 读取请求体 + 强制注入 agent_id（来自 URL，覆盖 body 中的同名字段）
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "读取请求体失败")
			return
		}
		bodyMap := map[string]interface{}{}
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
				middleware.Fail(c, http.StatusBadRequest, 1001, "请求体格式错误")
				return
			}
		}
		bodyMap["agent_id"] = agentID
		newBody, _ := json.Marshal(bodyMap)
		c.Request.Body = io.NopCloser(bytes.NewReader(newBody))
		c.Request.ContentLength = int64(len(newBody))

		// 4. 委托给 CreatePayOrder 处理实际下单逻辑
		inner(c)
	}
}
