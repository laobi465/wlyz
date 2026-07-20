// Package handler v0.4.x 残留项 1-4 H5 端单元测试
// 覆盖：
//   1. H5EndUserListOrders  - 终端用户订单列表（U-11）
//   2. H5EndUserGetOrder    - 终端用户订单详情（U-11）
//   3. PublicNoticeDetail   - 公告详情公开端点（U-12）
//   4. PublicContact        - 客服联系方式公开端点（U-14）
//
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试基础设施：SQLite 内存库 + miniredis（复用 notice_stats_test.go 中已有 helpers）
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 测试基础设施扩展 ==============

// setupH5EndUserDeps 在 setupNoticeStatsDeps 基础上额外迁移 AppCard / EndUser 表
// notice_stats_test.go 已迁移 AppOrder / Notice / SysConfig，此处仅需补 AppCard / EndUser
func setupH5EndUserDeps(t *testing.T, cfgOverrides map[string]string) *Deps {
	t.Helper()
	deps := setupNoticeStatsDeps(t, cfgOverrides)
	require.NoError(t, deps.DB.AutoMigrate(&model.AppCard{}, &model.EndUser{}))
	deps.DB.Exec("DELETE FROM app_card")
	deps.DB.Exec("DELETE FROM end_user")
	return deps
}

// seedEndUserOrder 创建终端用户订单（buyer_user_id 必填）
func seedEndUserOrder(t *testing.T, db *gorm.DB, id, tenantID, appID uint64, buyerUserID *uint64, orderNo, payStatus, cardIDs string) *model.AppOrder {
	t.Helper()
	o := &model.AppOrder{
		BaseModel:   model.BaseModel{ID: id},
		TenantID:    tenantID,
		AppID:       appID,
		OrderNo:     orderNo,
		BuyerUserID: buyerUserID,
		Quantity:    1,
		UnitPrice:   10.0,
		TotalAmount: 10.0,
		PayChannel:  "epay_alipay",
		PayStatus:   payStatus,
		CardIDs:     cardIDs,
	}
	require.NoError(t, db.Create(o).Error)
	return o
}

// seedAppCardForOrder 创建与订单关联的卡密
func seedAppCardForOrder(t *testing.T, db *gorm.DB, id, tenantID, appID uint64, cardKey, status string) *model.AppCard {
	t.Helper()
	c := &model.AppCard{
		BaseModel:   model.BaseModel{ID: id},
		TenantID:    tenantID,
		AppID:       appID,
		CardKey:     cardKey,
		CardKeyHash: "hash-" + cardKey,
		Checksum:    "cs",
		Status:      status,
	}
	require.NoError(t, db.Create(c).Error)
	return c
}

// callH5Endpoint 模拟 H5 鉴权上下文调用端点（注入 enduser_id）
// 返回响应体解析后的 map（成功响应的 data 字段）
func callH5Endpoint(t *testing.T, method, path string, handler gin.HandlerFunc, userID uint64) map[string]interface{} {
	t.Helper()
	g := setupGin()
	g.Handle(method, path, func(c *gin.Context) {
		c.Set("enduser_id", userID)
		c.Next()
	}, handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "接口应返回 200")

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code, "code 应为 0")
	return resp.Data
}

// callPublicEndpoint 调用公开端点（无需鉴权上下文）
func callPublicEndpoint(t *testing.T, method, path string, handler gin.HandlerFunc) map[string]interface{} {
	t.Helper()
	g := setupGin()
	g.Handle(method, path, handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "接口应返回 200")

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code, "code 应为 0")
	return resp.Data
}

// ============== 1. H5EndUserListOrders 测试 ==============

// TestH5EndUserListOrders_Empty 空列表返回 0 条
func TestH5EndUserListOrders_Empty(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	data := callH5Endpoint(t, "GET", "/h5/orders", H5EndUserListOrders(deps), 1001)
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, list)
	assert.Equal(t, float64(0), data["total"])
	assert.Equal(t, float64(1), data["page"])
	assert.Equal(t, float64(20), data["page_size"])
}

// TestH5EndUserListOrders_WithOrders 有订单时按 id DESC 返回
func TestH5EndUserListOrders_WithOrders(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	buyer := uint64(1001)
	seedEndUserOrder(t, deps.DB, 7001, 5001, 6001, &buyer, "ORD-7001", "paid", "[]")
	seedEndUserOrder(t, deps.DB, 7002, 5001, 6001, &buyer, "ORD-7002", "pending", "[]")
	seedEndUserOrder(t, deps.DB, 7003, 5001, 6001, &buyer, "ORD-7003", "closed", "[]")

	data := callH5Endpoint(t, "GET", "/h5/orders", H5EndUserListOrders(deps), 1001)
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	assert.Len(t, list, 3, "应返回 3 条订单")
	assert.Equal(t, float64(3), data["total"])

	// 按 id DESC 排序：第一条应为 7003
	first := list[0].(map[string]interface{})
	assert.Equal(t, "ORD-7003", first["order_no"])
	assert.Equal(t, "closed", first["pay_status"])

	third := list[2].(map[string]interface{})
	assert.Equal(t, "ORD-7001", third["order_no"])
}

// TestH5EndUserListOrders_StatusFilter 状态筛选正确过滤
func TestH5EndUserListOrders_StatusFilter(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	buyer := uint64(1001)
	seedEndUserOrder(t, deps.DB, 7001, 5001, 6001, &buyer, "ORD-7001", "paid", "[]")
	seedEndUserOrder(t, deps.DB, 7002, 5001, 6001, &buyer, "ORD-7002", "pending", "[]")
	seedEndUserOrder(t, deps.DB, 7003, 5001, 6001, &buyer, "ORD-7003", "paid", "[]")

	// status=paid 仅返回 2 条
	g := setupGin()
	g.GET("/h5/orders", func(c *gin.Context) {
		c.Set("enduser_id", uint64(1001))
		c.Next()
	}, H5EndUserListOrders(deps))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/h5/orders?status=paid", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	list := resp.Data["list"].([]interface{})
	assert.Len(t, list, 2, "status=paid 应返回 2 条")
	for _, item := range list {
		m := item.(map[string]interface{})
		assert.Equal(t, "paid", m["pay_status"])
	}
}

// TestH5EndUserListOrders_InvalidStatus 非法 status 返回 400
func TestH5EndUserListOrders_InvalidStatus(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	// 直接构造请求以正确处理 query string（callH5EndpointRaw 不支持 query）
	g := setupGin()
	g.GET("/h5/orders", func(c *gin.Context) {
		c.Set("enduser_id", uint64(1001))
		c.Next()
	}, H5EndUserListOrders(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/h5/orders?status=invalid", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEqual(t, 0, resp["code"], "非法 status 应返回非 0 业务码")
	assert.Equal(t, "无效的 status 参数", resp["message"])
}

// TestH5EndUserListOrders_UserScope 仅返回当前用户的订单
func TestH5EndUserListOrders_UserScope(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	buyer1 := uint64(1001)
	buyer2 := uint64(1002)
	seedEndUserOrder(t, deps.DB, 7001, 5001, 6001, &buyer1, "ORD-7001", "paid", "[]")
	seedEndUserOrder(t, deps.DB, 7002, 5001, 6001, &buyer2, "ORD-7002", "paid", "[]")
	seedEndUserOrder(t, deps.DB, 7003, 5001, 6001, &buyer1, "ORD-7003", "pending", "[]")

	// 用户 1001 调用：仅看到 7001 + 7003
	data := callH5Endpoint(t, "GET", "/h5/orders", H5EndUserListOrders(deps), 1001)
	list := data["list"].([]interface{})
	assert.Len(t, list, 2)
	assert.Equal(t, float64(2), data["total"])

	// 用户 1002 调用：仅看到 7002
	data = callH5Endpoint(t, "GET", "/h5/orders", H5EndUserListOrders(deps), 1002)
	list = data["list"].([]interface{})
	assert.Len(t, list, 1)
	assert.Equal(t, float64(1), data["total"])
	first := list[0].(map[string]interface{})
	assert.Equal(t, "ORD-7002", first["order_no"])
}

// ============== 2. H5EndUserGetOrder 测试 ==============

// TestH5EndUserGetOrder_NotFound 订单不存在返回 404
func TestH5EndUserGetOrder_NotFound(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	g := setupGin()
	g.GET("/h5/orders/:order_no", func(c *gin.Context) {
		c.Set("enduser_id", uint64(1001))
		c.Next()
	}, H5EndUserGetOrder(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/h5/orders/NOT-EXIST", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEqual(t, 0, resp["code"])
	assert.Equal(t, "订单不存在或不属于当前用户", resp["message"])
}

// TestH5EndUserGetOrder_NotOwner 订单不属于当前用户返回 404
func TestH5EndUserGetOrder_NotOwner(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	buyer1 := uint64(1001)
	seedEndUserOrder(t, deps.DB, 7001, 5001, 6001, &buyer1, "ORD-7001", "paid", "[]")

	// 用户 1002 试图查看 1001 的订单
	g := setupGin()
	g.GET("/h5/orders/:order_no", func(c *gin.Context) {
		c.Set("enduser_id", uint64(1002))
		c.Next()
	}, H5EndUserGetOrder(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/h5/orders/ORD-7001", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "订单不存在或不属于当前用户", resp["message"])
}

// TestH5EndUserGetOrder_PendingNoCards pending 订单不返回卡密
func TestH5EndUserGetOrder_PendingNoCards(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	buyer := uint64(1001)
	seedEndUserOrder(t, deps.DB, 7001, 5001, 6001, &buyer, "ORD-7001", "pending", "[]")

	g := setupGin()
	g.GET("/h5/orders/:order_no", func(c *gin.Context) {
		c.Set("enduser_id", uint64(1001))
		c.Next()
	}, H5EndUserGetOrder(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/h5/orders/ORD-7001", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	assert.Equal(t, "pending", resp.Data["pay_status"])

	// card_keys 和 cards 应为空数组
	cardKeys, ok := resp.Data["card_keys"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, cardKeys, "pending 订单不应返回卡密")

	cards, ok := resp.Data["cards"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, cards, "pending 订单不应返回卡密明细")
}

// TestH5EndUserGetOrder_PaidWithCards paid 订单返回卡密明文
func TestH5EndUserGetOrder_PaidWithCards(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	buyer := uint64(1001)
	// 创建 2 张卡密
	seedAppCardForOrder(t, deps.DB, 8001, 5001, 6001, "CARD-AAA", "active")
	seedAppCardForOrder(t, deps.DB, 8002, 5001, 6001, "CARD-BBB", "active")
	// 订单关联卡密 ID
	seedEndUserOrder(t, deps.DB, 7001, 5001, 6001, &buyer, "ORD-7001", "paid", "[8001,8002]")

	g := setupGin()
	g.GET("/h5/orders/:order_no", func(c *gin.Context) {
		c.Set("enduser_id", uint64(1001))
		c.Next()
	}, H5EndUserGetOrder(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/h5/orders/ORD-7001", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	assert.Equal(t, "paid", resp.Data["pay_status"])

	// card_ids 应为 [8001, 8002]
	cardIDs, ok := resp.Data["card_ids"].([]interface{})
	require.True(t, ok)
	assert.Len(t, cardIDs, 2)

	// card_keys 应为 ["CARD-AAA", "CARD-BBB"]
	cardKeys, ok := resp.Data["card_keys"].([]interface{})
	require.True(t, ok)
	require.Len(t, cardKeys, 2)
	assert.Equal(t, "CARD-AAA", cardKeys[0])
	assert.Equal(t, "CARD-BBB", cardKeys[1])

	// cards 数组每项应包含 id + card_key + status
	cards, ok := resp.Data["cards"].([]interface{})
	require.True(t, ok)
	require.Len(t, cards, 2)
	first := cards[0].(map[string]interface{})
	assert.Equal(t, "CARD-AAA", first["card_key"])
	assert.Equal(t, "active", first["status"])
}

// TestH5EndUserGetOrder_PaidNoCardIDs paid 订单但 card_ids 为空时返回空数组
func TestH5EndUserGetOrder_PaidNoCardIDs(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	buyer := uint64(1001)
	seedEndUserOrder(t, deps.DB, 7001, 5001, 6001, &buyer, "ORD-7001", "paid", "[]")

	g := setupGin()
	g.GET("/h5/orders/:order_no", func(c *gin.Context) {
		c.Set("enduser_id", uint64(1001))
		c.Next()
	}, H5EndUserGetOrder(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/h5/orders/ORD-7001", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	assert.Equal(t, "paid", resp.Data["pay_status"])
	cardKeys := resp.Data["card_keys"].([]interface{})
	assert.Empty(t, cardKeys, "card_ids 为空时 card_keys 也应为空")
}

// ============== 3. PublicNoticeDetail 测试 ==============

// TestPublicNoticeDetail_NotFound 公告不存在返回 404
func TestPublicNoticeDetail_NotFound(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	g := setupGin()
	g.GET("/public/notices/:id", PublicNoticeDetail(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/notices/9999", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(4001), resp["code"])
	assert.Equal(t, "公告不存在", resp["message"])
}

// TestPublicNoticeDetail_InvalidID 非法 ID 格式返回 400
func TestPublicNoticeDetail_InvalidID(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	g := setupGin()
	g.GET("/public/notices/:id", PublicNoticeDetail(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/notices/abc", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1001), resp["code"])
	assert.Equal(t, "公告 ID 格式错误", resp["message"])
}

// TestPublicNoticeDetail_DraftStatus 草稿状态不可访问
func TestPublicNoticeDetail_DraftStatus(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	n := &model.Notice{
		Type:    "platform",
		Title:   "草稿公告",
		Content: "draft",
		StartAt: time.Now().Add(-1 * time.Hour),
		Status:  "draft",
	}
	seedNotice(t, deps.DB, n)

	g := setupGin()
	g.GET("/public/notices/:id", PublicNoticeDetail(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/notices/"+uintToStr(n.ID), nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(4001), resp["code"])
	assert.Equal(t, "公告不存在或已下线", resp["message"])
}

// TestPublicNoticeDetail_NotStarted 未到 start_at 不可访问
func TestPublicNoticeDetail_NotStarted(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	n := &model.Notice{
		Type:    "platform",
		Title:   "未来公告",
		Content: "future",
		StartAt: time.Now().Add(1 * time.Hour), // 1 小时后开始
		Status:  "published",
	}
	seedNotice(t, deps.DB, n)

	g := setupGin()
	g.GET("/public/notices/:id", PublicNoticeDetail(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/notices/"+uintToStr(n.ID), nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(4001), resp["code"])
	assert.Equal(t, "公告尚未发布", resp["message"])
}

// TestPublicNoticeDetail_Expired 已过期不可访问
func TestPublicNoticeDetail_Expired(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	endAt := time.Now().Add(-1 * time.Hour)
	n := &model.Notice{
		Type:    "platform",
		Title:   "过期公告",
		Content: "expired",
		StartAt: time.Now().Add(-2 * time.Hour),
		EndAt:   &endAt,
		Status:  "published",
	}
	seedNotice(t, deps.DB, n)

	g := setupGin()
	g.GET("/public/notices/:id", PublicNoticeDetail(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/notices/"+uintToStr(n.ID), nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(4001), resp["code"])
	assert.Equal(t, "公告已过期", resp["message"])
}

// TestPublicNoticeDetail_IncrementsViewCount 浏览数 +1（并发安全）
func TestPublicNoticeDetail_IncrementsViewCount(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	n := &model.Notice{
		Type:          "platform",
		Title:         "正常公告",
		Content:       "<p>hello</p>",
		ContentFormat: "html",
		StartAt:       time.Now().Add(-1 * time.Hour),
		Status:        "published",
		ViewCount:     10,
	}
	seedNotice(t, deps.DB, n)

	// 调用 1 次
	g := setupGin()
	g.GET("/public/notices/:id", PublicNoticeDetail(deps))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/notices/"+uintToStr(n.ID), nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	assert.Equal(t, "正常公告", resp.Data["title"])
	assert.Equal(t, "<p>hello</p>", resp.Data["content"])
	assert.Equal(t, "html", resp.Data["content_format"])
	assert.Equal(t, float64(11), resp.Data["view_count"], "view_count 应从 10 增加到 11")

	// 验证数据库中的 view_count 也已 +1
	var updated model.Notice
	require.NoError(t, deps.DB.First(&updated, n.ID).Error)
	assert.Equal(t, 11, updated.ViewCount, "数据库 view_count 应为 11")

	// 再调用 1 次，view_count 应再次 +1
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/public/notices/"+uintToStr(n.ID), nil)
	g.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var resp2 struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	assert.Equal(t, float64(12), resp2.Data["view_count"], "第二次调用 view_count 应为 12")

	var updated2 model.Notice
	require.NoError(t, deps.DB.First(&updated2, n.ID).Error)
	assert.Equal(t, 12, updated2.ViewCount, "数据库 view_count 应为 12")
}

// TestPublicNoticeDetail_DisabledByConfig 总开关关闭时返回 403
func TestPublicNoticeDetail_DisabledByConfig(t *testing.T) {
	deps := setupH5EndUserDeps(t, map[string]string{
		CfgKeyNoticePublicDetailEnabled: "0",
	})

	n := &model.Notice{
		Type:    "platform",
		Title:   "正常公告",
		Content: "hello",
		StartAt: time.Now().Add(-1 * time.Hour),
		Status:  "published",
	}
	seedNotice(t, deps.DB, n)

	g := setupGin()
	g.GET("/public/notices/:id", PublicNoticeDetail(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/notices/"+uintToStr(n.ID), nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(4003), resp["code"])
	assert.Equal(t, "公告详情功能已关闭", resp["message"])
}

// TestPublicNoticeDetail_TextFormat text 格式公告正常返回
func TestPublicNoticeDetail_TextFormat(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	n := &model.Notice{
		Type:          "platform",
		Title:         "纯文本公告",
		Content:       "这是一条纯文本公告",
		ContentFormat: "text",
		StartAt:       time.Now().Add(-1 * time.Hour),
		Status:        "published",
		ViewCount:     0,
	}
	seedNotice(t, deps.DB, n)

	g := setupGin()
	g.GET("/public/notices/:id", PublicNoticeDetail(deps))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/notices/"+uintToStr(n.ID), nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	assert.Equal(t, "text", resp.Data["content_format"])
	assert.Equal(t, "这是一条纯文本公告", resp.Data["content"])
	assert.Equal(t, float64(1), resp.Data["view_count"], "view_count 从 0 增加到 1")
}

// ============== 4. PublicContact 测试 ==============

// TestPublicContact_Defaults 无配置时返回 4 个空字符串
func TestPublicContact_Defaults(t *testing.T) {
	deps := setupH5EndUserDeps(t, nil)

	data := callPublicEndpoint(t, "GET", "/public/contact", PublicContact(deps))
	assert.Equal(t, "", data["qq_group"])
	assert.Equal(t, "", data["wechat"])
	assert.Equal(t, "", data["email"])
	assert.Equal(t, "", data["phone"])
}

// TestPublicContact_WithConfig 有配置时返回对应值
func TestPublicContact_WithConfig(t *testing.T) {
	deps := setupH5EndUserDeps(t, map[string]string{
		CfgKeyContactQQGroup: "123456789",
		CfgKeyContactWechat:  "wechat_id_001",
		CfgKeyContactEmail:   "support@example.com",
		CfgKeyContactPhone:   "400-123-4567",
	})

	data := callPublicEndpoint(t, "GET", "/public/contact", PublicContact(deps))
	assert.Equal(t, "123456789", data["qq_group"])
	assert.Equal(t, "wechat_id_001", data["wechat"])
	assert.Equal(t, "support@example.com", data["email"])
	assert.Equal(t, "400-123-4567", data["phone"])
}

// TestPublicContact_PartialConfig 部分配置时仅返回已配置的字段
func TestPublicContact_PartialConfig(t *testing.T) {
	deps := setupH5EndUserDeps(t, map[string]string{
		CfgKeyContactQQGroup: "999999",
		// 其他 3 项不配置，应为空字符串
	})

	data := callPublicEndpoint(t, "GET", "/public/contact", PublicContact(deps))
	assert.Equal(t, "999999", data["qq_group"], "QQ 群应返回配置值")
	assert.Equal(t, "", data["wechat"], "未配置的应为空字符串")
	assert.Equal(t, "", data["email"])
	assert.Equal(t, "", data["phone"])
}

// ============== 5. 配置键常量校验 ==============

// TestH5ContactConfigKeys 配置键常量应与 sys_config 中的 key 一致
func TestH5ContactConfigKeys(t *testing.T) {
	assert.Equal(t, "notice.public_detail.enabled", CfgKeyNoticePublicDetailEnabled)
	assert.Equal(t, "contact.qq_group", CfgKeyContactQQGroup)
	assert.Equal(t, "contact.wechat", CfgKeyContactWechat)
	assert.Equal(t, "contact.email", CfgKeyContactEmail)
	assert.Equal(t, "contact.phone", CfgKeyContactPhone)
}

// ============== 6. ConfigCache 读取校验 ==============

// TestContactConfigCache_Read ConfigCache 应能正确读取 contact.* 配置
func TestContactConfigCache_Read(t *testing.T) {
	deps := setupH5EndUserDeps(t, map[string]string{
		CfgKeyContactQQGroup: "111111",
		CfgKeyContactEmail:   "test@test.com",
	})

	ctx := context.Background()
	assert.Equal(t, "111111", deps.CfgCache.GetString(ctx, CfgKeyContactQQGroup, ""))
	assert.Equal(t, "", deps.CfgCache.GetString(ctx, CfgKeyContactWechat, ""))
	assert.Equal(t, "test@test.com", deps.CfgCache.GetString(ctx, CfgKeyContactEmail, ""))
	assert.Equal(t, "", deps.CfgCache.GetString(ctx, CfgKeyContactPhone, ""))
}
