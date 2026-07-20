// 代理控制台业务接口 Handler
// 代理角色 JWT claims: user_id(agent.id) / tenant_id(开发者 id) / username / role='agent'
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取
// 严格遵循铁律 06：不确定处标注「待核实」
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

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/multilevel"
	"github.com/your-org/keyauth-saas/apps/server/internal/quota"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 公共 DTO ==============

// agentDashboardRecentOrder 工作台最近订单（联表 app + app_card_type）
type agentDashboardRecentOrder struct {
	OrderID      uint64    `json:"order_id"`
	OrderNo      string    `json:"order_no"`
	AppID        uint64    `json:"app_id"`
	AppName      string    `json:"app_name"`
	CardTypeID   uint64    `json:"card_type_id"`
	CardTypeName string    `json:"card_type_name"`
	Quantity     int       `json:"quantity"`
	TotalAmount  float64   `json:"total_amount"`
	PayStatus    string    `json:"pay_status"`
	PayChannel   string    `json:"pay_channel"`
	CreatedAt    time.Time `json:"created_at"`
}

// agentProfile /agent/auth/me 返回结构
type agentProfile struct {
	AgentID         uint64     `json:"agent_id"`
	Username        string     `json:"username"`
	RealName        string     `json:"real_name"`
	Phone           string     `json:"phone"`
	Email           string     `json:"email"`
	Status          string     `json:"status"`
	Balance         float64    `json:"balance"`
	FrozenBalance   float64    `json:"frozen_balance"`
	CommissionRate  float64    `json:"commission_rate"`
	CommissionMode  string     `json:"commission_mode"`
	TotalCommission float64    `json:"total_commission"`
	TotalWithdraw   float64    `json:"total_withdraw"`
	TenantID        uint64     `json:"tenant_id"`
	TenantName      string     `json:"tenant_name"`
	InviterID       *uint64    `json:"inviter_id"`
	InviterUsername string     `json:"inviter_username"`
	TOTPEnabled     bool       `json:"totp_enabled"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	LastLoginIP     string     `json:"last_login_ip"`
	CreatedAt       time.Time  `json:"created_at"`
}

// agentCardTypeItem 代理可购买卡类列表项
// 注：agent_base_price 直接复用 AppCardType.AgentBasePrice（不重复定义，避免 JSON 字段冲突）
type agentCardTypeItem struct {
	model.AppCardType
	AppName         string  `json:"app_name"`
	AgentCommission float64 `json:"agent_commission"`
}

// agentCardItem 代理卡密列表项（联表 app + app_card_type）
type agentCardItem struct {
	model.AppCard
	AppName      string `json:"app_name"`
	CardTypeName string `json:"card_type_name"`
}

// agentOrderItem 代理订单列表项（联表 app + app_card_type + agent_commission）
type agentOrderItem struct {
	model.AppOrder
	AppName           string  `json:"app_name"`
	CardTypeName      string  `json:"card_type_name"`
	SettledCommission float64 `json:"commission_amount" gorm:"column:settled_commission"`
}

// agentBalanceLogItem 代理流水明细项（联表 agent_withdraw + app_order）
type agentBalanceLogItem struct {
	model.AgentBalanceLog
	RelatedOrderNo  string `json:"related_order_no"`
	WithdrawMethod  string `json:"withdraw_method"`
	WithdrawAccount string `json:"withdraw_account"`
}

// agentNoticeItem 代理消息通知项
type agentNoticeItem struct {
	ID        uint64     `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Pinned    bool       `json:"pinned"`
	PublishAt time.Time  `json:"publish_at"`
	ExpireAt  *time.Time `json:"expire_at"`
	Read      bool       `json:"read"`
	CreatedAt time.Time  `json:"created_at"`
}

// ============== 公共辅助 ==============

// agentFrozenBalance 代理冻结余额（pending 提现总额）
func agentFrozenBalance(db *gorm.DB, agentID uint64) float64 {
	var v float64
	db.Model(&model.AgentWithdraw{}).
		Where("agent_id = ? AND status = ?", agentID, "pending").
		Select("COALESCE(SUM(amount), 0)").Scan(&v)
	return v
}

// agentTotalCommission 代理累计佣金（不含 rejected）
func agentTotalCommission(db *gorm.DB, agentID uint64) float64 {
	var v float64
	db.Model(&model.AgentCommission{}).
		Where("agent_id = ? AND settle_status != ?", agentID, "rejected").
		Select("COALESCE(SUM(amount), 0)").Scan(&v)
	return v
}

// agentTotalWithdrawPaid 代理累计已打款提现
func agentTotalWithdrawPaid(db *gorm.DB, agentID uint64) float64 {
	var v float64
	db.Model(&model.AgentWithdraw{}).
		Where("agent_id = ? AND status = ?", agentID, "paid").
		Select("COALESCE(SUM(amount), 0)").Scan(&v)
	return v
}

// ============== 1. AgentDashboard 代理工作台 ==============

// AgentDashboard GET /api/v1/agent/dashboard
// 一次返回余额、冻结、今日/累计购卡与消费、佣金、提现统计、最近 10 个订单
func AgentDashboard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		// 1. 查 agent 表
		var agent model.Agent
		if err := deps.DB.Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "代理账号不存在或无权访问")
			return
		}

		// 2. 聚合统计
		frozenBalance := agentFrozenBalance(deps.DB, agentID)
		totalCommission := agentTotalCommission(deps.DB, agentID)
		totalWithdraw := agentTotalWithdrawPaid(deps.DB, agentID)

		var pendingWithdraw float64
		deps.DB.Model(&model.AgentWithdraw{}).
			Where("agent_id = ? AND status = ?", agentID, "pending").
			Select("COALESCE(SUM(amount), 0)").Scan(&pendingWithdraw)

		// 今日购卡/消费：purchased = 订单总额，spent = 净成本（总额 - 佣金抵扣）
		var todayPurchased, todaySpent, totalPurchased, totalSpent float64
		today := time.Now().Format("2006-01-02")
		deps.DB.Model(&model.AppOrder{}).
			Where("agent_id = ? AND DATE(created_at) = ?", agentID, today).
			Select("COALESCE(SUM(total_amount), 0)").Scan(&todayPurchased)
		deps.DB.Model(&model.AppOrder{}).
			Where("agent_id = ? AND DATE(created_at) = ?", agentID, today).
			Select("COALESCE(SUM(total_amount - commission_amount), 0)").Scan(&todaySpent)

		deps.DB.Model(&model.AppOrder{}).
			Where("agent_id = ?", agentID).
			Select("COALESCE(SUM(total_amount), 0)").Scan(&totalPurchased)
		deps.DB.Model(&model.AppOrder{}).
			Where("agent_id = ?", agentID).
			Select("COALESCE(SUM(total_amount - commission_amount), 0)").Scan(&totalSpent)

		// 3. 最近 10 个订单（联表 app + app_card_type）
		var recentOrders []agentDashboardRecentOrder
		deps.DB.Table("app_order AS o").
			Select("o.id AS order_id, o.order_no, o.app_id, app.name AS app_name, "+
				"o.card_type_id, ct.name AS card_type_name, o.quantity, o.total_amount, "+
				"o.pay_status, o.pay_channel, o.created_at").
			Joins("LEFT JOIN app ON app.id = o.app_id").
			Joins("LEFT JOIN app_card_type AS ct ON ct.id = o.card_type_id").
			Where("o.agent_id = ?", agentID).
			Order("o.id DESC").
			Limit(10).
			Scan(&recentOrders)

		middleware.Success(c, gin.H{
			"agent_id":         agent.ID,
			"username":         agent.Username,
			"balance":          agent.Balance,
			"frozen_balance":   frozenBalance,
			"today_purchased":  todayPurchased,
			"today_spent":      todaySpent,
			"total_purchased":  totalPurchased,
			"total_spent":      totalSpent,
			"total_commission": totalCommission,
			"total_withdraw":   totalWithdraw,
			"pending_withdraw": pendingWithdraw,
			"recent_orders":    recentOrders,
		})
	}
}

// ============== 2. AgentMe 当前代理扩展信息 ==============

// AgentMe GET /api/v1/agent/auth/me
// 返回完整 AgentProfile，覆盖 auth.go 中只返回基础信息的 CurrentUser
// 注：路由注册时需将 /agent/auth/me 由 handler.CurrentUser 改为 handler.AgentMe
//     本文件不修改 router.go，需开发者同步调整路由注册
func AgentMe(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		var agent model.Agent
		if err := deps.DB.Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "代理账号不存在或无权访问")
			return
		}

		// 联表 sys_tenant 拿 tenant_name（开发者 username）
		var tenantName string
		deps.DB.Model(&model.SysTenant{}).
			Where("id = ?", tenantID).
			Select("username").Scan(&tenantName)

		// 联表邀请人用户名（如有）
		inviterUsername := ""
		if agent.InviterID != nil {
			deps.DB.Model(&model.Agent{}).
				Where("id = ?", *agent.InviterID).
				Select("username").Scan(&inviterUsername)
		}

		frozenBalance := agentFrozenBalance(deps.DB, agentID)
		totalCommission := agentTotalCommission(deps.DB, agentID)
		totalWithdraw := agentTotalWithdrawPaid(deps.DB, agentID)

		middleware.Success(c, agentProfile{
			AgentID:         agent.ID,
			Username:        agent.Username,
			RealName:        agent.RealName,
			Phone:           agent.Phone,
			Email:           agent.Email,
			Status:          agent.Status,
			Balance:         agent.Balance,
			FrozenBalance:   frozenBalance,
			CommissionRate:  agent.CommissionRate,
			CommissionMode:  agent.CommissionMode,
			TotalCommission: totalCommission,
			TotalWithdraw:   totalWithdraw,
			TenantID:        agent.TenantID,
			TenantName:      tenantName,
			InviterID:       agent.InviterID,
			InviterUsername: inviterUsername,
			TOTPEnabled:     agent.TOTPSecret != "",
			LastLoginAt:     agent.LastLoginAt,
			LastLoginIP:     agent.LastLoginIP,
			CreatedAt:       agent.CreatedAt,
		})
	}
}

// ============== 3. AgentListCardTypes 代理可购买卡类列表 ==============

// AgentListCardTypes GET /api/v1/agent/card_types
// 查询当前 tenant 下 active 状态的卡类，附 app_name 与代理字段
func AgentListCardTypes(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		// 查 agent 拿 commission_rate（percentage 模式下计算佣金用）
		var agent model.Agent
		if err := deps.DB.Select("id, commission_rate, status").
			Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "代理账号不存在或无权访问")
			return
		}
		if agent.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "代理账号已被禁用")
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

		q := deps.DB.Model(&model.AppCardType{}).
			Where("tenant_id = ? AND status = ?", tenantID, "active")
		if appIDStr := c.Query("app_id"); appIDStr != "" {
			if appID, err := strconv.ParseUint(appIDStr, 10, 64); err == nil && appID > 0 {
				q = q.Where("app_id = ?", appID)
			}
		}

		var total int64
		q.Count(&total)

		var types []model.AppCardType
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&types).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询卡类失败")
			return
		}

		// 批量查询 app（name + agent_commission_mode）
		appIDs := make(map[uint64]struct{}, len(types))
		for _, t := range types {
			appIDs[t.AppID] = struct{}{}
		}
		type appInfo struct {
			Name string
			Mode string
		}
		appInfoMap := make(map[uint64]appInfo, len(appIDs))
		if len(appIDs) > 0 {
			ids := make([]uint64, 0, len(appIDs))
			for id := range appIDs {
				ids = append(ids, id)
			}
			var apps []model.App
			deps.DB.Select("id, name, agent_commission_mode").Where("id IN ?", ids).Find(&apps)
			for _, a := range apps {
				appInfoMap[a.ID] = appInfo{Name: a.Name, Mode: a.AgentCommissionMode}
			}
		}

		list := make([]agentCardTypeItem, 0, len(types))
		for _, t := range types {
			info := appInfoMap[t.AppID]
			item := agentCardTypeItem{
				AppCardType: t,
				AppName:     info.Name,
			}
			// percentage 模式 commission = price * rate / 100，rate 取 agent.CommissionRate
			// diff 模式 commission = price - agent_base_price
			if info.Mode == "percentage" {
				item.AgentCommission = t.Price * agent.CommissionRate / 100
			} else {
				item.AgentCommission = t.Price - t.AgentBasePrice
			}
			list = append(list, item)
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 4. AgentListCards 代理卡密列表 ==============

// AgentListCards GET /api/v1/agent/cards
// 查询当前代理生成的卡密（creator_type='agent' AND created_by=当前 agent_id）
func AgentListCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 20
		}

		q := deps.DB.Model(&model.AppCard{}).
			Where("tenant_id = ? AND creator_type = ? AND created_by = ?",
				tenantID, "agent", agentID)
		if appIDStr := c.Query("app_id"); appIDStr != "" {
			if appID, err := strconv.ParseUint(appIDStr, 10, 64); err == nil && appID > 0 {
				q = q.Where("app_id = ?", appID)
			}
		}
		if ctIDStr := c.Query("card_type_id"); ctIDStr != "" {
			if ctID, err := strconv.ParseUint(ctIDStr, 10, 64); err == nil && ctID > 0 {
				q = q.Where("card_type_id = ?", ctID)
			}
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}
		if batchNo := c.Query("batch_no"); batchNo != "" {
			q = q.Where("batch_no = ?", batchNo)
		}

		var total int64
		q.Count(&total)

		var cards []model.AppCard
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&cards).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询卡密失败")
			return
		}

		// 批量查询 app_name + card_type_name
		appIDs := make(map[uint64]struct{}, len(cards))
		ctIDs := make(map[uint64]struct{}, len(cards))
		for _, card := range cards {
			appIDs[card.AppID] = struct{}{}
			ctIDs[card.CardTypeID] = struct{}{}
		}
		appNameMap := make(map[uint64]string, len(appIDs))
		ctNameMap := make(map[uint64]string, len(ctIDs))
		if len(appIDs) > 0 {
			ids := make([]uint64, 0, len(appIDs))
			for id := range appIDs {
				ids = append(ids, id)
			}
			var apps []model.App
			deps.DB.Select("id, name").Where("id IN ?", ids).Find(&apps)
			for _, a := range apps {
				appNameMap[a.ID] = a.Name
			}
		}
		if len(ctIDs) > 0 {
			ids := make([]uint64, 0, len(ctIDs))
			for id := range ctIDs {
				ids = append(ids, id)
			}
			var cts []model.AppCardType
			deps.DB.Select("id, name").Where("id IN ?", ids).Find(&cts)
			for _, ct := range cts {
				ctNameMap[ct.ID] = ct.Name
			}
		}

		list := make([]agentCardItem, 0, len(cards))
		for _, card := range cards {
			list = append(list, agentCardItem{
				AppCard:      card,
				AppName:      appNameMap[card.AppID],
				CardTypeName: ctNameMap[card.CardTypeID],
			})
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 5. AgentGenerateCards 代理购卡 ==============

type agentGenerateCardsReq struct {
	CardTypeID uint64 `json:"card_type_id" binding:"required"`
	Quantity   int    `json:"quantity" binding:"required,min=1,max=1000"`
	Prefix     string `json:"prefix" binding:"omitempty,max=16"`
	GroupTag   string `json:"group_tag" binding:"omitempty,max=64"`
}

// AgentGenerateCards POST /api/v1/agent/cards/generate
// 流程：校验 → 扣余额 → 生成卡密 → 实时结算佣金 → 返回卡密明文
func AgentGenerateCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		var req agentGenerateCardsReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 1. 校验卡类
		var ct model.AppCardType
		if err := deps.DB.Where("id = ? AND tenant_id = ? AND status = ?",
			req.CardTypeID, tenantID, "active").First(&ct).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "卡类不存在、已下架或无权访问")
			return
		}

		// 2. 校验应用状态
		var app model.App
		if err := deps.DB.Select("id, status, agent_commission_mode").
			Where("id = ? AND tenant_id = ?", ct.AppID, tenantID).First(&app).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "应用不存在或无权访问")
			return
		}
		if app.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "应用已被禁用")
			return
		}

		// 3. 校验代理
		var agent model.Agent
		if err := deps.DB.Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "代理账号不存在或无权访问")
			return
		}
		if agent.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "代理账号已被禁用")
			return
		}

		// 4. 计算总价 + 校验余额
		costTotal := ct.AgentBasePrice * float64(req.Quantity)
		if costTotal < 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "卡类代理底价配置异常")
			return
		}
		if agent.Balance < costTotal {
			middleware.Fail(c, http.StatusForbidden, 1007,
				"余额不足，当前余额 "+strconv.FormatFloat(agent.Balance, 'f', 2, 64)+
					"，本次需扣 "+strconv.FormatFloat(costTotal, 'f', 2, 64))
			return
		}

		// 5. 计算实时佣金
		// percentage: commission = price * quantity * rate / 100
		// diff: commission = (price - agent_base_price) * quantity
		var commission float64
		switch app.AgentCommissionMode {
		case "percentage":
			commission = ct.Price * float64(req.Quantity) * agent.CommissionRate / 100
		default: // diff
			commission = (ct.Price - ct.AgentBasePrice) * float64(req.Quantity)
		}
		// diff 模式下 price<agent_base_price 视为开发者配置异常，佣金按 0 计（不向代理倒扣）
		if commission < 0 {
			commission = 0
		}

		// 6. 生成批次号
		batchNo := fmt.Sprintf("AB%s%06d", time.Now().Format("20060102"), agentID%1000000)

		// 7. 事务：扣余额 → 生成卡密 → 写 deduct 流水 → 加回佣金 → 写 commission 流水
	var (
		cardKeys         []string
		cardIDs          []uint64
		balanceAfter     float64
		crossCommissions []multilevel.CrossCommissionResult // v0.4.0：跨级佣金明细
	)
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 7.1 扣代理余额（用 SQL 表达式避免并发覆盖）
			if err := tx.Model(&model.Agent{}).Where("id = ?", agentID).
				UpdateColumn("balance", gorm.Expr("balance - ?", costTotal)).Error; err != nil {
				return fmt.Errorf("扣减代理余额失败: %w", err)
			}
			agent.Balance -= costTotal

			// 7.2 批量生成卡密（参考 card.go TenantGenerateCards）
			cardKeys = make([]string, 0, req.Quantity)
			cardIDs = make([]uint64, 0, req.Quantity)
			for i := 0; i < req.Quantity; i++ {
				key, hash, checksum, err := crypto.GenerateCardKey(req.Prefix)
				if err != nil {
					return fmt.Errorf("生成第 %d 张卡密失败: %w", i+1, err)
				}
				card := &model.AppCard{
					TenantID:        tenantID,
					AppID:           ct.AppID,
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
					CreatedBy:       agentID,
					CreatorType:     "agent",
				}
				if err := tx.Create(card).Error; err != nil {
					return fmt.Errorf("入库第 %d 张卡密失败: %w", i+1, err)
				}
				cardKeys = append(cardKeys, key)
				cardIDs = append(cardIDs, card.ID)
			}

			// 7.3 写扣款流水（type='deduct'）
			cardIDsJSON, _ := json.Marshal(cardIDs)
			deductLog := &model.AgentBalanceLog{
				AgentID:        agentID,
				TenantID:       tenantID,
				Type:           "deduct",
				Amount:         -costTotal,
				BalanceAfter:   agent.Balance,
				RelatedCardIDs: string(cardIDsJSON),
				Status:         "settled",
				Remark:         "代理购卡",
			}
			if err := tx.Create(deductLog).Error; err != nil {
				return fmt.Errorf("写入扣款流水失败: %w", err)
			}

			// 7.4 实时结算佣金（加回 balance）
		if commission > 0 {
			if err := tx.Model(&model.Agent{}).Where("id = ?", agentID).
				UpdateColumn("balance", gorm.Expr("balance + ?", commission)).Error; err != nil {
				return fmt.Errorf("结算佣金失败: %w", err)
			}
			agent.Balance += commission

			// 写佣金流水（type='commission'）
			commissionLog := &model.AgentBalanceLog{
				AgentID:        agentID,
				TenantID:       tenantID,
				Type:           "commission",
				Amount:         commission,
				BalanceAfter:   agent.Balance,
				RelatedCardIDs: string(cardIDsJSON),
				Status:         "settled",
				Remark:         "代理购卡佣金",
			}
			if err := tx.Create(commissionLog).Error; err != nil {
				return fmt.Errorf("写入佣金流水失败: %w", err)
			}

			// 7.5 v0.4.0 多级代理跨级佣金分发
			//   沿 parent_id 链向上传递（level 2 → 父级 level 1；level 3 → 父级 level 2 + 祖父级 level 1）
			//   父级非 active 自动跳过；比例从 sys_config 读取
			crossResults, crossErr := multilevel.DistributeCrossCommission(
				c.Request.Context(), tx, deps.CfgCache, &agent, commission, string(cardIDsJSON),
			)
			if crossErr != nil {
				return fmt.Errorf("跨级佣金分发失败: %w", crossErr)
			}
			crossCommissions = crossResults
		}

		balanceAfter = agent.Balance
		return nil
	})
	if txErr != nil {
		middleware.Fail(c, http.StatusInternalServerError, 5003, "代理购卡失败: "+txErr.Error())
		return
	}

	resp := gin.H{
		"batch_no":      batchNo,
		"quantity":      req.Quantity,
		"card_keys":     cardKeys,
		"card_ids":      cardIDs,
		"cost_total":    costTotal,
		"commission":    commission,
		"balance_after": balanceAfter,
		"warn":          "卡密明文仅本次返回一次，请立即保存或导出",
	}
	// v0.4.0：跨级佣金明细（仅当有上级代理被结算时返回）
	if len(crossCommissions) > 0 {
		resp["cross_commissions"] = crossCommissions
	}
	middleware.Success(c, resp)
	}
}

// ============== 6. AgentListOrders 代理订单列表 ==============

// AgentListOrders GET /api/v1/agent/orders
// 查询关联到当前代理的订单，附 app/card_type 名称与已结算佣金
func AgentListOrders(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
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

		q := deps.DB.Table("app_order AS o").
			Select("o.*, app.name AS app_name, ct.name AS card_type_name, "+
				"(SELECT COALESCE(SUM(ac.amount), 0) FROM agent_commission ac "+
				"WHERE ac.order_id = o.id AND ac.settle_status != 'rejected') AS settled_commission").
			Joins("LEFT JOIN app ON app.id = o.app_id").
			Joins("LEFT JOIN app_card_type AS ct ON ct.id = o.card_type_id").
			Where("o.agent_id = ?", agentID)
		if status := c.Query("status"); status != "" {
			q = q.Where("o.pay_status = ?", status)
		}

		var total int64
		q.Count(&total)

		var list []agentOrderItem
		if err := q.Order("o.id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Scan(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询订单失败")
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

// ============== 7. AgentListCommission 佣金/流水明细 ==============

// AgentListCommission GET /api/v1/agent/commission
// 返回 agent_balance_log 流水，type 字段直接透传（deduct/commission/withdraw/recharge/adjust）
// 联表 app_order 拿 related_order_no；type='withdraw' 时透传 pay_method/pay_voucher
func AgentListCommission(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
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

		q := deps.DB.Model(&model.AgentBalanceLog{}).
			Where("agent_id = ? AND tenant_id = ?", agentID, tenantID)
		if t := c.Query("type"); t != "" {
			q = q.Where("type = ?", t)
		}
		if s := c.Query("status"); s != "" {
			q = q.Where("status = ?", s)
		}

		var total int64
		q.Count(&total)

		var logs []model.AgentBalanceLog
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&logs).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询流水失败")
			return
		}

		// 批量查询 related_order_no
		orderIDs := make([]uint64, 0, len(logs))
		for _, l := range logs {
			if l.RelatedOrderID != nil {
				orderIDs = append(orderIDs, *l.RelatedOrderID)
			}
		}
		orderNoMap := make(map[uint64]string, len(orderIDs))
		if len(orderIDs) > 0 {
			var orders []model.AppOrder
			deps.DB.Select("id, order_no").Where("id IN ?", orderIDs).Find(&orders)
			for _, o := range orders {
				orderNoMap[o.ID] = o.OrderNo
			}
		}

		list := make([]agentBalanceLogItem, 0, len(logs))
		for _, l := range logs {
			item := agentBalanceLogItem{
				AgentBalanceLog: l,
			}
			if l.RelatedOrderID != nil {
				item.RelatedOrderNo = orderNoMap[*l.RelatedOrderID]
			}
			// balance_log 已在写入时存了 pay_method/pay_voucher，直接透传
			if l.Type == "withdraw" {
				item.WithdrawMethod = l.PayMethod
				item.WithdrawAccount = l.PayVoucher
			}
			list = append(list, item)
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 8. AgentWithdraw 提现申请 ==============

type agentWithdrawReq struct {
	Amount   float64 `json:"amount" binding:"required,gt=0"`
	Method   string  `json:"method" binding:"required,oneof=alipay wechat bank"`
	Account  string  `json:"account" binding:"required,max=128"`
	RealName string  `json:"real_name" binding:"omitempty,max=64"`
	Remark   string  `json:"remark" binding:"omitempty,max=255"`
}

// AgentWithdraw POST /api/v1/agent/withdraw
// 流程：校验余额/最小金额 → 事务扣余额 → 写 agent_withdraw → 写 balance_log
func AgentWithdraw(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		var req agentWithdrawReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		var agent model.Agent
		if err := deps.DB.Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "代理账号不存在或无权访问")
			return
		}
		if agent.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "代理账号已被禁用")
			return
		}
		if agent.Balance < req.Amount {
			middleware.Fail(c, http.StatusForbidden, 1007,
				"余额不足，当前余额 "+strconv.FormatFloat(agent.Balance, 'f', 2, 64))
			return
		}

		// 校验最小提现金额（从 sys_config 读取，铁律 05）
		minAmount := deps.CfgCache.GetFloat64(ctx, "agent.withdraw.min_amount", 10.00)
		if req.Amount < minAmount {
			middleware.Fail(c, http.StatusBadRequest, 1001,
				"提现金额不能少于 "+strconv.FormatFloat(minAmount, 'f', 2, 64))
			return
		}

		// 拼装 remark：把 real_name 一并记入流水（agent_withdraw 表无 real_name 字段，记入 audit_remark）
		balanceRemark := strings.TrimSpace(req.Remark)
		auditRemark := ""
		if req.RealName != "" {
			auditRemark = "实名:" + req.RealName
			if balanceRemark != "" {
				balanceRemark = auditRemark + "; " + balanceRemark
			} else {
				balanceRemark = auditRemark
			}
		}

		// 事务：扣余额 → 写 agent_withdraw → 写 balance_log
		var withdrawID uint64
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 1. 扣余额（进入冻结）
			if err := tx.Model(&model.Agent{}).Where("id = ?", agentID).
				UpdateColumn("balance", gorm.Expr("balance - ?", req.Amount)).Error; err != nil {
				return fmt.Errorf("扣减余额失败: %w", err)
			}
			agent.Balance -= req.Amount

			// 2. 写 agent_withdraw（real_name 写入 audit_remark 字段持久化）
			wd := &model.AgentWithdraw{
				AgentID:      agentID,
				TenantID:     tenantID,
				Amount:       req.Amount,
				PayMethod:    req.Method,
				PayAccount:   req.Account,
				Status:       "pending",
				AuditRemark:  auditRemark,
			}
			if err := tx.Create(wd).Error; err != nil {
				return fmt.Errorf("创建提现记录失败: %w", err)
			}
			withdrawID = wd.ID

			// 3. 写 balance_log（type='withdraw', status='pending'）
			log := &model.AgentBalanceLog{
				AgentID:      agentID,
				TenantID:     tenantID,
				Type:         "withdraw",
				Amount:       -req.Amount,
				BalanceAfter: agent.Balance,
				PayMethod:    req.Method,
				PayVoucher:   req.Account,
				Status:       "pending",
				Remark:       balanceRemark,
			}
			if err := tx.Create(log).Error; err != nil {
				return fmt.Errorf("写入流水失败: %w", err)
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "提现申请失败: "+txErr.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":     withdrawID,
			"status": "pending",
			"amount": req.Amount,
		})
	}
}

// ============== 9. AgentRecharge 充值申请 ==============

// agentRechargeReq 代理充值申请请求
type agentRechargeReq struct {
	Amount     float64 `json:"amount" binding:"required,gt=0"`
	PayMethod  string  `json:"pay_method" binding:"required,oneof=alipay wechat bank manual"`
	PayVoucher string  `json:"pay_voucher" binding:"omitempty,max=255"`
	Remark     string  `json:"remark" binding:"omitempty,max=255"`
}

// AgentRecharge POST /api/v1/agent/recharge
// 流程：校验金额上限 → 写 agent_balance_log(type=recharge, status=pending) → 等待开发者审核
// 审核通过后由开发者通过 AdminUpdateAgent 或后续审批接口调整 balance 并将本流水置为 settled
func AgentRecharge(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		var req agentRechargeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		var agent model.Agent
		if err := deps.DB.Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "代理账号不存在或无权访问")
			return
		}
		if agent.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "代理账号已被禁用")
			return
		}

		// 非手工支付方式必须上传付款凭证
		if req.PayMethod != "manual" && req.PayVoucher == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "请上传付款凭证")
			return
		}

		// 校验单笔充值上下限（从 sys_config 读取，铁律 05）
		minAmount := deps.CfgCache.GetFloat64(ctx, "agent.recharge.min_amount", 1.00)
		maxAmount := deps.CfgCache.GetFloat64(ctx, "agent.recharge.max_amount", 100000.00)
		if req.Amount < minAmount {
			middleware.Fail(c, http.StatusBadRequest, 1001,
				"充值金额不能少于 "+strconv.FormatFloat(minAmount, 'f', 2, 64))
			return
		}
		if req.Amount > maxAmount {
			middleware.Fail(c, http.StatusBadRequest, 1001,
				"单笔充值金额不能超过 "+strconv.FormatFloat(maxAmount, 'f', 2, 64))
			return
		}

		// 写入充值流水：status=pending，balance_after 保持当前余额（审核通过后再调整）
		log := &model.AgentBalanceLog{
			AgentID:      agentID,
			TenantID:     tenantID,
			Type:         "recharge",
			Amount:       req.Amount,
			BalanceAfter: agent.Balance,
			PayMethod:    req.PayMethod,
			PayVoucher:   req.PayVoucher,
			Status:       "pending",
			Remark:       req.Remark,
		}
		if err := deps.DB.Create(log).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "提交充值申请失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":           log.ID,
			"agent_id":     agentID,
			"amount":       req.Amount,
			"pay_method":   req.PayMethod,
			"status":       "pending",
			"created_at":   log.CreatedAt,
			"message":      "充值申请已提交，等待开发者审核",
		})
	}
}

// ============== 10. AgentListNotices 代理消息通知 ==============

// AgentListNotices GET /api/v1/agent/notices
// 查询平台公告 + 当前开发者公告 + 代理专属通知，附已读状态
func AgentListNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
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

		now := time.Now()
		// 代理可见公告：平台公告(type=platform, tenant_id IS NULL) + 开发者向公告(type=tenant, tenant_id=当前) + 代理专属(type=agent_notify)
		q := deps.DB.Model(&model.Notice{}).
			Where("status = ?", "published").
			Where("start_at <= ?", now).
			Where("end_at IS NULL OR end_at > ?", now).
			Where(
				"(type = ? AND tenant_id IS NULL) OR (type = ? AND tenant_id = ?) OR (type = ? AND tenant_id = ?)",
				"platform", "tenant", tenantID, "agent_notify", tenantID,
			)
		if t := c.Query("type"); t != "" {
			q = q.Where("type = ?", t)
		}
		// unread_only 过滤：通过子查询排除已读
		if c.Query("unread_only") == "1" || c.Query("unread_only") == "true" {
			q = q.Where(
				"id NOT IN (SELECT notice_id FROM notice_read WHERE user_type = ? AND user_id = ?)",
				"agent", agentID,
			)
		}

		var total int64
		q.Count(&total)

		var notices []model.Notice
		if err := q.Order("is_pinned DESC, created_at DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&notices).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询公告失败")
			return
		}

		// 批量查询已读状态
		noticeIDs := make([]uint64, 0, len(notices))
		for _, n := range notices {
			noticeIDs = append(noticeIDs, n.ID)
		}
		readMap := make(map[uint64]bool, len(noticeIDs))
		if len(noticeIDs) > 0 {
			var reads []model.NoticeRead
			deps.DB.Where("user_type = ? AND user_id = ? AND notice_id IN ?",
				"agent", agentID, noticeIDs).Find(&reads)
			for _, r := range reads {
				readMap[r.NoticeID] = true
			}
		}

		list := make([]agentNoticeItem, 0, len(notices))
		for _, n := range notices {
			list = append(list, agentNoticeItem{
				ID:        n.ID,
				Type:      n.Type,
				Title:     n.Title,
				Content:   n.Content,
				Pinned:    n.IsPinned,
				PublishAt: n.StartAt,
				ExpireAt:  n.EndAt,
				Read:      readMap[n.ID],
				CreatedAt: n.CreatedAt,
			})
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 11. AgentReadNotice 标记已读 ==============

// AgentReadNotice POST /api/v1/agent/notices/:id/read
// 幂等：重复调用不报错
func AgentReadNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}
		noticeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || noticeID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "公告 ID 格式错误")
			return
		}

		// 校验公告存在
		var notice model.Notice
		if err := deps.DB.First(&notice, noticeID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "公告不存在")
			return
		}

		// FirstOrCreate 幂等
		read := &model.NoticeRead{
			NoticeID: noticeID,
			UserType: "agent",
			UserID:   agentID,
		}
		if err := deps.DB.Where("notice_id = ? AND user_type = ? AND user_id = ?",
			noticeID, "agent", agentID).
			FirstOrCreate(read).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "标记已读失败")
			return
		}

		middleware.Success(c, gin.H{
			"notice_id": noticeID,
			"read":      true,
		})
	}
}

// ============== 12. v0.4.0 多级代理：下级邀请码管理 ==============

// agentGenInviteCodeReq 代理生成下级邀请码请求体
type agentGenInviteCodeReq struct {
	Count          int     `json:"count" binding:"required,min=1,max=50"`
	ExpireDays     int     `json:"expire_days" binding:"required,min=1,max=365"`
	CommissionRate float64 `json:"commission_rate" binding:"omitempty,min=0,max=100"`
}

// AgentGenInviteCode POST /api/v1/agent/invite_codes
// 代理生成下级邀请码（仅 level < max_level 且 agent_can_create=true 时允许）
func AgentGenInviteCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		var req agentGenInviteCodeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 1. 查代理（必须 active）
		var agent model.Agent
		if err := deps.DB.Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 1003, "代理账号不存在或无权访问")
			return
		}
		if agent.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 1005, "代理账号已被禁用")
			return
		}

		// 2. 校验多级代理资格（agent_can_create + level < max_level）
		ctx := c.Request.Context()
		if err := multilevel.CanCreateSubordinate(ctx, deps.CfgCache, &agent); err != nil {
			middleware.Fail(c, http.StatusForbidden, 1050, "无法创建下级邀请码: "+err.Error())
			return
		}

		// 3. 佣金比例（请求未传则用代理自己的 CommissionRate）
		rate := req.CommissionRate
		if rate <= 0 {
			rate = agent.CommissionRate
		}

		// 4. 套餐配额校验（quota.CheckMaxAgents 防止发放无效邀请码）
		if err := quota.CheckMaxAgents(deps.DB, tenantID); err != nil {
			middleware.Fail(c, http.StatusForbidden, 1008, err.Error())
			return
		}

		// 5. 事务内批量生成
		codes := make([]string, 0, req.Count)
		expiresAt := time.Now().AddDate(0, 0, req.ExpireDays)
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			for i := 0; i < req.Count; i++ {
				code, err := genInviteCodeUnique(tx)
				if err != nil {
					return err
				}
				ic := &model.AgentInviteCode{
					TenantID:              tenantID,
					Code:                  code,
					MaxUses:               1,
					UsedCount:             0,
					ValidDays:             req.ExpireDays,
					ExpiresAt:             expiresAt,
					Status:                "active",
					DefaultCommissionRate: rate,
					CreatedBy:             agentID,
					CreatorType:           "agent",      // v0.4.0：代理创建
					CreatorAgentID:        agentID,      // v0.4.0：创建者代理 ID
				}
				if err := tx.Create(ic).Error; err != nil {
					return fmt.Errorf("创建邀请码失败: %w", err)
				}
				codes = append(codes, code)
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "生成邀请码失败: "+txErr.Error())
			return
		}

		middleware.Success(c, gin.H{
			"codes":       codes,
			"count":       len(codes),
			"expire_days": req.ExpireDays,
			"expires_at":  expiresAt.Unix(),
			"creator":     "agent",
			"creator_id":  agentID,
			"level":       agent.Level,
		})
	}
}

// AgentListInviteCodes GET /api/v1/agent/invite_codes
// 代理查询自己创建的下级邀请码列表
func AgentListInviteCodes(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		status := c.Query("status")
		page, size := parsePagination(c)

		query := deps.DB.Model(&model.AgentInviteCode{}).
			Where("tenant_id = ? AND creator_type = ? AND creator_agent_id = ?",
				tenantID, "agent", agentID)
		if status != "" {
			query = query.Where("status = ?", status)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询邀请码总数失败")
			return
		}
		var codes []model.AgentInviteCode
		if err := query.Order("created_at DESC").
			Offset((page - 1) * size).Limit(size).Find(&codes).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询邀请码列表失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":  codes,
			"total": total,
			"page":  page,
			"size":  size,
		})
	}
}

// AgentDisableInviteCode POST /api/v1/agent/invite_codes/:id/disable
// 代理禁用自己创建的下级邀请码
func AgentDisableInviteCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}
		codeID, err := parseUintParam(c, "id")
		if err != nil || codeID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "邀请码 ID 无效")
			return
		}

		// 校验：必须是当前代理创建的邀请码
		var ic model.AgentInviteCode
		if err := deps.DB.Where("id = ? AND tenant_id = ? AND creator_type = ? AND creator_agent_id = ?",
			codeID, tenantID, "agent", agentID).First(&ic).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1004, "邀请码不存在或无权操作")
			return
		}
		if ic.Status == "exhausted" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "邀请码已用尽，无需禁用")
			return
		}
		if err := deps.DB.Model(&ic).Update("status", "disabled").Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "禁用邀请码失败")
			return
		}

		middleware.Success(c, gin.H{
			"id":     ic.ID,
			"status": "disabled",
		})
	}
}

// ============== 13. v0.4.0 多级代理：下级代理查询 ==============

// AgentListSubordinates GET /api/v1/agent/subordinates
// 代理查询直接下级代理列表（单层，不递归）
func AgentListSubordinates(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		subs, err := multilevel.ListSubordinates(c.Request.Context(), deps.DB, agentID, tenantID)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询下级代理失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"list":  subs,
			"total": len(subs),
		})
	}
}

// AgentGetTree GET /api/v1/agent/tree
// 代理查询自己的下级代理树（递归，最深 max_level-1 层）
func AgentGetTree(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}

		// 最深递归层数 = max_level - 1（例如 max_level=3 时，level=1 代理可下钻 2 层）
		maxLevel := int(deps.CfgCache.GetInt(c.Request.Context(), "agent.commission.max_level", 3))
		if maxLevel < 1 {
			maxLevel = 1
		}
		maxDepth := maxLevel - 1

		tree, err := multilevel.BuildAgentTree(c.Request.Context(), deps.DB, agentID, maxDepth)
		if err != nil {
			if errors.Is(err, multilevel.ErrAgentNotFound) {
				middleware.Fail(c, http.StatusNotFound, 1004, "代理账号不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5002, "构建代理树失败: "+err.Error())
			return
		}

		// 校验：树根必须属于当前 tenant（防越权）
		if tree.Agent.TenantID != tenantID {
			middleware.Fail(c, http.StatusForbidden, 1003, "无权访问该代理树")
			return
		}

		middleware.Success(c, gin.H{
			"tree": tree,
		})
	}
}

// ============== 标记未使用导入（防编译报错） ==============

var _ = context.Background
