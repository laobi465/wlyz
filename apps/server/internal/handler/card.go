// 卡类 + 卡密管理 Handler
// 功能：卡类 CRUD、卡密批量生成、卡密查询/封禁/解封/删除
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/heartbeat"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/quota"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 卡类 CRUD ==============

type createCardTypeReq struct {
	AppID           uint64  `json:"app_id" binding:"required"`
	Name            string  `json:"name" binding:"required,min=2,max=64"`
	Type            string  `json:"type" binding:"required,oneof=duration count permanent trial feature"`
	DurationSeconds int64   `json:"duration_seconds" binding:"omitempty,min=-1"`
	MaxUses         int     `json:"max_uses" binding:"omitempty,min=1,max=99999"`
	Price           float64 `json:"price" binding:"omitempty,min=0"`
	AgentBasePrice  float64 `json:"agent_base_price" binding:"omitempty,min=0"`
	Features        string  `json:"features" binding:"omitempty,max=2000"` // JSON 字符串
}

// TenantCreateCardType 创建卡类
// POST /api/v1/tenant/card_types
func TenantCreateCardType(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		var req createCardTypeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 校验应用归属
		if !checkAppOwnership(deps.DB, req.AppID, tenantID) {
			middleware.Fail(c, http.StatusForbidden, 1003, "应用不存在或无权访问")
			return
		}

		// 校验 duration 与 type 的匹配
		if req.Type == "duration" && req.DurationSeconds <= 0 && req.DurationSeconds != -1 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "期限卡必须设置有效的 duration_seconds（-1 表示永久）")
			return
		}
		if req.Type == "permanent" {
			req.DurationSeconds = -1 // 永久卡固定 -1
		}
		if req.Type == "count" && req.MaxUses <= 0 {
			req.MaxUses = 1 // 计次卡默认 1 次
		}

		ct := &model.AppCardType{
			TenantID:        tenantID,
			AppID:           req.AppID,
			Name:            req.Name,
			Type:            req.Type,
			DurationSeconds: req.DurationSeconds,
			MaxUses:         req.MaxUses,
			Price:           req.Price,
			AgentBasePrice:  req.AgentBasePrice,
			Features:        req.Features,
			Status:          "active",
		}
		if err := deps.DB.Create(ct).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "创建卡类失败: "+err.Error())
			return
		}
		middleware.Success(c, ct)
	}
}

// TenantListCardTypes 卡类列表
// GET /api/v1/tenant/card_types?app_id=&page=&page_size=
func TenantListCardTypes(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 50
		}

		q := deps.DB.Model(&model.AppCardType{}).Where("tenant_id = ?", tenantID)
		if appIDStr := c.Query("app_id"); appIDStr != "" {
			appID, _ := strconv.ParseUint(appIDStr, 10, 64)
			q = q.Where("app_id = ?", appID)
		}

		var total int64
		q.Count(&total)

		var list []model.AppCardType
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}
		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// TenantUpdateCardType 更新卡类
// PUT /api/v1/tenant/card_types/:id
func TenantUpdateCardType(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var req struct {
			Name           *string  `json:"name"`
			Price          *float64 `json:"price"`
			AgentBasePrice *float64 `json:"agent_base_price"`
			Status         *string  `json:"status" binding:"omitempty,oneof=active disabled"`
			Features       *string  `json:"features"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误")
			return
		}

		// 校验归属
		var ct model.AppCardType
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&ct).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "卡类不存在或无权访问")
			return
		}

		updates := make(map[string]interface{})
		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.Price != nil {
			updates["price"] = *req.Price
		}
		if req.AgentBasePrice != nil {
			updates["agent_base_price"] = *req.AgentBasePrice
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.Features != nil {
			updates["features"] = *req.Features
		}
		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "未提交任何更新字段")
			return
		}

		if err := deps.DB.Model(&model.AppCardType{}).Where("id = ? AND tenant_id = ?", id, tenantID).
			Updates(updates).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新失败")
			return
		}
		middleware.Success(c, gin.H{"id": id, "updated": true})
	}
}

// ============== 卡密批量生成 ==============

type generateCardsReq struct {
	AppID      uint64 `json:"app_id" binding:"required"`
	CardTypeID uint64 `json:"card_type_id" binding:"required"`
	Quantity   int    `json:"quantity" binding:"required,min=1,max=10000"`
	Prefix     string `json:"prefix" binding:"omitempty,max=16"`
	GroupTag   string `json:"group_tag" binding:"omitempty,max=64"`
}

// TenantGenerateCards 批量生成卡密
// POST /api/v1/tenant/cards/generate
// 事务内批量生成卡密，使用 crypto/rand 保证不可预测
func TenantGenerateCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		userID := getUserID(c)

		var req generateCardsReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		ctx := c.Request.Context()

		// 1. 校验应用归属
		var app model.App
		if err := deps.DB.Where("id = ? AND tenant_id = ?", req.AppID, tenantID).First(&app).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "应用不存在或无权访问")
			return
		}
		if app.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "应用已被禁用，无法生成卡密")
			return
		}

		// 2. 校验卡类归属
		var ct model.AppCardType
		if err := deps.DB.Where("id = ? AND tenant_id = ? AND app_id = ?", req.CardTypeID, tenantID, req.AppID).
			First(&ct).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "卡类不存在或不属于该应用")
			return
		}
		if ct.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "卡类已被禁用")
			return
		}

		// 3. 校验套餐配额（卡密总数上限）—— v0.3.5：抽到 quota 包统一管理
		if err := quota.CheckMaxCards(deps.DB, tenantID, req.Quantity); err != nil {
			var qErr *quota.ExceededError
			if errors.As(err, &qErr) {
				middleware.Fail(c, http.StatusForbidden, 1007,
					"将超过套餐卡密上限 "+itoa(qErr.Limit)+" 张（当前 "+itoa(qErr.Current)+" 张，本次生成 "+itoa(qErr.AddCount)+" 张）")
			} else {
				middleware.Fail(c, http.StatusForbidden, 1007, err.Error())
			}
			return
		}

		// 4. 单批次生成上限（从 sys_config 读取）
		maxBatch := deps.CfgCache.GetInt(ctx, "card.generate.max_batch", 10000)
		if req.Quantity > maxBatch {
			middleware.Fail(c, http.StatusBadRequest, 1001,
				"单批次最多生成 "+itoa(maxBatch)+" 张卡密")
			return
		}

		// 5. 生成批次号
		batchNo := fmt.Sprintf("B%s%06d", time.Now().Format("20060102"), userID%1000000)

		// 6. 事务批量生成
		cards := make([]*model.AppCard, 0, req.Quantity)
		cardKeys := make([]string, 0, req.Quantity) // 返回给前端（仅本次显示）
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			for i := 0; i < req.Quantity; i++ {
				key, hash, checksum, err := crypto.GenerateCardKey(req.Prefix)
				if err != nil {
					return fmt.Errorf("生成第 %d 张卡密失败: %w", i+1, err)
				}
				card := &model.AppCard{
					TenantID:        tenantID,
					AppID:           req.AppID,
					CardTypeID:      req.CardTypeID,
					CardKey:         key,
					CardKeyHash:     hash,
					Checksum:        checksum,
					Status:          "unused",
					BatchNo:         batchNo,
					Prefix:          req.Prefix,
					GroupTag:        req.GroupTag,
					DurationSeconds: ct.DurationSeconds,
					MaxUses:         ct.MaxUses,
					CreatedBy:       userID,
					CreatorType:     "tenant",
				}
				if err := tx.Create(card).Error; err != nil {
					return fmt.Errorf("入库第 %d 张卡密失败: %w", i+1, err)
				}
				cards = append(cards, card)
				cardKeys = append(cardKeys, key)
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "批量生成失败: "+txErr.Error())
			return
		}

		middleware.Success(c, gin.H{
			"batch_no":   batchNo,
			"quantity":   req.Quantity,
			"card_keys":  cardKeys, // 仅本次返回，开发者需自行保存
			"card_ids":   extractCardIDs(cards),
			"warn":       "卡密明文仅本次返回一次，请立即保存或导出",
		})
	}
}

// ============== 卡密列表 ==============

// TenantListCards 卡密列表
// GET /api/v1/tenant/cards?app_id=&status=&batch_no=&keyword=&page=&page_size=
func TenantListCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 20
		}

		q := deps.DB.Model(&model.AppCard{}).Where("tenant_id = ?", tenantID)
		if appIDStr := c.Query("app_id"); appIDStr != "" {
			appID, _ := strconv.ParseUint(appIDStr, 10, 64)
			q = q.Where("app_id = ?", appID)
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}
		if batchNo := c.Query("batch_no"); batchNo != "" {
			q = q.Where("batch_no = ?", batchNo)
		}
		if kw := c.Query("keyword"); kw != "" {
			// keyword 默认查卡密明文（注意：生产环境可考虑仅按 hash 查询）
			q = q.Where("card_key LIKE ? OR prefix LIKE ?", "%"+kw+"%", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var cards []model.AppCard
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&cards).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":      cards,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 卡密详情 ==============

// TenantGetCard 卡密详情
// GET /api/v1/tenant/cards/:id
func TenantGetCard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var card model.AppCard
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&card).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "卡密不存在或无权访问")
			return
		}
		middleware.Success(c, card)
	}
}

// ============== 卡密封禁/解封 ==============

type banCardReq struct {
	Reason string `json:"reason" binding:"required,max=255"`
}

// TenantBanCard 封禁卡密
// POST /api/v1/tenant/cards/:id/ban
func TenantBanCard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var req banCardReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 校验归属
		var card model.AppCard
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&card).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "卡密不存在或无权访问")
			return
		}
		// 状态机：unused/active 可封禁；expired/banned/disabled 不可封禁
		if card.Status != "unused" && card.Status != "active" {
			middleware.Fail(c, http.StatusBadRequest, 1009,
				"当前状态（"+card.Status+"）不可封禁，仅 unused/active 状态可封禁")
			return
		}

		now := time.Now()
		if err := deps.DB.Model(&model.AppCard{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":       "banned",
				"banned_at":    now,
				"banned_reason": req.Reason,
			}).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "封禁失败")
			return
		}

		// 同时下线该卡密绑定的所有设备：清 Redis 心跳 + DB 标记 banned
		// 铁律 05： heartbeat.Remove 内部已用 appID/deviceID 拼 Redis Key，无硬编码
		var devices []model.AppDevice
		if err := deps.DB.Where("card_id = ? AND tenant_id = ? AND status = ?", id, tenantID, "active").
			Find(&devices).Error; err == nil {
			ctx := c.Request.Context()
			for _, d := range devices {
				_ = heartbeat.Remove(ctx, deps.Redis, d.AppID, d.ID)
			}
			if len(devices) > 0 {
				deviceIDs := make([]uint64, 0, len(devices))
				for _, d := range devices {
					deviceIDs = append(deviceIDs, d.ID)
				}
				deps.DB.Model(&model.AppDevice{}).Where("id IN ?", deviceIDs).
					Updates(map[string]interface{}{
						"status":           "banned",
						"last_heartbeat_at": nil,
					})
			}
		}
		// 铁律 06：Redis 清理失败不阻塞封禁主流程（卡密已 banned，下次 verify 会因 card.status 拒绝）
		middleware.Success(c, gin.H{"id": id, "status": "banned"})
	}
}

// TenantUnbanCard 解封卡密
// POST /api/v1/tenant/cards/:id/unban
func TenantUnbanCard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var card model.AppCard
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&card).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "卡密不存在或无权访问")
			return
		}
		if card.Status != "banned" {
			middleware.Fail(c, http.StatusBadRequest, 1009, "仅 banned 状态可解封")
			return
		}

		// 解封后根据是否激活过，恢复到 active 或 unused
		newStatus := "unused"
		if card.ActivatedAt != nil {
			// 已激活过的卡密解封后回到 active，但需校验是否过期
			if card.ExpiresAt != nil && card.ExpiresAt.Before(time.Now()) {
				newStatus = "expired"
			} else {
				newStatus = "active"
			}
		}

		if err := deps.DB.Model(&model.AppCard{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":        newStatus,
				"banned_at":     nil,
				"banned_reason": "",
			}).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "解封失败")
			return
		}

		middleware.Success(c, gin.H{"id": id, "status": newStatus})
	}
}

// ============== 卡密删除 ==============

// TenantDeleteCard 删除卡密
// DELETE /api/v1/tenant/cards/:id
// 安全策略：仅 unused 状态可删除（已激活/已封禁的不可删，保留审计轨迹）
func TenantDeleteCard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var card model.AppCard
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&card).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "卡密不存在或无权访问")
			return
		}
		if card.Status != "unused" {
			middleware.Fail(c, http.StatusBadRequest, 1009,
				"仅 unused 状态卡密可删除（当前状态: "+card.Status+"），已激活卡密请使用封禁")
			return
		}

		if err := deps.DB.Delete(&model.AppCard{}, id).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "删除失败")
			return
		}
		middleware.Success(c, gin.H{"id": id, "deleted": true})
	}
}

// ============== 辅助函数 ==============

// checkAppOwnership 校验应用是否属于指定租户
func checkAppOwnership(db *gorm.DB, appID, tenantID uint64) bool {
	var count int64
	db.Model(&model.App{}).Where("id = ? AND tenant_id = ?", appID, tenantID).Count(&count)
	return count > 0
}

// extractCardIDs 从卡密列表提取 ID
func extractCardIDs(cards []*model.AppCard) []uint64 {
	ids := make([]uint64, 0, len(cards))
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	return ids
}

// ============== 卡密 CSV 导出/导入（v0.3.6） ==============

// TenantExportCardsCSV 导出卡密为 CSV
// GET /api/v1/tenant/cards/export?app_id=&status=&batch_no=&keyword=
// 铁律 05：导出条数上限从 sys_config 读取（card.export.max_rows，默认 10000）
// 铁律 04：CSV 内容为真实数据，无硬编码假数据
func TenantExportCardsCSV(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)

		// 导出条数上限从 sys_config 读取
		maxRows := deps.CfgCache.GetInt(c.Request.Context(), "card.export.max_rows", 10000)
		if maxRows <= 0 || maxRows > 100000 {
			maxRows = 10000 // 兜底，防 sys_config 配错拖垮服务
		}

		q := deps.DB.Model(&model.AppCard{}).Where("tenant_id = ?", tenantID)
		if appIDStr := c.Query("app_id"); appIDStr != "" {
			if appID, err := strconv.ParseUint(appIDStr, 10, 64); err == nil {
				q = q.Where("app_id = ?", appID)
			}
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}
		if batchNo := c.Query("batch_no"); batchNo != "" {
			q = q.Where("batch_no = ?", batchNo)
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("card_key LIKE ? OR prefix LIKE ?", "%"+kw+"%", "%"+kw+"%")
		}

		var cards []model.AppCard
		if err := q.Order("id DESC").Limit(maxRows).Find(&cards).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		filename := "cards_" + time.Now().Format("20060102_150405") + ".csv"
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
		// BOM 让 Excel 正确识别 UTF-8
		_, _ = c.Writer.Write([]byte("\xEF\xBB\xBF"))
		_, _ = c.Writer.Write([]byte("ID,AppID,CardTypeID,CardKey,Checksum,Status,BatchNo,Prefix,GroupTag,DurationSeconds,UsedCount,MaxUses,ActivatedAt,ExpiresAt,LastVerifyAt,CreatedBy,CreatorType,OrderID,BannedAt,BannedReason,CreatedAt\n"))
		for _, card := range cards {
			_, _ = c.Writer.Write([]byte(csvRow(
				card.ID, card.AppID, card.CardTypeID, card.CardKey, card.Checksum, card.Status,
				card.BatchNo, card.Prefix, card.GroupTag, card.DurationSeconds, card.UsedCount, card.MaxUses,
				ptrTimeFmt(card.ActivatedAt), ptrTimeFmt(card.ExpiresAt), ptrTimeFmt(card.LastVerifyAt),
				card.CreatedBy, card.CreatorType, ptrUint64Str(card.OrderID),
				ptrTimeFmt(card.BannedAt), card.BannedReason, card.CreatedAt.Format(time.RFC3339),
			)))
		}
	}
}

// ptrTimeFmt *time.Time 安全格式化
func ptrTimeFmt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// importCardsReq 导入卡密请求
type importCardsReq struct {
	AppID           uint64 `json:"app_id" binding:"required"`
	CardTypeID      uint64 `json:"card_type_id" binding:"required"`
	Prefix          string `json:"prefix" binding:"omitempty,max=16"`
	GroupTag        string `json:"group_tag" binding:"omitempty,max=64"`
	DurationSeconds int64  `json:"duration_seconds" binding:"omitempty,min=-1"`
	MaxUses         int    `json:"max_uses" binding:"omitempty,min=1,max=99999"`
	// Cards: CSV 解析后的卡密明文列表（前端解析 CSV 上传，传 JSON 数组）
	Cards []string `json:"cards" binding:"required,min=1"`
}

// TenantImportCardsCSV 导入卡密（前端解析 CSV 后传 JSON 数组）
// POST /api/v1/tenant/cards/import
// 铁律 05：单次导入上限从 sys_config 读取（card.import.max_rows，默认 5000）
// 铁律 06：导入失败的事务回滚 + 失败明细返回，禁只报"成功"假象
func TenantImportCardsCSV(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		userID := getUserID(c)

		var req importCardsReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 单次导入上限
		maxRows := deps.CfgCache.GetInt(c.Request.Context(), "card.import.max_rows", 5000)
		if maxRows <= 0 || maxRows > 50000 {
			maxRows = 5000
		}
		if len(req.Cards) > maxRows {
			middleware.Fail(c, http.StatusBadRequest, 1001,
				"单次最多导入 "+strconv.Itoa(maxRows)+" 张卡密，当前 "+strconv.Itoa(len(req.Cards)))
			return
		}

		// 校验应用归属
		if !checkAppOwnership(deps.DB, req.AppID, tenantID) {
			middleware.Fail(c, http.StatusForbidden, 1003, "应用不存在或无权访问")
			return
		}
		// 校验卡类归属 + 取 duration/max_uses 默认值
		var ct model.AppCardType
		if err := deps.DB.Where("id = ? AND app_id = ?", req.CardTypeID, req.AppID).First(&ct).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "卡类不存在或不属于该应用")
			return
		}
		// 未传则用卡类默认值
		if req.DurationSeconds == 0 {
			req.DurationSeconds = ct.DurationSeconds
		}
		if req.MaxUses == 0 {
			req.MaxUses = ct.MaxUses
		}
		if req.Prefix == "" {
			req.Prefix = ct.Name[:min(16, len(ct.Name))] // 取卡类名前 16 字符作前缀
		}

		// 套餐配额校验
		if err := quota.CheckMaxCards(deps.DB, tenantID, len(req.Cards)); err != nil {
			var qErr *quota.ExceededError
			if errors.As(err, &qErr) {
				middleware.Fail(c, http.StatusForbidden, 1007,
					"将超过套餐卡密上限 "+itoa(qErr.Limit)+" 张（当前 "+itoa(qErr.Current)+" 张，本次导入 "+itoa(qErr.AddCount)+" 张）")
			} else {
				middleware.Fail(c, http.StatusForbidden, 1007, err.Error())
			}
			return
		}

		// 去重 + 校验空值
		seen := make(map[string]struct{}, len(req.Cards))
		cleaned := make([]string, 0, len(req.Cards))
		emptyCount := 0
		dupCount := 0
		for _, k := range req.Cards {
			k = strings.TrimSpace(k)
			if k == "" {
				emptyCount++
				continue
			}
			if _, ok := seen[k]; ok {
				dupCount++
				continue
			}
			seen[k] = struct{}{}
			cleaned = append(cleaned, k)
		}
		if len(cleaned) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "无有效卡密可导入（全部为空或重复）")
			return
		}

		batchNo := fmt.Sprintf("I%s%06d", time.Now().Format("20060102"), userID%1000000)

		// 事务批量入库（按 CardKeyHash 计算，重复 hash 直接跳过并记失败）
		failed := make([]map[string]interface{}, 0)
		var successCount int
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			for i, k := range cleaned {
				hash := crypto.SHA512Hex(k)
				// 检查 hash 是否已存在（跨租户唯一）
				var exists int64
				if err := tx.Model(&model.AppCard{}).Where("card_key_hash = ?", hash).Count(&exists).Error; err != nil {
					return fmt.Errorf("查询卡密 hash 失败: %w", err)
				}
				if exists > 0 {
					failed = append(failed, map[string]interface{}{
						"row":      i + 1,
						"card_key": k,
						"reason":   "卡密已存在",
					})
					continue
				}
				card := &model.AppCard{
					TenantID:        tenantID,
					AppID:           req.AppID,
					CardTypeID:      req.CardTypeID,
					CardKey:         k,
					CardKeyHash:     hash,
					Checksum:        crypto.SHA512Checksum8(k + hash),
					Status:          "unused",
					BatchNo:         batchNo,
					Prefix:          req.Prefix,
					GroupTag:        req.GroupTag,
					DurationSeconds: req.DurationSeconds,
					MaxUses:         req.MaxUses,
					CreatedBy:       userID,
					CreatorType:     "tenant",
				}
				if err := tx.Create(card).Error; err != nil {
					failed = append(failed, map[string]interface{}{
						"row":      i + 1,
						"card_key": k,
						"reason":   "入库失败: " + err.Error(),
					})
					continue
				}
				successCount++
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "导入失败: "+txErr.Error())
			return
		}

		// 记录操作日志
		RecordOperation(deps, c, "card", "import_csv", "success",
			"tenant", nil, map[string]interface{}{
				"tenant_id":      tenantID,
				"batch_no":       batchNo,
				"success_count":  successCount,
				"failed_count":   len(failed),
				"empty_count":    emptyCount,
				"dup_count":      dupCount,
			})

		middleware.Success(c, gin.H{
			"batch_no":      batchNo,
			"success_count": successCount,
			"failed_count":  len(failed),
			"empty_count":   emptyCount,
			"dup_count":     dupCount,
			"failed":        failed,
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 标记未使用导入（防编译报错）
var _ = context.Background
var _ = strings.TrimSpace
