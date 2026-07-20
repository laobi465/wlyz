// Package backup v0.4.0 数据备份恢复核心逻辑单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试覆盖：
//   1. serializeValue（nil/string/[]byte/bool/time.Time/默认）
//   2. parseTablesFilter（空 / 单 / 多 / 含空格）
//   3. extractPayload（gzip 压缩 / 未压缩 / 缺少分隔符 / 非法 metadata JSON）
//   4. CreateBackup（无加密无压缩 / gzip 压缩 / AES 加密 / 表过滤）
//   5. RestoreBackup（无加密 / 加密 / checksum 不匹配 / 状态非 success）
//   6. CleanupExpired（按保留天数清理 / retention=0 不清理）
//   7. VerifyChecksum（成功 / 失败）
//   8. 状态机常量
package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 测试基础设施 ==============

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:backup_test_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SystemBackupLog{}, &model.SysConfig{},
		// 业务表用于测试备份
		&model.App{}, &model.AppCard{},
	))
	db.Exec("DELETE FROM system_backup_log")
	db.Exec("DELETE FROM sys_config")
	db.Exec("DELETE FROM app")
	db.Exec("DELETE FROM app_card")
	return db
}

func setupTestCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) (*config.ConfigCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	defaults := map[string]string{
		CfgKeyBackupDir:     "data/backups_test",
		CfgKeyRetentionDays: "30",
		CfgKeyAutoEnabled:   "0",
		CfgKeyEncryptionKey: "", // 默认不加密
		CfgKeyCompress:      "1",
		CfgKeyTablesFilter:  "",
	}
	if overrides == nil {
		overrides = map[string]string{}
	}
	for k, v := range defaults {
		if _, ok := overrides[k]; !ok {
			overrides[k] = v
		}
	}
	for k, v := range overrides {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "backup",
		}).Error)
	}
	return config.NewConfigCache(db, rdb), mr
}

// genValidAESKey 生成随机 32 字节 AES 密钥的 hex 编码字符串
func genValidAESKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return hex.EncodeToString(key)
}

// seedTestData 写入测试数据
func seedTestData(t *testing.T, db *gorm.DB) {
	t.Helper()
	app := &model.App{
		TenantID:         1,
		Name:             "Test App",
		AppKey:           "test-app-key-" + time.Now().Format("150405.000000"),
		AppSecret:        "test-app-secret",
		SignSecret:       "test-sign-secret",
		Status:           "active",
		MaxDevices:       1,
		HeartbeatInterval: 60,
		HeartbeatTimeout:  180,
		OfflineGrace:      86400,
	}
	require.NoError(t, db.Create(app).Error)
	card := &model.AppCard{
		TenantID:        1,
		AppID:           app.ID,
		CardKey:         "TEST-CARD-001",
		CardKeyHash:     "hash-" + time.Now().Format("150405.000000"),
		Checksum:        "abcd1234",
		Status:          "active",
		DurationSeconds: 2592000,
		MaxUses:         1,
	}
	require.NoError(t, db.Create(card).Error)
}

// cleanupBackupDir 清理测试备份目录
func cleanupBackupDir(t *testing.T, dir string) {
	t.Helper()
	abs, _ := filepath.Abs(dir)
	_ = os.RemoveAll(abs)
}

// ============== 1. serializeValue ==============

func TestSerializeValue_Nil(t *testing.T) {
	assert.Equal(t, "NULL", serializeValue(nil))
}

func TestSerializeValue_String(t *testing.T) {
	assert.Equal(t, "'hello'", serializeValue("hello"))
	// 单引号转义
	assert.Equal(t, "'it''s'", serializeValue("it's"))
}

func TestSerializeValue_Bytes(t *testing.T) {
	result := serializeValue([]byte{0x12, 0x34, 0xab})
	assert.Equal(t, "x'1234ab'", result)
}

func TestSerializeValue_Bool(t *testing.T) {
	assert.Equal(t, "1", serializeValue(true))
	assert.Equal(t, "0", serializeValue(false))
}

func TestSerializeValue_Time(t *testing.T) {
	ts := time.Date(2026, 7, 20, 15, 30, 45, 0, time.UTC)
	assert.Equal(t, "'2026-07-20 15:30:45'", serializeValue(ts))
}

func TestSerializeValue_Int(t *testing.T) {
	assert.Equal(t, "42", serializeValue(42))
	assert.Equal(t, "-100", serializeValue(-100))
}

// ============== 2. parseTablesFilter ==============

func TestParseTablesFilter_Empty(t *testing.T) {
	result := parseTablesFilter("")
	assert.Empty(t, result)
}

func TestParseTablesFilter_Single(t *testing.T) {
	result := parseTablesFilter("app")
	assert.True(t, result["app"])
	assert.Len(t, result, 1)
}

func TestParseTablesFilter_Multiple(t *testing.T) {
	result := parseTablesFilter("app,app_card,app_order")
	assert.True(t, result["app"])
	assert.True(t, result["app_card"])
	assert.True(t, result["app_order"])
	assert.Len(t, result, 3)
}

func TestParseTablesFilter_WithSpaces(t *testing.T) {
	result := parseTablesFilter(" app , app_card ,  app_order ")
	assert.True(t, result["app"])
	assert.True(t, result["app_card"])
	assert.True(t, result["app_order"])
}

func TestParseTablesFilter_OnlyCommas(t *testing.T) {
	result := parseTablesFilter(" , , , ")
	assert.Empty(t, result)
}

// ============== 3. extractPayload ==============

func TestExtractPayload_Uncompressed(t *testing.T) {
	metadata := BackupMetadata{
		Version:    1,
		TablesCount: 3,
		RowsCount:   10,
		Compressed:  false,
	}
	metadataBytes, _ := json.Marshal(metadata)
	payload := append(metadataBytes, '\n')
	payload = append(payload, []byte("INSERT INTO app VALUES (1);\n")...)

	m, mgr := setupManagerForTest(t)
	_ = m
	extractedMeta, sqlData, err := mgr.extractPayload(payload)
	require.NoError(t, err)
	assert.Equal(t, 1, extractedMeta.Version)
	assert.Equal(t, 3, extractedMeta.TablesCount)
	assert.Contains(t, string(sqlData), "INSERT INTO app")
}

func TestExtractPayload_GzipCompressed(t *testing.T) {
	metadata := BackupMetadata{
		Version:    1,
		TablesCount: 2,
		Compressed:  true,
	}
	metadataBytes, _ := json.Marshal(metadata)
	original := append(metadataBytes, '\n')
	original = append(original, []byte("INSERT INTO app VALUES (1);\n")...)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(original)
	_ = gw.Close()

	_, mgr := setupManagerForTest(t)
	extractedMeta, sqlData, err := mgr.extractPayload(buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, 2, extractedMeta.TablesCount)
	assert.Contains(t, string(sqlData), "INSERT INTO app")
}

func TestExtractPayload_MissingNewline(t *testing.T) {
	// 缺少 \n 分隔符
	payload := []byte(`{"version":1}`)
	_, mgr := setupManagerForTest(t)
	_, _, err := mgr.extractPayload(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "缺少 metadata 分隔符")
}

func TestExtractPayload_InvalidMetadataJSON(t *testing.T) {
	payload := []byte("not-json\nINSERT INTO app VALUES (1);")
	_, mgr := setupManagerForTest(t)
	_, _, err := mgr.extractPayload(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析 metadata JSON 失败")
}

// setupManagerForTest 创建临时 Manager 用于纯函数测试
func setupManagerForTest(t *testing.T) (*gorm.DB, *Manager) {
	t.Helper()
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	return db, &Manager{db: db, cache: cache}
}

// ============== 4. CreateBackup ==============

func TestCreateBackup_NoEncryptionNoCompression(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress:      "0",
		CfgKeyEncryptionKey: "",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	opts := BackupOptions{BackupType: BackupTypeManual, TriggerBy: 1, TriggerIP: "127.0.0.1"}
	result, err := mgr.CreateBackup(context.Background(), opts)

	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.NotEmpty(t, result.FilePath)
	assert.Greater(t, result.FileSize, int64(0))
	assert.NotEmpty(t, result.Checksum)
	assert.Equal(t, 64, len(result.Checksum)) // SHA-256 hex = 64 字符
	assert.Greater(t, result.TablesCount, 0)
	assert.Greater(t, result.RowsCount, int64(0))

	// 文件应存在
	_, err = os.Stat(result.FilePath)
	assert.NoError(t, err)

	// 审计日志应写入
	var log model.SystemBackupLog
	require.NoError(t, db.First(&log, result.LogID).Error)
	assert.Equal(t, StatusSuccess, log.Status)
	assert.Equal(t, result.Checksum, log.Checksum)
}

func TestCreateBackup_WithGzipCompression(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress:      "1",
		CfgKeyEncryptionKey: "",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	opts := BackupOptions{BackupType: BackupTypeManual, TriggerBy: 1}
	result, err := mgr.CreateBackup(context.Background(), opts)

	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)

	// 验证文件确实是 gzip 格式（前两字节 0x1f 0x8b）
	content, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	assert.Equal(t, byte(0x1f), content[0])
	assert.Equal(t, byte(0x8b), content[1])
}

func TestCreateBackup_WithAESEncryption(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	aesKey := genValidAESKey(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress:      "0",
		CfgKeyEncryptionKey: aesKey,
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	opts := BackupOptions{BackupType: BackupTypeManual, TriggerBy: 1}
	result, err := mgr.CreateBackup(context.Background(), opts)

	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)

	// 加密文件不应是 gzip magic（0x1f 0x8b）
	content, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	assert.NotEqual(t, byte(0x1f), content[0])

	// 用 AES 解密验证内容
	key, _ := hex.DecodeString(aesKey)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := content[:nonceSize], content[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	require.NoError(t, err, "AES-GCM 解密应成功")
	assert.Contains(t, string(plaintext), "INSERT INTO")
}

func TestCreateBackup_WithTablesFilter(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress:    "0",
		CfgKeyTablesFilter: "app", // 仅备份 app 表
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	opts := BackupOptions{BackupType: BackupTypeManual, TriggerBy: 1}
	result, err := mgr.CreateBackup(context.Background(), opts)

	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	// 仅 1 个表（app）
	assert.Equal(t, 1, result.TablesCount)
}

func TestCreateBackup_InvalidEncryptionKey(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyEncryptionKey: "invalid-hex", // 非 hex
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	opts := BackupOptions{BackupType: BackupTypeManual, TriggerBy: 1}
	result, err := mgr.CreateBackup(context.Background(), opts)

	assert.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.ErrorMessage, "加密密钥格式错误")
}

// ============== 5. RestoreBackup ==============

func TestRestoreBackup_NoEncryption(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress:      "0",
		CfgKeyEncryptionKey: "",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	// 先备份
	backupResult, err := mgr.CreateBackup(context.Background(), BackupOptions{
		BackupType: BackupTypeManual,
		TriggerBy:  1,
	})
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, backupResult.Status)

	// 清空数据
	require.NoError(t, db.Exec("DELETE FROM app_card").Error)
	require.NoError(t, db.Exec("DELETE FROM app").Error)
	var appCount int64
	db.Model(&model.App{}).Count(&appCount)
	assert.Equal(t, int64(0), appCount)

	// 恢复
	restoreResult, err := mgr.RestoreBackup(context.Background(), RestoreOptions{
		BackupLogID: backupResult.LogID,
		TriggerBy:   1,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, restoreResult.Status)
	assert.Greater(t, restoreResult.TablesCount, 0)
	assert.Greater(t, restoreResult.RowsCount, int64(0))

	// 验证数据恢复
	db.Model(&model.App{}).Count(&appCount)
	assert.Equal(t, int64(1), appCount, "app 表应恢复 1 条")
}

func TestRestoreBackup_WithAESEncryption(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	aesKey := genValidAESKey(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress:      "1", // 同时压缩 + 加密
		CfgKeyEncryptionKey: aesKey,
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	backupResult, err := mgr.CreateBackup(context.Background(), BackupOptions{
		BackupType: BackupTypeManual,
		TriggerBy:  1,
	})
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, backupResult.Status)

	// 清空 + 恢复
	require.NoError(t, db.Exec("DELETE FROM app_card").Error)
	require.NoError(t, db.Exec("DELETE FROM app").Error)

	restoreResult, err := mgr.RestoreBackup(context.Background(), RestoreOptions{
		BackupLogID: backupResult.LogID,
		TriggerBy:   1,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, restoreResult.Status)

	var appCount int64
	db.Model(&model.App{}).Count(&appCount)
	assert.Equal(t, int64(1), appCount)
}

func TestRestoreBackup_ChecksumMismatch(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress: "0",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	backupResult, err := mgr.CreateBackup(context.Background(), BackupOptions{
		BackupType: BackupTypeManual,
		TriggerBy:  1,
	})
	require.NoError(t, err)

	// 篡改备份文件
	require.NoError(t, os.WriteFile(backupResult.FilePath, []byte("tampered"), 0o600))

	// 恢复应失败（checksum 不匹配）
	restoreResult, err := mgr.RestoreBackup(context.Background(), RestoreOptions{
		BackupLogID: backupResult.LogID,
		TriggerBy:   1,
	})
	assert.Error(t, err)
	assert.Equal(t, StatusFailed, restoreResult.Status)
	assert.Contains(t, restoreResult.ErrorMessage, "checksum")
}

func TestRestoreBackup_SourceStatusNotSuccess(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)

	// 创建一个 failed 状态的备份日志
	log := &model.SystemBackupLog{
		BackupType: BackupTypeManual,
		Status:     StatusFailed,
	}
	require.NoError(t, db.Create(log).Error)

	mgr := NewManager(db, cache)
	_, err := mgr.RestoreBackup(context.Background(), RestoreOptions{
		BackupLogID: log.ID,
		TriggerBy:   1,
	})
	assert.Error(t, err)
}

func TestRestoreBackup_SourceNotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)

	mgr := NewManager(db, cache)
	_, err := mgr.RestoreBackup(context.Background(), RestoreOptions{
		BackupLogID: 9999,
		TriggerBy:   1,
	})
	assert.Error(t, err)
}

// ============== 6. CleanupExpired ==============

func TestCleanupExpired_DeletesOldBackups(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyRetentionDays: "7",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	// 创建 10 天前的备份日志
	oldTime := time.Now().AddDate(0, 0, -10)
	oldLog := &model.SystemBackupLog{
		BackupType: BackupTypeManual,
		Status:     StatusSuccess,
		FilePath:   "/tmp/nonexistent_old.bak",
		CreatedAt:  oldTime,
	}
	require.NoError(t, db.Create(oldLog).Error)

	// 创建 1 天前的备份日志（不应被删除）
	recentLog := &model.SystemBackupLog{
		BackupType: BackupTypeManual,
		Status:     StatusSuccess,
		FilePath:   "/tmp/nonexistent_recent.bak",
		CreatedAt:  time.Now().AddDate(0, 0, -1),
	}
	require.NoError(t, db.Create(recentLog).Error)

	mgr := NewManager(db, cache)
	deleted, err := mgr.CleanupExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, deleted, "应仅清理 1 个过期备份")

	// 验证日志状态更新
	var updated model.SystemBackupLog
	require.NoError(t, db.First(&updated, oldLog.ID).Error)
	assert.Equal(t, StatusDeleted, updated.Status)

	var recent model.SystemBackupLog
	require.NoError(t, db.First(&recent, recentLog.ID).Error)
	assert.Equal(t, StatusSuccess, recent.Status)
}

func TestCleanupExpired_ZeroRetentionNoOp(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyRetentionDays: "0", // 永不清理
	})

	// 创建 100 天前的备份
	oldLog := &model.SystemBackupLog{
		BackupType: BackupTypeManual,
		Status:     StatusSuccess,
		CreatedAt:  time.Now().AddDate(0, 0, -100),
	}
	require.NoError(t, db.Create(oldLog).Error)

	mgr := NewManager(db, cache)
	deleted, err := mgr.CleanupExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestCleanupExpired_OnlyCleansSuccessStatus(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyRetentionDays: "1",
	})

	// failed 状态的旧日志不应被清理
	failedLog := &model.SystemBackupLog{
		BackupType: BackupTypeManual,
		Status:     StatusFailed,
		CreatedAt:  time.Now().AddDate(0, 0, -10),
	}
	require.NoError(t, db.Create(failedLog).Error)

	mgr := NewManager(db, cache)
	deleted, err := mgr.CleanupExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, deleted, "failed 状态不应被清理")
}

// ============== 7. VerifyChecksum ==============

func TestVerifyChecksum_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress: "0",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	backupResult, err := mgr.CreateBackup(context.Background(), BackupOptions{
		BackupType: BackupTypeManual,
		TriggerBy:  1,
	})
	require.NoError(t, err)

	ok, err := mgr.VerifyChecksum(context.Background(), backupResult.LogID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerifyChecksum_FailedAfterTamper(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress: "0",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	backupResult, err := mgr.CreateBackup(context.Background(), BackupOptions{
		BackupType: BackupTypeManual,
		TriggerBy:  1,
	})
	require.NoError(t, err)

	// 篡改文件
	require.NoError(t, os.WriteFile(backupResult.FilePath, []byte("tampered"), 0o600))

	ok, err := mgr.VerifyChecksum(context.Background(), backupResult.LogID)
	require.NoError(t, err)
	assert.False(t, ok, "篡改后 checksum 应不匹配")
}

func TestVerifyChecksum_LogNotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	_, err := mgr.VerifyChecksum(context.Background(), 9999)
	assert.Error(t, err)
}

// ============== 8. 状态机常量 ==============

func TestBackupTypeConstants(t *testing.T) {
	types := map[string]string{
		"manual":         BackupTypeManual,
		"auto":           BackupTypeAuto,
		"restore_source": BackupTypeRestoreSource,
	}
	seen := map[string]bool{}
	for name, val := range types {
		assert.False(t, seen[val], "BackupType 常量 %s=%s 重复", name, val)
		seen[val] = true
	}
}

func TestBackupStatusConstants(t *testing.T) {
	statuses := []string{StatusPending, StatusRunning, StatusSuccess, StatusFailed, StatusDeleted}
	seen := map[string]bool{}
	for _, s := range statuses {
		assert.False(t, seen[s], "Status 常量重复: %s", s)
		seen[s] = true
	}
}

func TestBackupConfigKeyConstants(t *testing.T) {
	keys := []string{
		CfgKeyBackupDir, CfgKeyRetentionDays, CfgKeyAutoEnabled,
		CfgKeyEncryptionKey, CfgKeyCompress, CfgKeyTablesFilter,
	}
	seen := map[string]bool{}
	for _, k := range keys {
		assert.False(t, seen[k], "配置键常量重复: %s", k)
		seen[k] = true
	}
	for _, k := range keys {
		assert.True(t, strings.HasPrefix(k, "backup."), "配置键 %s 应以 backup. 开头", k)
	}
}

// ============== 9. 集成：备份+恢复 round-trip ==============

func TestBackupRestore_RoundTrip_WithCompressionAndEncryption(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	aesKey := genValidAESKey(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress:      "1",
		CfgKeyEncryptionKey: aesKey,
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)

	// 备份
	backupResult, err := mgr.CreateBackup(context.Background(), BackupOptions{
		BackupType: BackupTypeAuto,
		TriggerBy:  0,
	})
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, backupResult.Status)

	// 备份前数据快照
	var origAppCount, origCardCount int64
	db.Model(&model.App{}).Count(&origAppCount)
	db.Model(&model.AppCard{}).Count(&origCardCount)

	// 清空数据
	require.NoError(t, db.Exec("DELETE FROM app_card").Error)
	require.NoError(t, db.Exec("DELETE FROM app").Error)

	// 恢复
	restoreResult, err := mgr.RestoreBackup(context.Background(), RestoreOptions{
		BackupLogID: backupResult.LogID,
		TriggerBy:   1,
	})
	require.NoError(t, err)
	require.Equal(t, StatusSuccess, restoreResult.Status)

	// 验证数据完全一致
	var newAppCount, newCardCount int64
	db.Model(&model.App{}).Count(&newAppCount)
	db.Model(&model.AppCard{}).Count(&newCardCount)
	assert.Equal(t, origAppCount, newAppCount, "app 行数应恢复一致")
	assert.Equal(t, origCardCount, newCardCount, "app_card 行数应恢复一致")

	// 验证恢复审计日志的 restored_from 关联
	var restoreLog model.SystemBackupLog
	require.NoError(t, db.First(&restoreLog, restoreResult.LogID).Error)
	assert.Equal(t, backupResult.LogID, restoreLog.RestoredFrom)
	assert.Equal(t, BackupTypeRestoreSource, restoreLog.BackupType)
}

// ============== 10. 边界场景 ==============

func TestCreateBackup_EmptyDatabase(t *testing.T) {
	// 空业务数据库也能备份（sys_config 表有 6 行配置数据但业务表 app/app_card 为空）
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress: "0",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	result, err := mgr.CreateBackup(context.Background(), BackupOptions{
		BackupType: BackupTypeManual,
		TriggerBy:  1,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	// 业务表（app/app_card）应为 0 行；sys_config 有 6 行配置
	assert.Equal(t, int64(6), result.RowsCount, "应仅备份 sys_config 表的 6 行配置")
}

func TestCreateBackup_ChecksumIsSHA256Hex(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCompress: "0",
	})
	t.Cleanup(func() { cleanupBackupDir(t, "data/backups_test") })

	mgr := NewManager(db, cache)
	result, err := mgr.CreateBackup(context.Background(), BackupOptions{
		BackupType: BackupTypeManual,
		TriggerBy:  1,
	})
	require.NoError(t, err)

	// 校验和应为 64 字符 hex
	assert.Len(t, result.Checksum, 64)
	// 应能 hex 解码
	_, err = hex.DecodeString(result.Checksum)
	assert.NoError(t, err)

	// 文件内容 SHA-256 应匹配
	content, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	expected := sha256.Sum256(content)
	assert.Equal(t, hex.EncodeToString(expected[:]), result.Checksum)
}

func TestGetBackupFilePath_LogNotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	_, err := mgr.GetBackupFilePath(context.Background(), 9999)
	assert.Error(t, err)
}

func TestGetBackupFilePath_StatusNotSuccess(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	log := &model.SystemBackupLog{
		BackupType: BackupTypeManual,
		Status:     StatusFailed,
	}
	require.NoError(t, db.Create(log).Error)

	mgr := NewManager(db, cache)
	_, err := mgr.GetBackupFilePath(context.Background(), log.ID)
	assert.Error(t, err)
}

func TestIsAutoBackupEnabled(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyAutoEnabled: "1",
	})
	mgr := NewManager(db, cache)
	assert.True(t, mgr.IsAutoBackupEnabled(context.Background()))
}
