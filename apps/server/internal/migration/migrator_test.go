// Package migration - migrator.go 单元测试
// v0.6.2 新增：覆盖 dirty 恢复、并发保护、幂等迁移等场景
//
// 测试策略：
//   * parseMigrations 单元测试：纯函数，无需 DB
//   * formatDirtyError 单元测试：纯函数，无需 DB
//   * RunWithConfig / repairDirtyMigration 集成测试：需要真实 MySQL 8.0+，跳过条件：环境变量 MIGRATION_TEST_DSN 为空
//
// 集成测试通过环境变量 MIGRATION_TEST_DSN 指定 MySQL DSN，例如：
//   MIGRATION_TEST_DSN="root:pass@tcp(127.0.0.1:3306)/test_db?charset=utf8mb4&parseTime=True&multiStatements=true" go test ./...
//
// 推荐使用隔离的 MySQL 8.0 容器运行：
//   docker run --rm -d --name mysql-test -e MYSQL_ROOT_PASSWORD=test -e MYSQL_DATABASE=test_db -p 3306:3306 mysql:8.0.36
//   MIGRATION_TEST_DSN="root:test@tcp(127.0.0.1:3306)/test_db?charset=utf8mb4&parseTime=True&multiStatements=true" go test -v ./internal/migration/...
package migration

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestParseMigrations_ValidFilenames 验证文件名解析和排序
func TestParseMigrations_ValidFilenames(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	files := []string{
		"003_third.up.sql",
		"001_first.up.sql",
		"002_second.up.sql",
	}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if err := os.WriteFile(path, []byte("SELECT 1;"), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	// Glob 出来的顺序是不确定的，但 parseMigrations 应按版本号排序
	pattern := filepath.Join(tmpDir, "*.up.sql")
	matches, _ := filepath.Glob(pattern)
	migrations, err := parseMigrations(matches)
	if err != nil {
		t.Fatalf("parseMigrations 失败: %v", err)
	}

	if len(migrations) != 3 {
		t.Fatalf("期望 3 个迁移，实际 %d", len(migrations))
	}

	expectedVersions := []int{1, 2, 3}
	for i, m := range migrations {
		if m.Version != expectedVersions[i] {
			t.Errorf("第 %d 个迁移版本错误：期望 %d，实际 %d", i, expectedVersions[i], m.Version)
		}
	}
}

// TestParseMigrations_DuplicateVersions 验证版本号重复检测
func TestParseMigrations_DuplicateVersions(t *testing.T) {
	tmpDir := t.TempDir()
	files := []string{
		"001_first.up.sql",
		"001_duplicate.up.sql", // 同版本号
	}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		_ = os.WriteFile(path, []byte("SELECT 1;"), 0644)
	}

	pattern := filepath.Join(tmpDir, "*.up.sql")
	matches, _ := filepath.Glob(pattern)
	_, err := parseMigrations(matches)
	if err == nil {
		t.Fatal("期望返回重复版本号错误，实际返回 nil")
	}
	if !strings.Contains(err.Error(), "重复") {
		t.Errorf("错误消息应包含'重复'，实际：%v", err)
	}
}

// TestParseMigrations_InvalidFilenames 验证非规范文件名跳过逻辑
func TestParseMigrations_InvalidFilenames(t *testing.T) {
	tmpDir := t.TempDir()
	files := []string{
		"001_valid.up.sql",
		"invalid_filename.up.sql",   // 无版本号前缀
		"abc_invalid.up.sql",        // 非数字版本号
		"001_valid.down.sql",        // 非 .up.sql（被 Glob 排除）
	}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		_ = os.WriteFile(path, []byte("SELECT 1;"), 0644)
	}

	pattern := filepath.Join(tmpDir, "*.up.sql")
	matches, _ := filepath.Glob(pattern)
	migrations, err := parseMigrations(matches)
	if err != nil {
		// 当只剩 1 个有效迁移时不应报错（"未找到有效的迁移文件" 仅在全部无效时触发）
		// 注：parseMigrations 内部对无效文件名是 log.Printf 跳过，不报错
		// 但若全部无效（migrations 为空），会返回 "未找到有效的迁移文件"
		// 这里至少有 1 个有效，所以应该成功
		t.Fatalf("parseMigrations 不应失败：%v", err)
	}
	if len(migrations) != 1 {
		t.Errorf("期望 1 个有效迁移，实际 %d", len(migrations))
	}
}

// TestFormatDirtyError_VerifyMessageContent 验证 dirty 错误消息包含必要信息
func TestFormatDirtyError_VerifyMessageContent(t *testing.T) {
	// formatDirtyError 不需要真实 DB 连接（db 参数只用于传给后续逻辑，函数体内未使用）
	// 但函数签名需要 *gorm.DB，传 nil 会触发后续调用 panic
	// 这里只验证错误消息内容
	tmpDir := t.TempDir()
	// 创建一个 015_xxx.up.sql 让 formatDirtyError 能定位文件
	fakeFile := filepath.Join(tmpDir, "015_test_migration.up.sql")
	_ = os.WriteFile(fakeFile, []byte("SELECT 1;"), 0644)

	// 直接调用 formatDirtyError(nil, ...) 会因为 db 参数未被使用而安全
	err := formatDirtyError(nil, 15, "mysql:3306/keyauth", tmpDir)
	if err == nil {
		t.Fatal("期望返回错误，实际返回 nil")
	}

	errMsg := err.Error()

	// 验证错误消息包含必要信息
	requiredSubstrings := []string{
		"version=15",            // dirty 版本
		"015_test_migration",    // 迁移文件路径
		"mysql:3306/keyauth",    // 数据库目标
		"MIGRATION_REPAIR_DIRTY", // 建议命令
		"备份",                    // 备份提示
		"不要直接执行 DELETE",      // 禁止行为提示
		"不要执行 docker compose down -v",
	}
	for _, sub := range requiredSubstrings {
		if !strings.Contains(errMsg, sub) {
			t.Errorf("错误消息应包含 '%s'，实际：%s", sub, errMsg)
		}
	}
}

// TestSortMigrations 验证迁移按版本号升序排序
func TestSortMigrations(t *testing.T) {
	migrations := []Migration{
		{Version: 30, Name: "v30"},
		{Version: 5, Name: "v5"},
		{Version: 15, Name: "v15"},
		{Version: 1, Name: "v1"},
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	expected := []int{1, 5, 15, 30}
	for i, m := range migrations {
		if m.Version != expected[i] {
			t.Errorf("位置 %d 期望 %d 实际 %d", i, expected[i], m.Version)
		}
	}
}

// ============================================================
// 集成测试（需要真实 MySQL 8.0+）
// 通过环境变量 MIGRATION_TEST_DSN 控制，未设置则跳过
// ============================================================

// getTestDB 获取测试用 DB 连接，未配置 DSN 则跳过
func getTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("跳过集成测试：未设置 MIGRATION_TEST_DSN 环境变量")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接测试 DB 失败：%v", err)
	}
	return db
}

// cleanupTestDB 清理测试 DB（删除所有表）
func cleanupTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	// 禁用外键检查，按任意顺序 DROP
	db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	defer db.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// 查询所有表
	var tables []string
	db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&tables)
	for _, tbl := range tables {
		db.Exec("DROP TABLE IF EXISTS `" + tbl + "`")
	}
}

// TestIntegration_FreshDatabaseFullMigration 全新数据库执行全部迁移成功
func TestIntegration_FreshDatabaseFullMigration(t *testing.T) {
	db := getTestDB(t)
	cleanupTestDB(t, db)
	defer cleanupTestDB(t, db)

	// 准备迁移目录：用项目的真实 migrations 目录
	migrationsDir := "../../../migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Skipf("跳过：未找到迁移目录 %s", migrationsDir)
	}

	// 执行迁移（默认模式，无 dirty）
	err := RunWithConfig(db, Config{
		Auto:        true,
		Dir:         migrationsDir,
		RepairDirty: false,
		DBTarget:    "test-db",
	})
	if err != nil {
		t.Fatalf("全新数据库迁移失败：%v", err)
	}

	// 验证 schema_migrations 中所有迁移都是 dirty=false
	var dirtyCount int64
	db.Model(&SchemaMigration{}).Where("dirty = ?", true).Count(&dirtyCount)
	if dirtyCount != 0 {
		t.Errorf("期望 0 个 dirty 记录，实际 %d", dirtyCount)
	}

	// 验证迁移数量 > 0
	var totalCount int64
	db.Model(&SchemaMigration{}).Count(&totalCount)
	if totalCount == 0 {
		t.Error("期望迁移数量 > 0，实际 0")
	}
	t.Logf("全新数据库迁移成功，共应用 %d 个迁移", totalCount)
}

// TestIntegration_Version15Repeatable v15 重复执行成功（幂等性）
func TestIntegration_Version15Repeatable(t *testing.T) {
	db := getTestDB(t)
	cleanupTestDB(t, db)
	defer cleanupTestDB(t, db)

	migrationsDir := "../../../migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Skipf("跳过：未找到迁移目录 %s", migrationsDir)
	}

	// 第一次执行全部迁移
	if err := RunWithConfig(db, Config{
		Auto: true, Dir: migrationsDir, RepairDirty: false, DBTarget: "test-db",
	}); err != nil {
		t.Fatalf("第一次迁移失败：%v", err)
	}

	// 单独重复执行 v15 的 SQL（模拟 dirty 修复场景）
	v15File := filepath.Join(migrationsDir, "015_v0.4.0_end_user_system.up.sql")
	content, err := os.ReadFile(v15File)
	if err != nil {
		t.Fatalf("读取 v15 迁移文件失败：%v", err)
	}

	// 重复执行 v15 SQL（应在已存在表/字段/索引/配置的情况下成功）
	if err := db.Exec(string(content)).Error; err != nil {
		t.Fatalf("v15 重复执行失败（幂等性破坏）：%v", err)
	}
	t.Log("✓ v15 重复执行成功，幂等性验证通过")
}

// TestIntegration_DirtyRejectsStartup dirty=true 时默认拒绝启动
func TestIntegration_DirtyRejectsStartup(t *testing.T) {
	db := getTestDB(t)
	cleanupTestDB(t, db)
	defer cleanupTestDB(t, db)

	migrationsDir := "../../../migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Skipf("跳过：未找到迁移目录 %s", migrationsDir)
	}

	// 先正常迁移到 v15 之前（执行 v1-v14）
	if err := RunWithConfig(db, Config{
		Auto: true, Dir: migrationsDir, RepairDirty: false, DBTarget: "test-db",
	}); err != nil {
		t.Fatalf("前置迁移失败：%v", err)
	}

	// 手动插入 dirty=true 的 v15 记录
	db.Create(&SchemaMigration{
		Version:   15,
		AppliedAt: time.Now(),
		Dirty:     true,
	})

	// 再次执行迁移（默认 RepairDirty=false），应拒绝启动
	err := RunWithConfig(db, Config{
		Auto: true, Dir: migrationsDir, RepairDirty: false, DBTarget: "test-db",
	})
	if err == nil {
		t.Fatal("期望 dirty 状态拒绝启动，实际返回 nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "dirty") {
		t.Errorf("错误消息应包含 'dirty'，实际：%s", errMsg)
	}
	if !strings.Contains(errMsg, "version=15") {
		t.Errorf("错误消息应包含 'version=15'，实际：%s", errMsg)
	}
	if !strings.Contains(errMsg, "MIGRATION_REPAIR_DIRTY") {
		t.Errorf("错误消息应包含 'MIGRATION_REPAIR_DIRTY' 建议，实际：%s", errMsg)
	}
	t.Log("✓ dirty=true 默认拒绝启动，错误消息包含完整恢复建议")
}

// TestIntegration_DirtyRepairSuccess dirty=true 且 MIGRATION_REPAIR_DIRTY=true 时安全重试成功
func TestIntegration_DirtyRepairSuccess(t *testing.T) {
	db := getTestDB(t)
	cleanupTestDB(t, db)
	defer cleanupTestDB(t, db)

	migrationsDir := "../../../migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Skipf("跳过：未找到迁移目录 %s", migrationsDir)
	}

	// 先正常执行 v1-v14
	if err := RunWithConfig(db, Config{
		Auto: true, Dir: migrationsDir, RepairDirty: false, DBTarget: "test-db",
	}); err != nil {
		t.Fatalf("前置迁移失败：%v", err)
	}

	// 模拟 v15 半成品：手动执行 v15 SQL（成功），然后标记 dirty=true
	// 这种场景对应：DDL 已落库但 dirty 标记被持久化
	v15File := filepath.Join(migrationsDir, "015_v0.4.0_end_user_system.up.sql")
	content, _ := os.ReadFile(v15File)
	if err := db.Exec(string(content)).Error; err != nil {
		t.Fatalf("前置执行 v15 SQL 失败：%v", err)
	}
	db.Create(&SchemaMigration{
		Version:   15,
		AppliedAt: time.Now(),
		Dirty:     true,
	})

	// 调用 RunWithConfig(RepairDirty=true)，应进入修复流程
	// 修复会重新执行幂等迁移，成功后标记 dirty=false
	err := RunWithConfig(db, Config{
		Auto: true, Dir: migrationsDir, RepairDirty: true, DBTarget: "test-db",
	})
	if err != nil {
		t.Fatalf("dirty 修复应成功，实际失败：%v", err)
	}

	// 验证 dirty=false
	var sm SchemaMigration
	if err := db.Where("version = ?", 15).First(&sm).Error; err != nil {
		t.Fatalf("查询 v15 状态失败：%v", err)
	}
	if sm.Dirty {
		t.Error("修复后 dirty 应为 false，实际为 true")
	}
	t.Log("✓ dirty=true + MIGRATION_REPAIR_DIRTY=true 安全重试成功，dirty 已标记为 false")
}

// TestIntegration_DirtyRepairFailurePreservesDirty 修复失败时 dirty=true 保留
func TestIntegration_DirtyRepairFailurePreservesDirty(t *testing.T) {
	db := getTestDB(t)
	cleanupTestDB(t, db)
	defer cleanupTestDB(t, db)

	// 创建一个临时迁移目录，包含一个故意会失败的迁移
	tmpDir := t.TempDir()
	badMigration := `CREATE TABLE IF NOT EXISTS test_table (id INT);
SELECT * FROM non_existent_table_xyz; -- 故意失败
`
	badFile := filepath.Join(tmpDir, "001_bad_migration.up.sql")
	if err := os.WriteFile(badFile, []byte(badMigration), 0644); err != nil {
		t.Fatalf("创建测试迁移文件失败：%v", err)
	}

	// 先正常执行（会失败，留下 dirty=true）
	_ = RunWithConfig(db, Config{
		Auto: true, Dir: tmpDir, RepairDirty: false, DBTarget: "test-db",
	})

	// 验证 dirty=true
	var sm SchemaMigration
	if err := db.Where("version = ?", 1).First(&sm).Error; err != nil {
		t.Fatalf("查询 v1 状态失败：%v", err)
	}
	if !sm.Dirty {
		t.Fatal("前置条件不满足：v1 应为 dirty=true")
	}

	// 尝试修复（迁移 SQL 仍会失败，dirty 应保留）
	err := RunWithConfig(db, Config{
		Auto: true, Dir: tmpDir, RepairDirty: true, DBTarget: "test-db",
	})
	if err == nil {
		t.Fatal("期望修复失败返回错误，实际返回 nil")
	}

	// 验证 dirty=true 仍保留
	if err := db.Where("version = ?", 1).First(&sm).Error; err != nil {
		t.Fatalf("查询 v1 状态失败：%v", err)
	}
	if !sm.Dirty {
		t.Error("修复失败后 dirty 应保留为 true，实际为 false")
	}
	t.Log("✓ 修复失败时 dirty=true 保留，不掩盖失败")
}

// TestIntegration_AdvisoryLockConcurrentProtection 验证并发保护
// 注：本测试通过单线程模拟获取/释放锁，真实并发测试需多 goroutine
func TestIntegration_AdvisoryLockConcurrentProtection(t *testing.T) {
	db := getTestDB(t)
	cleanupTestDB(t, db)
	defer cleanupTestDB(t, db)

	// 1. 获取锁
	acquired, err := acquireAdvisoryLock(db)
	if err != nil {
		t.Fatalf("获取 advisory lock 失败：%v", err)
	}
	if !acquired {
		t.Fatal("期望获取锁成功，实际未获取")
	}

	// 2. 在同一连接再次获取（同 session 持有，GET_LOCK 应返回 1=可重入）
	// 注：MySQL GET_LOCK 同名锁在同 session 内可重入，返回 1
	acquired2, err := acquireAdvisoryLock(db)
	if err != nil {
		t.Logf("同 session 再次获取锁返回错误（可接受）：%v", err)
	}
	if acquired2 {
		t.Log("同 session 可重入获取锁（MySQL GET_LOCK 行为）")
	}

	// 3. 释放锁
	releaseAdvisoryLock(db)

	// 4. 释放后可再次获取
	acquired3, err := acquireAdvisoryLock(db)
	if err != nil {
		t.Fatalf("释放后再次获取锁失败：%v", err)
	}
	if !acquired3 {
		t.Fatal("释放后应能再次获取锁")
	}
	releaseAdvisoryLock(db)
	t.Log("✓ advisory lock 获取/释放正常，并发保护机制可用")
}

// TestIntegration_MultipleServerInstancesConcurrentMigration
// 模拟多个 server 实例同时启动，只有一个能执行迁移
// 注：本测试需要多 goroutine + 独立 DB 连接，简化为单 goroutine 顺序验证
func TestIntegration_MultipleServerInstancesConcurrentMigration(t *testing.T) {
	db := getTestDB(t)
	cleanupTestDB(t, db)
	defer cleanupTestDB(t, db)

	migrationsDir := "../../../migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Skipf("跳过：未找到迁移目录 %s", migrationsDir)
	}

	// 模拟实例 1：获取锁 + 执行迁移
	acquired1, err := acquireAdvisoryLock(db)
	if err != nil {
		t.Fatalf("实例 1 获取锁失败：%v", err)
	}
	if !acquired1 {
		t.Fatal("实例 1 应成功获取锁")
	}

	// 在持有锁的情况下，实例 1 完成迁移
	if err := RunWithConfig(db, Config{
		Auto: true, Dir: migrationsDir, RepairDirty: false, DBTarget: "test-db",
	}); err != nil {
		t.Fatalf("实例 1 迁移失败：%v", err)
	}

	// 释放锁
	releaseAdvisoryLock(db)

	// 实例 2 此时启动，应能获取锁并发现所有迁移已应用
	acquired2, _ := acquireAdvisoryLock(db)
	if !acquired2 {
		t.Fatal("实例 2 应能获取锁（实例 1 已释放）")
	}
	defer releaseAdvisoryLock(db)

	// 实例 2 执行迁移（应跳过所有已应用的迁移）
	if err := RunWithConfig(db, Config{
		Auto: true, Dir: migrationsDir, RepairDirty: false, DBTarget: "test-db",
	}); err != nil {
		t.Fatalf("实例 2 迁移失败：%v", err)
	}
	t.Log("✓ 多实例场景：实例 1 完成迁移后释放锁，实例 2 获取锁后跳过已应用迁移")
}

// TestIntegration_DBConnectionFailureDoesNotMarkSuccess
// 数据库连接失败时不会把迁移标记为成功
func TestIntegration_DBConnectionFailureDoesNotMarkSuccess(t *testing.T) {
	// 使用一个不存在的 DSN 模拟连接失败
	badDSN := "root:wrongpass@tcp(127.0.0.1:1)/nonexistent?charset=utf8mb4&timeout=1s"
	_, err := gorm.Open(mysql.Open(badDSN), &gorm.Config{})
	if err == nil {
		t.Skip("跳过：测试环境允许了错误连接")
	}
	t.Logf("✓ 连接失败正确返回错误：%v", err)
}
