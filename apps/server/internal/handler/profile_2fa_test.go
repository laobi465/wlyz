// Package handler 2FA 备用码 DB 持久化单元测试
// v0.4.0：覆盖 loadUserBackupCodes / updateUserBackupCodes / consumeBackupCode 核心路径
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package handler

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/auth"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// setup2FATestDB 启动 SQLite 内存库 + AutoMigrate 三表（sys_admin/sys_tenant/agent）
func setup2FATestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SysAdmin{}, &model.SysTenant{}, &model.Agent{}))
	// 清空表（cache=shared 模式下可能残留旧数据）
	db.Exec("DELETE FROM sys_admin")
	db.Exec("DELETE FROM sys_tenant")
	db.Exec("DELETE FROM agent")
	return db
}

// setup2FACrypto 启动 AES-256 crypto manager
func setup2FACrypto(t *testing.T) *crypto.Manager {
	t.Helper()
	mgr, err := crypto.NewManager("0123456789abcdef0123456789abcdef", "", "")
	require.NoError(t, err)
	return mgr
}

// setup2FAMiniRedis 启动 miniredis + 返回 redis.Client
func setup2FAMiniRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

// setup2FADeps 构造测试 Deps
func setup2FADeps(t *testing.T) *Deps {
	t.Helper()
	rdb, _ := setup2FAMiniRedis(t)
	return &Deps{
		DB:     setup2FATestDB(t),
		Redis:  rdb,
		Crypto: setup2FACrypto(t),
	}
}

// seedAdminWithBackupCodes 创建 admin 用户 + 设置 backup_codes 字段（AES 加密）
func seedAdminWithBackupCodes(t *testing.T, deps *Deps, codes []string) uint64 {
	t.Helper()
	admin := model.SysAdmin{
		BaseModel:    model.BaseModel{ID: 1},
		Username:     "admin-test",
		PasswordHash: "$2a$12$dummyhashplaceholderdummyhashplaceholderdummyhash",
		Status:       "active",
	}
	require.NoError(t, deps.DB.Create(&admin).Error)
	if len(codes) > 0 {
		enc, err := deps.Crypto.EncryptAES(joinStrings(codes, ","))
		require.NoError(t, err)
		require.NoError(t, updateUserBackupCodes(deps, auth.RoleAdmin, 1, enc))
	}
	return 1
}

// joinStrings 简易 strings.Join 包装（避免引入 strings 包）
func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// ============== loadUserBackupCodes ==============

// TestLoadUserBackupCodes_DB读取 验证从 DB 字段读取 AES 加密的备用码
func TestLoadUserBackupCodes_DB读取(t *testing.T) {
	deps := setup2FADeps(t)
	codes := []string{"abc12345", "def67890", "ghi13579"}
	uid := seedAdminWithBackupCodes(t, deps, codes)

	enc, err := loadUserBackupCodes(deps, auth.RoleAdmin, uid)
	require.NoError(t, err)
	assert.NotEmpty(t, enc, "应读取到加密的备用码")

	plain, err := deps.Crypto.DecryptAES(enc)
	require.NoError(t, err)
	assert.Equal(t, "abc12345,def67890,ghi13579", plain)
}

// TestLoadUserBackupCodes_DB为空回退Redis 验证 v0.3.x 老用户兼容路径
func TestLoadUserBackupCodes_DB为空回退Redis(t *testing.T) {
	deps := setup2FADeps(t)
	uid := seedAdminWithBackupCodes(t, deps, nil) // DB 字段为空

	// 在 Redis 写入 v0.3.x 老数据
	oldEnc, err := deps.Crypto.EncryptAES("legacy123,legacy456")
	require.NoError(t, err)
	require.NoError(t, deps.Redis.Set(context.Background(),
		twoFABackupKey(auth.RoleAdmin, uid), oldEnc, 0).Err())

	enc, err := loadUserBackupCodes(deps, auth.RoleAdmin, uid)
	require.NoError(t, err)
	assert.Equal(t, oldEnc, enc, "DB 为空时应回退到 Redis 老数据")
}

// TestLoadUserBackupCodes_用户不存在 验证 gorm.ErrRecordNotFound 透传
func TestLoadUserBackupCodes_用户不存在(t *testing.T) {
	deps := setup2FADeps(t)
	_, err := loadUserBackupCodes(deps, auth.RoleAdmin, 9999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record not found")
}

// TestLoadUserBackupCodes_TenantRole 验证 tenant 角色读取
func TestLoadUserBackupCodes_TenantRole(t *testing.T) {
	deps := setup2FADeps(t)
	tenant := model.SysTenant{
		BaseModel:    model.BaseModel{ID: 100},
		TenantCode:   "T001",
		Username:     "tenant-test",
		PasswordHash: "hash",
		Status:       "active",
	}
	require.NoError(t, deps.DB.Create(&tenant).Error)

	enc, err := deps.Crypto.EncryptAES("tcode1,tcode2")
	require.NoError(t, err)
	require.NoError(t, updateUserBackupCodes(deps, auth.RoleTenant, 100, enc))

	got, err := loadUserBackupCodes(deps, auth.RoleTenant, 100)
	require.NoError(t, err)
	assert.Equal(t, enc, got)
}

// TestLoadUserBackupCodes_AgentRole 验证 agent 角色读取
func TestLoadUserBackupCodes_AgentRole(t *testing.T) {
	deps := setup2FADeps(t)
	agent := model.Agent{
		BaseModel:    model.BaseModel{ID: 200},
		TenantID:     100,
		Username:     "agent-test",
		PasswordHash: "hash",
		Status:       "active",
	}
	require.NoError(t, deps.DB.Create(&agent).Error)

	enc, err := deps.Crypto.EncryptAES("acode1")
	require.NoError(t, err)
	require.NoError(t, updateUserBackupCodes(deps, auth.RoleAgent, 200, enc))

	got, err := loadUserBackupCodes(deps, auth.RoleAgent, 200)
	require.NoError(t, err)
	assert.Equal(t, enc, got)
}

// TestLoadUserBackupCodes_不支持角色 验证未知 role 返回错误
func TestLoadUserBackupCodes_不支持角色(t *testing.T) {
	deps := setup2FADeps(t)
	_, err := loadUserBackupCodes(deps, "unknown_role", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported role")
}

// ============== updateUserBackupCodes ==============

// TestUpdateUserBackupCodes_清空 验证传入空字符串清空字段
func TestUpdateUserBackupCodes_清空(t *testing.T) {
	deps := setup2FADeps(t)
	uid := seedAdminWithBackupCodes(t, deps, []string{"abc", "def"})

	require.NoError(t, updateUserBackupCodes(deps, auth.RoleAdmin, uid, ""))

	enc, err := loadUserBackupCodes(deps, auth.RoleAdmin, uid)
	require.NoError(t, err)
	assert.Empty(t, enc, "清空后应返回空字符串")
}

// ============== consumeBackupCode ==============

// TestConsumeBackupCode_消费成功 验证正确输入后备用码被移除 + DB 更新 + Redis 清理
func TestConsumeBackupCode_消费成功(t *testing.T) {
	deps := setup2FADeps(t)
	codes := []string{"abc12345", "def67890", "ghi13579"}
	uid := seedAdminWithBackupCodes(t, deps, codes)

	// 同时在 Redis 写老数据，验证消费后会被清理
	oldEnc, _ := deps.Crypto.EncryptAES("legacy-old-data")
	deps.Redis.Set(context.Background(), twoFABackupKey(auth.RoleAdmin, uid), oldEnc, 0)

	matched, remaining, err := consumeBackupCode(deps, auth.RoleAdmin, uid, "def67890")
	require.NoError(t, err)
	assert.True(t, matched, "应消费成功")
	assert.Equal(t, []string{"abc12345", "ghi13579"}, remaining, "remaining 应排除已消费的码")

	// 验证 DB 已更新（剩 2 个码）
	enc, err := loadUserBackupCodes(deps, auth.RoleAdmin, uid)
	require.NoError(t, err)
	plain, err := deps.Crypto.DecryptAES(enc)
	require.NoError(t, err)
	assert.Equal(t, "abc12345,ghi13579", plain)

	// 验证 Redis 老数据已被清理
	exists, err := deps.Redis.Exists(context.Background(), twoFABackupKey(auth.RoleAdmin, uid)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "Redis 老数据应被清理")
}

// TestConsumeBackupCode_消费最后一个 验证剩余空时 DB 写入空字符串
func TestConsumeBackupCode_消费最后一个(t *testing.T) {
	deps := setup2FADeps(t)
	uid := seedAdminWithBackupCodes(t, deps, []string{"only-code"})

	matched, remaining, err := consumeBackupCode(deps, auth.RoleAdmin, uid, "only-code")
	require.NoError(t, err)
	assert.True(t, matched)
	assert.Empty(t, remaining, "剩余应为空切片")

	// DB 字段应为空字符串
	enc, err := loadUserBackupCodes(deps, auth.RoleAdmin, uid)
	require.NoError(t, err)
	assert.Empty(t, enc)
}

// TestConsumeBackupCode_输入不匹配 验证错误输入不消费
func TestConsumeBackupCode_输入不匹配(t *testing.T) {
	deps := setup2FADeps(t)
	codes := []string{"abc12345", "def67890"}
	uid := seedAdminWithBackupCodes(t, deps, codes)

	matched, remaining, err := consumeBackupCode(deps, auth.RoleAdmin, uid, "wrong-code")
	require.NoError(t, err)
	assert.False(t, matched, "不匹配的输入应返回 false")
	assert.Equal(t, codes, remaining, "remaining 应保持原列表不变")

	// DB 不应被修改
	enc, err := loadUserBackupCodes(deps, auth.RoleAdmin, uid)
	require.NoError(t, err)
	plain, _ := deps.Crypto.DecryptAES(enc)
	assert.Equal(t, "abc12345,def67890", plain)
}

// TestConsumeBackupCode_空输入 验证空 input 直接返回 false 不查 DB
func TestConsumeBackupCode_空输入(t *testing.T) {
	deps := setup2FADeps(t)
	matched, remaining, err := consumeBackupCode(deps, auth.RoleAdmin, 1, "")
	require.NoError(t, err)
	assert.False(t, matched)
	assert.Nil(t, remaining)
}

// TestConsumeBackupCode_无备用码 验证 DB 和 Redis 都为空时返回 false 不报错
func TestConsumeBackupCode_无备用码(t *testing.T) {
	deps := setup2FADeps(t)
	uid := seedAdminWithBackupCodes(t, deps, nil)

	matched, remaining, err := consumeBackupCode(deps, auth.RoleAdmin, uid, "any-code")
	require.NoError(t, err)
	assert.False(t, matched)
	assert.Nil(t, remaining)
}

// TestConsumeBackupCode_从Redis回退消费 验证 v0.3.x 老用户首次消费走 Redis 路径
func TestConsumeBackupCode_从Redis回退消费(t *testing.T) {
	deps := setup2FADeps(t)
	uid := seedAdminWithBackupCodes(t, deps, nil) // DB 为空

	// 模拟 v0.3.x 老数据：只写 Redis
	oldEnc, err := deps.Crypto.EncryptAES("legacy1,legacy2,legacy3")
	require.NoError(t, err)
	deps.Redis.Set(context.Background(), twoFABackupKey(auth.RoleAdmin, uid), oldEnc, 0)

	matched, remaining, err := consumeBackupCode(deps, auth.RoleAdmin, uid, "legacy2")
	require.NoError(t, err)
	assert.True(t, matched)
	assert.Equal(t, []string{"legacy1", "legacy3"}, remaining)

	// 消费后 DB 应已写入新值（legacy1,legacy3）
	enc, err := loadUserBackupCodes(deps, auth.RoleAdmin, uid)
	require.NoError(t, err)
	plain, _ := deps.Crypto.DecryptAES(enc)
	assert.Equal(t, "legacy1,legacy3", plain, "消费后剩余备用码应回写到 DB")

	// Redis 老数据应被清理
	exists, _ := deps.Redis.Exists(context.Background(), twoFABackupKey(auth.RoleAdmin, uid)).Result()
	assert.Equal(t, int64(0), exists)
}

// TestTwoFABackupKey_格式 验证 key 格式与文档一致
func TestTwoFABackupKey_格式(t *testing.T) {
	assert.Equal(t, "2fa:backup:admin:1", twoFABackupKey("admin", 1))
	assert.Equal(t, "2fa:backup:tenant:100", twoFABackupKey("tenant", 100))
	assert.Equal(t, "2fa:backup:agent:200", twoFABackupKey("agent", 200))
}

// TestTwoFASetupKey_格式 验证 setup key 格式
func TestTwoFASetupKey_格式(t *testing.T) {
	assert.Equal(t, "2fa:setup:admin:1", twoFASetupKey("admin", 1))
}

// 防止未使用导入警告（time 在未来扩展时使用）
var _ = time.Now
