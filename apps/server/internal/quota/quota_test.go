// Package quota 套餐配额检查单元测试
// 用 SQLite 内存库模拟 gorm.DB，避免依赖 MySQL
// 严格遵循铁律 06：所有断言基于已知设置的状态
package quota

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// setupTestDB 用 SQLite 内存库初始化测试 DB + 自动建表
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "SQLite 内存库初始化失败")

	// 自动建表（仅建需要的表）
	require.NoError(t, db.AutoMigrate(
		&model.SysTenant{},
		&model.SysPackage{},
		&model.App{},
		&model.AppCard{},
		&model.Agent{},
		&model.AppDevice{},
	))

	// 每个测试清空表（避免 cache=shared 共享数据）
	require.NoError(t, db.Exec("DELETE FROM sys_tenant").Error)
	require.NoError(t, db.Exec("DELETE FROM sys_package").Error)
	require.NoError(t, db.Exec("DELETE FROM app").Error)
	require.NoError(t, db.Exec("DELETE FROM app_card").Error)
	require.NoError(t, db.Exec("DELETE FROM agent").Error)
	require.NoError(t, db.Exec("DELETE FROM app_device").Error)

	return db
}

// seedTenantPkg 种子开发者 + 套餐
// 注意：SysPackage 的 MaxApps/MaxCards/MaxAgents 有 gorm default 值，
// 0 会被 gorm 自动替换为 default 值，需在 Create 后用 Update 强制设为 0
func seedTenantPkg(t *testing.T, db *gorm.DB, tenantID uint64, tenantStatus string, maxApps, maxCards, maxAgents int) {
	t.Helper()
	pkg := model.SysPackage{
		BaseModel: model.BaseModel{ID: tenantID * 10}, // 避免与 tenantID 冲突
		Name:      "test-pkg",
		MaxApps:   99, // 占位，后面 Update 强制设置
		MaxCards:  99,
		MaxAgents: 99,
		Status:    "active",
	}
	require.NoError(t, db.Create(&pkg).Error)
	// 强制覆盖 default 值（处理 0 = 不限场景）
	require.NoError(t, db.Model(&pkg).Updates(map[string]interface{}{
		"max_apps":   maxApps,
		"max_cards":  maxCards,
		"max_agents": maxAgents,
	}).Error)

	tenant := model.SysTenant{
		BaseModel: model.BaseModel{ID: tenantID},
		Username:  "test-tenant",
		Status:    tenantStatus,
		PackageID: pkg.ID,
	}
	require.NoError(t, db.Create(&tenant).Error)
}

// ============== ExceededError ==============

func TestExceededError_Error(t *testing.T) {
	t.Run("with addCount (cards)", func(t *testing.T) {
		e := &ExceededError{Resource: "卡密数", Current: 95, Limit: 100, AddCount: 10}
		s := e.Error()
		assert.Contains(t, s, "卡密数")
		assert.Contains(t, s, "95")
		assert.Contains(t, s, "100")
		assert.Contains(t, s, "10")
	})
	t.Run("without addCount (apps/agents)", func(t *testing.T) {
		e := &ExceededError{Resource: "应用数", Current: 5, Limit: 5}
		s := e.Error()
		assert.Contains(t, s, "应用数")
		assert.Contains(t, s, "5")
		// 不含 AddCount 字样（默认值 0 时不显示）
		assert.NotContains(t, s, "本次新增")
	})
}

func TestExceededError_Is(t *testing.T) {
	err := &ExceededError{Resource: "应用数"}
	// errors.Is 应能识别
	assert.True(t, errors.Is(err, &ExceededError{}))
	// 其他错误不应匹配
	assert.False(t, errors.Is(errors.New("other"), &ExceededError{}))
}

// ============== CheckMaxApps ==============

func TestCheckMaxApps_BelowLimit(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 5, 1000, 0)
	// 当前 0 个应用，上限 5，应通过
	err := CheckMaxApps(db, 1)
	require.NoError(t, err)
}

func TestCheckMaxApps_AtLimit(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 2, 1000, 0)
	// 预置 2 个应用（已达上限 2）
	for i := 1; i <= 2; i++ {
		require.NoError(t, db.Create(&model.App{
			BaseModel: model.BaseModel{ID: uint64(i)},
			TenantID:  1,
			Name:      "app",
			AppKey:    "ak_test_" + strconv.Itoa(i),
		}).Error)
	}
	err := CheckMaxApps(db, 1)
	require.Error(t, err)
	var exErr *ExceededError
	require.True(t, errors.As(err, &exErr))
	assert.Equal(t, "应用数", exErr.Resource)
	assert.Equal(t, 2, exErr.Current)
	assert.Equal(t, 2, exErr.Limit)
}

func TestCheckMaxApps_Unlimited(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 0, 0, 0) // 0 表示不限
	// 预置 100 个应用，仍应通过（0 = 不限）
	for i := 1; i <= 100; i++ {
		require.NoError(t, db.Create(&model.App{
			BaseModel: model.BaseModel{ID: uint64(i)},
			TenantID:  1,
			Name:      "app",
			AppKey:    "ak_test_" + strconv.Itoa(i),
		}).Error)
	}
	err := CheckMaxApps(db, 1)
	require.NoError(t, err)
}

func TestCheckMaxApps_TenantNotFound(t *testing.T) {
	db := setupTestDB(t)
	// 不种子任何数据
	err := CheckMaxApps(db, 999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "查询开发者失败")
}

func TestCheckMaxApps_TenantDisabled(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "suspended", 5, 1000, 0)
	err := CheckMaxApps(db, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已被禁用")
}

func TestCheckMaxApps_TenantExpired(t *testing.T) {
	db := setupTestDB(t)
	past := time.Now().Add(-24 * time.Hour)
	pkg := model.SysPackage{BaseModel: model.BaseModel{ID: 10}, Name: "p", MaxApps: 5, Status: "active"}
	require.NoError(t, db.Create(&pkg).Error)
	tenant := model.SysTenant{
		BaseModel: model.BaseModel{ID: 1},
		Username:  "x",
		Status:    "active",
		PackageID: 10,
		ExpiresAt: &past,
	}
	require.NoError(t, db.Create(&tenant).Error)

	err := CheckMaxApps(db, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已过期")
}

func TestCheckMaxApps_PackageDisabled(t *testing.T) {
	db := setupTestDB(t)
	pkg := model.SysPackage{BaseModel: model.BaseModel{ID: 10}, Name: "p", MaxApps: 5, Status: "disabled"}
	require.NoError(t, db.Create(&pkg).Error)
	tenant := model.SysTenant{BaseModel: model.BaseModel{ID: 1}, Username: "x", Status: "active", PackageID: 10}
	require.NoError(t, db.Create(&tenant).Error)

	err := CheckMaxApps(db, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "套餐已被禁用")
}

// ============== CheckMaxCards ==============

func TestCheckMaxCards_BelowLimit(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 5, 1000, 0)
	// 当前 100 张卡，新增 50 张，1000 上限，应通过
	for i := 1; i <= 100; i++ {
		require.NoError(t, db.Create(&model.AppCard{
			BaseModel:  model.BaseModel{ID: uint64(i)},
			TenantID:   1,
			CardKey:    "card-" + strconv.Itoa(i),
			CardKeyHash: "hash-" + strconv.Itoa(i),
			Checksum:   "cs" + strconv.Itoa(i),
		}).Error)
	}
	err := CheckMaxCards(db, 1, 50)
	require.NoError(t, err)
}

func TestCheckMaxCards_ExceedLimit(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 5, 100, 0)
	// 当前 95 张，新增 10 张 → 105 > 100，超限
	for i := 1; i <= 95; i++ {
		require.NoError(t, db.Create(&model.AppCard{
			BaseModel:  model.BaseModel{ID: uint64(i)},
			TenantID:   1,
			CardKey:    "card-" + strconv.Itoa(i),
			CardKeyHash: "hash-" + strconv.Itoa(i),
			Checksum:   "cs" + strconv.Itoa(i),
		}).Error)
	}
	err := CheckMaxCards(db, 1, 10)
	require.Error(t, err)
	var exErr *ExceededError
	require.True(t, errors.As(err, &exErr))
	assert.Equal(t, "卡密数", exErr.Resource)
	assert.Equal(t, 95, exErr.Current)
	assert.Equal(t, 100, exErr.Limit)
	assert.Equal(t, 10, exErr.AddCount)
}

func TestCheckMaxCards_Unlimited(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 0, 0, 0) // 不限
	err := CheckMaxCards(db, 1, 10000)
	require.NoError(t, err)
}

// ============== CheckMaxAgents ==============

func TestCheckMaxAgents_BelowLimit(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 5, 1000, 10)
	// 当前 5 个代理，上限 10，应通过
	for i := 1; i <= 5; i++ {
		require.NoError(t, db.Create(&model.Agent{
			BaseModel: model.BaseModel{ID: uint64(i)},
			TenantID:  1,
			Username:  "agent-" + strconv.Itoa(i),
		}).Error)
	}
	err := CheckMaxAgents(db, 1)
	require.NoError(t, err)
}

func TestCheckMaxAgents_AtLimit(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 5, 1000, 3)
	// 当前 3 个代理 = 上限 3
	for i := 1; i <= 3; i++ {
		require.NoError(t, db.Create(&model.Agent{
			BaseModel: model.BaseModel{ID: uint64(i)},
			TenantID:  1,
			Username:  "agent-" + strconv.Itoa(i),
		}).Error)
	}
	err := CheckMaxAgents(db, 1)
	require.Error(t, err)
	var exErr *ExceededError
	require.True(t, errors.As(err, &exErr))
	assert.Equal(t, "代理数", exErr.Resource)
	assert.Equal(t, 3, exErr.Current)
	assert.Equal(t, 3, exErr.Limit)
}

func TestCheckMaxAgents_NotAllowed(t *testing.T) {
	db := setupTestDB(t)
	seedTenantPkg(t, db, 1, "active", 5, 1000, 0) // MaxAgents=0 表示不允许
	err := CheckMaxAgents(db, 1)
	require.Error(t, err)
	var exErr *ExceededError
	require.True(t, errors.As(err, &exErr))
	assert.Equal(t, "代理数", exErr.Resource)
	assert.Equal(t, 0, exErr.Limit, "MaxAgents=0 时应返回 Limit=0 而非不限")
}

// ============== CheckMaxDevices ==============

func TestCheckMaxDevices_BelowLimit(t *testing.T) {
	db := setupTestDB(t)
	// 预置 2 个活跃设备 = 上限 3
	for i := 1; i <= 2; i++ {
		require.NoError(t, db.Create(&model.AppDevice{
			BaseModel: model.BaseModel{ID: uint64(i)},
			CardID:    100,
			Status:    "active",
		}).Error)
	}
	err := CheckMaxDevices(db, 100, 3)
	require.NoError(t, err)
}

func TestCheckMaxDevices_AtLimit(t *testing.T) {
	db := setupTestDB(t)
	// 预置 3 个活跃设备 = 上限 3
	for i := 1; i <= 3; i++ {
		require.NoError(t, db.Create(&model.AppDevice{
			BaseModel: model.BaseModel{ID: uint64(i)},
			CardID:    100,
			Status:    "active",
		}).Error)
	}
	err := CheckMaxDevices(db, 100, 3)
	require.Error(t, err)
	var exErr *ExceededError
	require.True(t, errors.As(err, &exErr))
	assert.Equal(t, "设备数", exErr.Resource)
	assert.Equal(t, 3, exErr.Current)
	assert.Equal(t, 3, exErr.Limit)
}

func TestCheckMaxDevices_IgnoresInactive(t *testing.T) {
	db := setupTestDB(t)
	// 5 个 banned + 2 个 active，上限 3 应通过（仅 active 计数）
	for i := 1; i <= 5; i++ {
		require.NoError(t, db.Create(&model.AppDevice{
			BaseModel: model.BaseModel{ID: uint64(i)},
			CardID:    100,
			Status:    "banned",
		}).Error)
	}
	for i := 6; i <= 7; i++ {
		require.NoError(t, db.Create(&model.AppDevice{
			BaseModel: model.BaseModel{ID: uint64(i)},
			CardID:    100,
			Status:    "active",
		}).Error)
	}
	err := CheckMaxDevices(db, 100, 3)
	require.NoError(t, err, "应仅统计 active 设备")
}

func TestCheckMaxDevices_Unlimited(t *testing.T) {
	db := setupTestDB(t)
	// maxDevices=0 表示不限
	err := CheckMaxDevices(db, 100, 0)
	require.NoError(t, err)
}

func TestCheckMaxDevices_DifferentCard(t *testing.T) {
	db := setupTestDB(t)
	// card 1 已有 5 个 active 设备
	for i := 1; i <= 5; i++ {
		require.NoError(t, db.Create(&model.AppDevice{
			BaseModel: model.BaseModel{ID: uint64(i)},
			CardID:    1,
			Status:    "active",
		}).Error)
	}
	// 校验 card 2 应通过（不同卡密不互相影响）
	err := CheckMaxDevices(db, 2, 3)
	require.NoError(t, err)
}
