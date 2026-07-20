// Package handler 代理子域名 + 门户 + 二维码单元测试
// v0.4.x 残留项 1/2/3：覆盖 AgentSubdomainStatus/Apply/Unbind + AdminList/Approve/Reject
//                    + PublicPortal + AgentPortalQrCode 核心路径
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试基础设施：SQLite 内存库 + miniredis + 真实 ConfigCache
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 测试基础设施 ==============

// setupSubdomainTestDB 启动 SQLite 内存库 + AutoMigrate 子域名/门户/二维码相关表
func setupSubdomainTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:agent_subdomain_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Agent{},
		&model.SysTenant{},
		&model.SysConfig{},
		&model.App{},
		&model.AppCardType{},
		&model.SysPackage{},
	))
	// 清空表（cache=shared 模式下可能残留旧数据）
	db.Exec("DELETE FROM agent")
	db.Exec("DELETE FROM sys_tenant")
	db.Exec("DELETE FROM sys_config")
	db.Exec("DELETE FROM app")
	db.Exec("DELETE FROM app_card_type")
	db.Exec("DELETE FROM sys_package")
	return db
}

// setupSubdomainCrypto 启动 AES-256 crypto manager
func setupSubdomainCrypto(t *testing.T) *crypto.Manager {
	t.Helper()
	mgr, err := crypto.NewManager("0123456789abcdef0123456789abcdef", "", "")
	require.NoError(t, err)
	return mgr
}

// setupSubdomainMiniRedis 启动 miniredis + 返回 redis.Client
func setupSubdomainMiniRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

// setupSubdomainDeps 构造测试 Deps（含真实 ConfigCache，便于测试 sys_config 读取）
func setupSubdomainDeps(t *testing.T, cfgOverrides map[string]string) *Deps {
	t.Helper()
	rdb, _ := setupSubdomainMiniRedis(t)
	db := setupSubdomainTestDB(t)

	// 写入默认 sys_config + overrides
	defaults := map[string]string{
		"agent.subdomain.enabled": "0",
		"agent.subdomain.pattern": "^[a-z0-9-]{3,32}$",
	}
	for k, v := range cfgOverrides {
		defaults[k] = v
	}
	for k, v := range defaults {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "agent",
		}).Error)
	}

	cfgCache := config.NewConfigCache(db, rdb)
	require.NoError(t, cfgCache.Preload(context.Background()))
	return &Deps{
		DB:       db,
		Redis:    rdb,
		Crypto:   setupSubdomainCrypto(t),
		CfgCache: cfgCache,
	}
}

// ============== 测试种子数据 ==============

func seedSubdomainTenant(t *testing.T, db *gorm.DB, id uint64, username string) {
	t.Helper()
	require.NoError(t, db.Create(&model.SysTenant{
		BaseModel:  model.BaseModel{ID: id},
		TenantCode: "tc-" + strconv.FormatUint(id, 10),
		Username:   username,
		Status:     "active",
	}).Error)
}

func seedSubdomainAgent(t *testing.T, db *gorm.DB, id, tenantID uint64, username string, status string) {
	t.Helper()
	require.NoError(t, db.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: id},
		TenantID:        tenantID,
		Username:        username,
		PasswordHash:    "$2a$12$dummyhash",
		Status:          status,
		CommissionRate:  10.0,
		CommissionMode:  "percentage",
		SubdomainStatus: "none",
	}).Error)
}

func seedSubdomainApp(t *testing.T, db *gorm.DB, id, tenantID uint64, name string) {
	t.Helper()
	require.NoError(t, db.Create(&model.App{
		BaseModel: model.BaseModel{ID: id},
		TenantID:  tenantID,
		AppKey:    "app-key-" + strconv.FormatUint(id, 10),
		Name:      name,
		Status:    "active",
	}).Error)
}

func seedSubdomainCardType(t *testing.T, db *gorm.DB, id, appID, tenantID uint64, name string, price float64) {
	t.Helper()
	require.NoError(t, db.Create(&model.AppCardType{
		BaseModel: model.BaseModel{ID: id},
		TenantID:  tenantID,
		AppID:     appID,
		Name:      name,
		Type:      "duration",
		Price:     price,
		Status:    "active",
	}).Error)
}

// ============== 通用调用辅助 ==============

// callSubdomainGet 调用 GET 端点并返回响应
func callSubdomainGet(t *testing.T, path string, handler gin.HandlerFunc, ctxSetup func(c *gin.Context)) map[string]interface{} {
	t.Helper()
	return callSubdomainRequest(t, "GET", path, "", handler, ctxSetup)
}

// callSubdomainPost 调用 POST 端点并返回响应
func callSubdomainPost(t *testing.T, path, body string, handler gin.HandlerFunc, ctxSetup func(c *gin.Context)) map[string]interface{} {
	t.Helper()
	return callSubdomainRequest(t, "POST", path, body, handler, ctxSetup)
}

// callSubdomainDelete 调用 DELETE 端点并返回响应
func callSubdomainDelete(t *testing.T, path string, handler gin.HandlerFunc, ctxSetup func(c *gin.Context)) map[string]interface{} {
	t.Helper()
	return callSubdomainRequest(t, "DELETE", path, "", handler, ctxSetup)
}

// callSubdomainRequest 通用调用：返回响应 code/message/data（不强制 HTTP 200，由调用方断言）
func callSubdomainRequest(t *testing.T, method, path, body string, handler gin.HandlerFunc, ctxSetup func(c *gin.Context)) map[string]interface{} {
	t.Helper()
	g := setupGin()
	if ctxSetup != nil {
		g.Use(func(c *gin.Context) {
			ctxSetup(c)
			c.Next()
		})
	}
	g.Handle(method, path, handler)

	w := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	g.ServeHTTP(w, req)

	var resp struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "解析响应失败，HTTP %d, body=%s", w.Code, w.Body.String())
	return map[string]interface{}{
		"code":       resp.Code,
		"message":    resp.Message,
		"data":       resp.Data,
		"http_status": w.Code,
	}
}

// withAgentContext 注入 agent JWT claims（user_id=agentID, tenant_id=tenantID, role=agent）
func withAgentContext(agentID, tenantID uint64) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set("user_id", agentID)
		c.Set("tenant_id", tenantID)
		c.Set("role", "agent")
	}
}

// withAdminContext 注入 admin JWT claims
func withAdminContext() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set("user_id", uint64(1))
		c.Set("tenant_id", uint64(0))
		c.Set("role", "admin")
	}
}

// ============== 1. AgentSubdomainStatus 测试 ==============

// TestAgentSubdomainStatus_DefaultConfig 默认配置（功能关闭）应返回 enabled=false
func TestAgentSubdomainStatus_DefaultConfig(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "active")

	resp := callSubdomainGet(t, "/agent/subdomain",
		AgentSubdomainStatus(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 0, resp["code"], "应成功")
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, false, data["enabled"], "默认 enabled=false")
	assert.Equal(t, "^[a-z0-9-]{3,32}$", data["pattern"], "默认 pattern")
	assert.Equal(t, "none", data["subdomain_status"], "默认 subdomain_status=none")
}

// TestAgentSubdomainStatus_EnabledConfig 启用配置后应返回 enabled=true
func TestAgentSubdomainStatus_EnabledConfig(t *testing.T) {
	deps := setupSubdomainDeps(t, map[string]string{
		"agent.subdomain.enabled": "1",
	})
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "active")

	resp := callSubdomainGet(t, "/agent/subdomain",
		AgentSubdomainStatus(deps),
		withAgentContext(200, 100))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, true, data["enabled"], "启用后 enabled=true")
}

// TestAgentSubdomainStatus_AgentNotFound 不存在的代理应返回 1003
func TestAgentSubdomainStatus_AgentNotFound(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)

	resp := callSubdomainGet(t, "/agent/subdomain",
		AgentSubdomainStatus(deps),
		withAgentContext(999, 100))

	assert.NotEqual(t, 0, resp["code"], "应失败")
	assert.Equal(t, 1003, resp["code"], "应返回 1003")
}

// ============== 2. AgentApplySubdomain 测试 ==============

// TestAgentApplySubdomain_Disabled 功能关闭时应拒绝申请
func TestAgentApplySubdomain_Disabled(t *testing.T) {
	deps := setupSubdomainDeps(t, nil) // 默认 enabled=0
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "active")

	body := `{"subdomain":"agent1"}`
	resp := callSubdomainPost(t, "/agent/subdomain/apply",
		body, AgentApplySubdomain(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 1051, resp["code"], "功能未启用应返回 1051")
}

// TestAgentApplySubdomain_Success 启用后合法申请应成功
func TestAgentApplySubdomain_Success(t *testing.T) {
	deps := setupSubdomainDeps(t, map[string]string{
		"agent.subdomain.enabled": "1",
	})
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "active")

	body := `{"subdomain":"myagent"}`
	resp := callSubdomainPost(t, "/agent/subdomain/apply",
		body, AgentApplySubdomain(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 0, resp["code"], "应成功")
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "myagent", data["subdomain"])
	assert.Equal(t, "pending", data["subdomain_status"])

	// 校验 DB 状态
	var agent model.Agent
	require.NoError(t, deps.DB.First(&agent, 200).Error)
	assert.Equal(t, "pending", agent.SubdomainStatus)
	assert.Equal(t, "myagent", agent.Subdomain)
}

// TestAgentApplySubdomain_InvalidFormat 格式不合法应拒绝
func TestAgentApplySubdomain_InvalidFormat(t *testing.T) {
	deps := setupSubdomainDeps(t, map[string]string{
		"agent.subdomain.enabled": "1",
	})
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "active")

	// 含下划线不符合 ^[a-z0-9-]{3,32}$（注意：handler 会先转小写，所以不能用大写测试）
	body := `{"subdomain":"my_agent"}`
	resp := callSubdomainPost(t, "/agent/subdomain/apply",
		body, AgentApplySubdomain(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 1001, resp["code"], "格式不合法应返回 1001")
}

// TestAgentApplySubdomain_DuplicateApproved 已 approved 的子域名不可重复申请
func TestAgentApplySubdomain_DuplicateApproved(t *testing.T) {
	deps := setupSubdomainDeps(t, map[string]string{
		"agent.subdomain.enabled": "1",
	})
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	// agent A 已 approved 子域名 "shared"
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		Subdomain:       "shared",
		SubdomainStatus: "approved",
		CommissionRate:  10.0,
	}).Error)
	// agent B 尝试申请同名
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 201},
		TenantID:        100,
		Username:        "agentB",
		Status:          "active",
		SubdomainStatus: "none",
		CommissionRate:  10.0,
	}).Error)

	body := `{"subdomain":"shared"}`
	resp := callSubdomainPost(t, "/agent/subdomain/apply",
		body, AgentApplySubdomain(deps),
		withAgentContext(201, 100))

	assert.Equal(t, 1054, resp["code"], "重复子域名应返回 1054")
}

// TestAgentApplySubdomain_PendingRepeat pending 状态重复申请应拒绝
func TestAgentApplySubdomain_PendingRepeat(t *testing.T) {
	deps := setupSubdomainDeps(t, map[string]string{
		"agent.subdomain.enabled": "1",
	})
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		Subdomain:       "pending-name",
		SubdomainStatus: "pending",
		CommissionRate:  10.0,
	}).Error)

	body := `{"subdomain":"another-name"}`
	resp := callSubdomainPost(t, "/agent/subdomain/apply",
		body, AgentApplySubdomain(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 1053, resp["code"], "pending 状态重复申请应返回 1053")
}

// ============== 3. AgentUnbindSubdomain 测试 ==============

// TestAgentUnbindSubdomain_Success approved 状态可解绑
func TestAgentUnbindSubdomain_Success(t *testing.T) {
	deps := setupSubdomainDeps(t, map[string]string{
		"agent.subdomain.enabled": "1",
	})
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		Subdomain:       "myagent",
		SubdomainStatus: "approved",
		CommissionRate:  10.0,
	}).Error)

	resp := callSubdomainDelete(t, "/agent/subdomain",
		AgentUnbindSubdomain(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 0, resp["code"], "应成功")
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "none", data["subdomain_status"])

	// 校验 DB
	var agent model.Agent
	require.NoError(t, deps.DB.First(&agent, 200).Error)
	assert.Equal(t, "none", agent.SubdomainStatus)
	assert.Equal(t, "", agent.Subdomain)
}

// TestAgentUnbindSubdomain_NoneStatus none 状态不可解绑
func TestAgentUnbindSubdomain_NoneStatus(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agentA", "active")

	resp := callSubdomainDelete(t, "/agent/subdomain",
		AgentUnbindSubdomain(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 1001, resp["code"], "none 状态应返回 1001")
}

// ============== 4. AdminApproveSubdomain 测试 ==============

// TestAdminApproveSubdomain_Success pending → approved
func TestAdminApproveSubdomain_Success(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		Subdomain:       "myagent",
		SubdomainStatus: "pending",
		CommissionRate:  10.0,
	}).Error)

	// 直接构造请求（带路径参数 :id）
	g := setupGin()
	g.Use(func(c *gin.Context) {
		withAdminContext()(c)
		c.Next()
	})
	g.POST("/admin/agents/:id/subdomain/approve", AdminApproveSubdomain(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/agents/200/subdomain/approve", bytes.NewBufferString(`{"remark":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	g.ServeHTTP(w, req)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code, "应成功")
	assert.Equal(t, "approved", resp.Data["subdomain_status"])

	// 校验 DB
	var agent model.Agent
	require.NoError(t, deps.DB.First(&agent, 200).Error)
	assert.Equal(t, "approved", agent.SubdomainStatus)
}

// TestAdminApproveSubdomain_NotPending 非 pending 状态不可审批
func TestAdminApproveSubdomain_NotPending(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		SubdomainStatus: "none",
		CommissionRate:  10.0,
	}).Error)

	g := setupGin()
	g.Use(func(c *gin.Context) {
		withAdminContext()(c)
		c.Next()
	})
	g.POST("/admin/agents/:id/subdomain/approve", AdminApproveSubdomain(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/agents/200/subdomain/approve", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	g.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1001, resp.Code, "非 pending 状态应返回 1001")
}

// TestAdminApproveSubdomain_ConflictAutoReject 并发冲突时自动驳回
func TestAdminApproveSubdomain_ConflictAutoReject(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	// agent A 已 approved "shared"
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		Subdomain:       "shared",
		SubdomainStatus: "approved",
		CommissionRate:  10.0,
	}).Error)
	// agent B 申请同名，状态 pending（模拟并发审批前已通过其他代理）
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 201},
		TenantID:        100,
		Username:        "agentB",
		Status:          "active",
		Subdomain:       "shared",
		SubdomainStatus: "pending",
		CommissionRate:  10.0,
	}).Error)

	g := setupGin()
	g.Use(func(c *gin.Context) {
		withAdminContext()(c)
		c.Next()
	})
	g.POST("/admin/agents/:id/subdomain/approve", AdminApproveSubdomain(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/agents/201/subdomain/approve", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	g.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1054, resp.Code, "冲突应返回 1054")

	// 校验 agent B 被自动驳回
	var agent model.Agent
	require.NoError(t, deps.DB.First(&agent, 201).Error)
	assert.Equal(t, "rejected", agent.SubdomainStatus, "冲突时应自动驳回")
}

// ============== 5. AdminRejectSubdomain 测试 ==============

// TestAdminRejectSubdomain_Success pending → rejected
func TestAdminRejectSubdomain_Success(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		Subdomain:       "myagent",
		SubdomainStatus: "pending",
		CommissionRate:  10.0,
	}).Error)

	// 直接构造请求（带路径参数 :id）
	g := setupGin()
	g.Use(func(c *gin.Context) {
		withAdminContext()(c)
		c.Next()
	})
	g.POST("/admin/agents/:id/subdomain/reject", AdminRejectSubdomain(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/agents/200/subdomain/reject", bytes.NewBufferString(`{"remark":"name too generic"}`))
	req.Header.Set("Content-Type", "application/json")
	g.ServeHTTP(w, req)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code, "应成功")
	assert.Equal(t, "rejected", resp.Data["subdomain_status"])

	// 校验 DB
	var agent model.Agent
	require.NoError(t, deps.DB.First(&agent, 200).Error)
	assert.Equal(t, "rejected", agent.SubdomainStatus)
}

// ============== 6. AdminListSubdomains 测试 ==============

// TestAdminListSubdomains_FiltersByStatus 按状态筛选
func TestAdminListSubdomains_FiltersByStatus(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	// 创建 3 个代理：1 pending + 1 approved + 1 none（不应出现）
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentPending",
		Status:          "active",
		Subdomain:       "pending-name",
		SubdomainStatus: "pending",
		CommissionRate:  10.0,
	}).Error)
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 201},
		TenantID:        100,
		Username:        "agentApproved",
		Status:          "active",
		Subdomain:       "approved-name",
		SubdomainStatus: "approved",
		CommissionRate:  10.0,
	}).Error)
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 202},
		TenantID:        100,
		Username:        "agentNone",
		Status:          "active",
		SubdomainStatus: "none",
		CommissionRate:  10.0,
	}).Error)

	g := setupGin()
	g.Use(func(c *gin.Context) {
		withAdminContext()(c)
		c.Next()
	})
	g.GET("/admin/agents/subdomains", AdminListSubdomains(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/agents/subdomains?status=pending", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			List  []map[string]interface{} `json:"list"`
			Total float64                  `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, float64(1), resp.Data.Total, "应只返回 1 个 pending 代理")
	require.Len(t, resp.Data.List, 1)
	assert.Equal(t, "agentPending", resp.Data.List[0]["username"])
}

// ============== 7. PublicPortal 测试 ==============

// TestPublicPortal_Success 返回代理门户信息 + 卡类列表
func TestPublicPortal_Success(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "active")
	seedSubdomainApp(t, deps.DB, 300, 100, "MyApp")
	seedSubdomainCardType(t, deps.DB, 400, 300, 100, "月卡", 9.9)

	g := setupGin()
	g.GET("/public/portal/:agent_id", PublicPortal(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/portal/200", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Agent     map[string]interface{}   `json:"agent"`
			CardTypes []map[string]interface{} `json:"card_types"`
			Total     float64                  `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "agent1", resp.Data.Agent["username"])
	assert.Equal(t, float64(1), resp.Data.Total, "应返回 1 个卡类")
	require.Len(t, resp.Data.CardTypes, 1)
	assert.Equal(t, "月卡", resp.Data.CardTypes[0]["name"])
	assert.Equal(t, "MyApp", resp.Data.CardTypes[0]["app_name"])

	// 验证不返回敏感字段：agent_base_price 不应在 card_types 中
	_, hasBasePrice := resp.Data.CardTypes[0]["agent_base_price"]
	assert.False(t, hasBasePrice, "不应返回 agent_base_price 敏感字段")
}

// TestPublicPortal_AgentDisabled 代理被禁用应返回 1008
func TestPublicPortal_AgentDisabled(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "disabled")

	g := setupGin()
	g.GET("/public/portal/:agent_id", PublicPortal(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/portal/200", nil)
	g.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1008, resp.Code, "代理被禁用应返回 1008")
}

// TestPublicPortal_NotFound 代理不存在应返回 1008
func TestPublicPortal_NotFound(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)

	g := setupGin()
	g.GET("/public/portal/:agent_id", PublicPortal(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/portal/999", nil)
	g.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1008, resp.Code, "代理不存在应返回 1008")
}

// TestPublicPortal_TenantDisabled 开发者被禁用应返回 4002
func TestPublicPortal_TenantDisabled(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	// 创建被禁用的开发者
	require.NoError(t, deps.DB.Create(&model.SysTenant{
		BaseModel:  model.BaseModel{ID: 100},
		TenantCode: "tc-100",
		Username:   "tenant1",
		Status:     "disabled",
	}).Error)
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "active")

	g := setupGin()
	g.GET("/public/portal/:agent_id", PublicPortal(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/public/portal/200", nil)
	g.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 4002, resp.Code, "开发者被禁用应返回 4002")
}

// ============== 8. AgentPortalQrCode 测试 ==============

// TestAgentPortalQrCode_NoSubdomain 未绑定子域名时回退 agent_id 路径
func TestAgentPortalQrCode_NoSubdomain(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	seedSubdomainAgent(t, deps.DB, 200, 100, "agent1", "active")

	resp := callSubdomainGet(t, "/agent/portal/qrcode",
		AgentPortalQrCode(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 0, resp["code"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(200), data["agent_id"])
	assert.Equal(t, "none", data["subdomain_status"])

	portalURL, ok := data["portal_url"].(string)
	require.True(t, ok)
	assert.Contains(t, portalURL, "/h5/portal/200", "未配置 base_url 时应回退到相对路径")

	qrcodeAPI, ok := data["qrcode_api"].(string)
	require.True(t, ok)
	assert.Contains(t, qrcodeAPI, "api.qrserver.com")
	assert.Contains(t, qrcodeAPI, portalURL)
}

// TestAgentPortalQrCode_WithApprovedSubdomain 已 approved 子域名 + base_url 配置时应使用子域名 URL
func TestAgentPortalQrCode_WithApprovedSubdomain(t *testing.T) {
	deps := setupSubdomainDeps(t, map[string]string{
		"agent.portal.base_url": "https://keyauth.example.com",
	})
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		Subdomain:       "myagent",
		SubdomainStatus: "approved",
		CommissionRate:  10.0,
	}).Error)

	resp := callSubdomainGet(t, "/agent/portal/qrcode",
		AgentPortalQrCode(deps),
		withAgentContext(200, 100))

	assert.Equal(t, 0, resp["code"])
	data := resp["data"].(map[string]interface{})
	portalURL, ok := data["portal_url"].(string)
	require.True(t, ok)
	assert.Contains(t, portalURL, "myagent.keyauth.example.com", "应使用 subdomain.host 形式")
	assert.Contains(t, portalURL, "/h5/portal/200")
}

// TestAgentPortalQrCode_PendingSubdomain pending 状态时仍使用 agent_id 路径
func TestAgentPortalQrCode_PendingSubdomain(t *testing.T) {
	deps := setupSubdomainDeps(t, map[string]string{
		"agent.portal.base_url": "https://keyauth.example.com",
	})
	seedSubdomainTenant(t, deps.DB, 100, "tenant1")
	require.NoError(t, deps.DB.Create(&model.Agent{
		BaseModel:       model.BaseModel{ID: 200},
		TenantID:        100,
		Username:        "agentA",
		Status:          "active",
		Subdomain:       "myagent",
		SubdomainStatus: "pending",
		CommissionRate:  10.0,
	}).Error)

	resp := callSubdomainGet(t, "/agent/portal/qrcode",
		AgentPortalQrCode(deps),
		withAgentContext(200, 100))

	data := resp["data"].(map[string]interface{})
	portalURL := data["portal_url"].(string)
	assert.Contains(t, portalURL, "https://keyauth.example.com/h5/portal/200",
		"pending 状态应回退到 base_url + agent_id 路径")
	assert.NotContains(t, portalURL, "myagent.keyauth", "pending 状态不应使用 subdomain URL")
}

// TestAgentPortalQrCode_AgentNotFound 代理不存在应返回 1003
func TestAgentPortalQrCode_AgentNotFound(t *testing.T) {
	deps := setupSubdomainDeps(t, nil)

	resp := callSubdomainGet(t, "/agent/portal/qrcode",
		AgentPortalQrCode(deps),
		withAgentContext(999, 100))

	assert.Equal(t, 1003, resp["code"], "代理不存在应返回 1003")
}
