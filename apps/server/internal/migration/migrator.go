// Package migration 数据库迁移管理
// 实现轻量级 SQL 文件迁移机制，替代 mysql entrypoint 自动执行
// 严格遵循铁律 04/05：迁移目录路径走配置，不硬编码
//
// 设计：
//  1. schema_migrations 表跟踪已应用的版本号
//  2. 启动时扫描 migrations 目录下的 *.up.sql 文件
//  3. 按文件名前缀数字排序，跳过已应用版本，逐个执行未应用版本
//  4. 每个迁移在事务中执行，失败时标记 dirty 并阻止启动
//  5. 支持多语句 SQL（依赖 DSN 中的 multiStatements=true）
//
// v0.6.2 修复（dirty 恢复 + 并发保护）：
//   - 引入 MySQL advisory lock（GET_LOCK / RELEASE_LOCK），避免多个 server 实例同时迁移
//   - 引入 MIGRATION_REPAIR_DIRTY=true，管理员可显式开启 dirty 状态修复流程
//   - 默认行为：发现 dirty 状态拒绝启动并输出安全恢复建议（绝不静默修复）
//   - 修复流程：读取 dirty 版本 → 重新执行幂等迁移 → 成功则 dirty=false，失败则保留 dirty=true
//   - 错误消息包含：dirty 版本、迁移文件路径、数据库目标、建议命令、是否需要备份
//   - 不再建议 `docker compose down -v` 或直接 `DELETE FROM schema_migrations WHERE dirty=1`
package migration

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SchemaMigration 迁移跟踪表
type SchemaMigration struct {
	Version   int       `gorm:"primaryKey;column:version"`
	AppliedAt time.Time `gorm:"column:applied_at;autoCreateTime"`
	Dirty     bool      `gorm:"column:dirty"`
}

// TableName 表名
func (SchemaMigration) TableName() string { return "schema_migrations" }

// Migration 单个迁移文件
type Migration struct {
	Version int    // 文件名前缀数字
	Name    string // 文件名（不含扩展名）
	UpFile  string // .up.sql 完整路径
}

// advisoryLockName 用于 GET_LOCK 的命名锁（数据库级，跨连接生效）
const advisoryLockName = "keyauth_migration_lock"

// advisoryLockTimeout GET_LOCK 超时秒数（0=非阻塞，立即返回；-1=无限等待）
// 这里用 30 秒：让并发实例有合理等待时间，避免立即放弃
const advisoryLockTimeout = 30

// Config 迁移器运行时配置（由 config.go 注入）
type Config struct {
	// Auto 启动时是否自动执行迁移
	Auto bool
	// Dir 迁移文件目录（绝对路径或相对工作目录）
	Dir string
	// RepairDirty 是否允许在 dirty 状态下重试修复（仅管理员显式开启）
	// 对应环境变量 MIGRATION_REPAIR_DIRTY=true
	RepairDirty bool
	// DBTarget 数据库连接目标描述（用于错误消息，如 "mysql:3306/keyauth"）
	DBTarget string
}

// Run 执行数据库迁移（默认入口）
// 流程：
//  1. 确保 schema_migrations 表存在
//  2. 获取 advisory lock（防止多实例并发迁移）
//  3. 检查 dirty 状态：
//     - 默认（RepairDirty=false）：拒绝启动，输出安全恢复建议
//     - 显式修复（RepairDirty=true）：进入 repair 流程
//  4. 扫描 migrationsDir 下的 *.up.sql 文件
//  5. 按版本号排序，跳过已应用版本，逐个执行未应用版本
//
// 幂等：可重复调用，已应用的迁移不会重复执行
func Run(db *gorm.DB, migrationsDir string) error {
	return RunWithConfig(db, Config{
		Auto:        true,
		Dir:         migrationsDir,
		RepairDirty: false,
		DBTarget:    "(unknown)",
	})
}

// RunWithConfig 带配置的迁移执行入口（v0.6.2 新增）
// 支持 MIGRATION_REPAIR_DIRTY 显式开启 dirty 修复
func RunWithConfig(db *gorm.DB, cfg Config) error {
	if !cfg.Auto {
		log.Printf("[MIGRATE] 自动迁移已禁用（MIGRATION_AUTO!=true），跳过")
		return nil
	}

	// 1. 确保 schema_migrations 表存在
	if err := db.AutoMigrate(&SchemaMigration{}); err != nil {
		return fmt.Errorf("创建 schema_migrations 表失败: %w", err)
	}

	// 2. 获取 advisory lock（并发保护）
	// 关键：多个 server 容器同时启动时，只有一个能拿到锁执行迁移
	// 拿不到锁的实例会在 advisoryLockTimeout 秒后失败，重启时重试
	lockAcquired, err := acquireAdvisoryLock(db)
	if err != nil {
		return fmt.Errorf("获取迁移锁失败（DBTarget=%s）: %w", cfg.DBTarget, err)
	}
	if !lockAcquired {
		return fmt.Errorf("获取迁移锁失败：另一个实例正在执行迁移（DBTarget=%s，等待 %d 秒超时）。\n"+
			"建议：等待另一个实例完成迁移后重启本实例；或检查是否有残留锁：SELECT RELEASE_LOCK('%s');",
			cfg.DBTarget, advisoryLockTimeout, advisoryLockName)
	}
	log.Printf("[MIGRATE] 已获取 advisory lock：%s", advisoryLockName)

	// 3. 检查 dirty 状态
	var dirty SchemaMigration
	if err := db.Where("dirty = ?", true).First(&dirty).Error; err == nil {
		// 发现 dirty 状态
		if !cfg.RepairDirty {
			// 默认行为：拒绝启动，输出安全恢复建议
			return formatDirtyError(db, dirty.Version, cfg.DBTarget, cfg.Dir)
		}
		// 管理员显式开启修复模式
		log.Printf("[MIGRATE] MIGRATION_REPAIR_DIRTY=true，进入 dirty 修复流程，version=%d", dirty.Version)
		if err := repairDirtyMigration(db, dirty.Version, cfg.Dir, cfg.DBTarget); err != nil {
			// 修复失败：保留 dirty=true，返回错误
			return fmt.Errorf("dirty 修复失败，version=%d（dirty 状态已保留，不会丢失）: %w", dirty.Version, err)
		}
		log.Printf("[MIGRATE] dirty 修复成功，version=%d，继续执行后续迁移", dirty.Version)
	}

	// 4. 扫描 .up.sql 文件
	if info, err := os.Stat(cfg.Dir); err != nil {
		return fmt.Errorf("迁移目录不存在 %s: %w", cfg.Dir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("迁移路径不是目录: %s", cfg.Dir)
	}

	pattern := filepath.Join(cfg.Dir, "*.up.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("扫描迁移文件失败 %s: %w", pattern, err)
	}
	if len(files) == 0 {
		log.Printf("[MIGRATE] 迁移目录 %s 无 .up.sql 文件，跳过", cfg.Dir)
		releaseAdvisoryLock(db)
		return nil
	}

	// 5. 解析版本号并排序
	migrations, err := parseMigrations(files)
	if err != nil {
		releaseAdvisoryLock(db)
		return err
	}

	// 6. 逐个执行未应用的迁移
	applied := 0
	for _, m := range migrations {
		var existing SchemaMigration
		result := db.Where("version = ?", m.Version).First(&existing)
		if result.Error == nil {
			// 已应用（dirty=false 在前面已处理）
			continue
		}

		log.Printf("[MIGRATE] 应用迁移 %d: %s", m.Version, m.Name)
		if err := applyMigration(db, m); err != nil {
			// 迁移失败：保留 dirty=true，释放锁，返回错误
			releaseAdvisoryLock(db)
			return fmt.Errorf("迁移 %d (%s) 失败: %w", m.Version, m.Name, err)
		}
		applied++
		log.Printf("[MIGRATE] 迁移 %d 完成", m.Version)
	}

	if applied == 0 {
		log.Printf("[MIGRATE] 数据库已是最新版本（共 %d 个迁移文件，全部已应用）", len(migrations))
	} else {
		log.Printf("[MIGRATE] 本次应用 %d 个迁移，剩余 %d 个已应用", applied, len(migrations)-applied)
	}

	// 7. 释放锁
	releaseAdvisoryLock(db)
	return nil
}

// acquireAdvisoryLock 获取 MySQL advisory lock
// 返回 (true, nil) 表示成功获取；(false, nil) 表示已被其他实例持有；(false, err) 表示出错
func acquireAdvisoryLock(db *gorm.DB) (bool, error) {
	var result int
	// GET_LOCK 返回：1=成功，0=超时未获取，NULL=出错（如线程被 kill）
	// 注意：MySQL 5.7+ 第二参数为超时秒数
	sql := fmt.Sprintf("SELECT GET_LOCK('%s', %d)", advisoryLockName, advisoryLockTimeout)
	if err := db.Raw(sql).Scan(&result).Error; err != nil {
		return false, err
	}
	switch result {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, fmt.Errorf("GET_LOCK 返回异常值 %d", result)
	}
}

// releaseAdvisoryLock 释放 advisory lock（忽略错误，因为锁有 session 级生命周期）
func releaseAdvisoryLock(db *gorm.DB) {
	sql := fmt.Sprintf("SELECT RELEASE_LOCK('%s')", advisoryLockName)
	var result int
	if err := db.Raw(sql).Scan(&result).Error; err != nil {
		log.Printf("[MIGRATE] 释放 advisory lock 失败（可忽略）: %v", err)
		return
	}
	if result == 1 {
		log.Printf("[MIGRATE] 已释放 advisory lock：%s", advisoryLockName)
	} else {
		// result == 0 表示锁不属于当前连接（可能已超时被释放）；NULL 表示锁不存在
		log.Printf("[MIGRATE] 释放 advisory lock：result=%d（可能已超时释放）", result)
	}
}

// formatDirtyError 生成 dirty 状态错误消息（默认拒绝启动时使用）
// 错误消息包含：dirty 版本、迁移文件路径、数据库目标、建议命令、备份提示
func formatDirtyError(db *gorm.DB, version int, dbTarget, migrationsDir string) error {
	// 定位迁移文件
	migrationFile := ""
	upFile := filepath.Join(migrationsDir, fmt.Sprintf("%03d_*.up.sql", version))
	if matches, err := filepath.Glob(upFile); err == nil && len(matches) > 0 {
		migrationFile = matches[0]
	}
	if migrationFile == "" {
		// 兜底：列出所有文件让用户自查
		migrationFile = fmt.Sprintf("(未找到 version=%03d 的 .up.sql，请检查 %s)", version, migrationsDir)
	}

	return fmt.Errorf(`
============================================================
[FATAL] 数据库迁移处于 dirty 状态，version=%d
============================================================
【诊断信息】
  dirty 版本：%d
  迁移文件：%s
  数据库目标：%s
  成因：之前启动时该版本迁移执行到一半失败（MySQL DDL 隐式提交导致事务回滚无效，
        部分 DDL 已落库，schema_migrations.dirty=1 被持久化）

【为什么默认拒绝启动】
  继续启动业务服务可能导致：
  - 半成品 schema 与代码不匹配，运行时数据损坏
  - 后续迁移在半成品基础上继续执行，错误累积
  - 因此 dirty 状态必须先修复，绝不能静默跳过

【安全恢复方式（推荐）】
  步骤 1：先备份数据库（强烈建议）
    docker compose exec -T mysql mysqldump -uroot -p<密码> <数据库名> > backup_$(date +%%Y%%m%%d_%%H%%M%%S).sql

  步骤 2：开启修复模式重新执行幂等迁移
    cd <项目根目录>
    # 编辑 .env 增加：MIGRATION_REPAIR_DIRTY=true
    echo "MIGRATION_REPAIR_DIRTY=true" >> .env
    # 在 docker-compose.yml 的 server 服务 environment 增加该变量（若无）
    docker compose up -d server

  步骤 3：观察 server 日志确认修复结果
    docker compose logs -f server
    # 成功：会看到 "dirty 修复成功，version=N"（N 为具体版本号）
    # 失败：dirty=true 仍保留，需查看具体 SQL 错误

  步骤 4：修复成功后移除 MIGRATION_REPAIR_DIRTY
    sed -i '/^MIGRATION_REPAIR_DIRTY=/d' .env
    docker compose up -d server

【辅助工具】
  查看当前 dirty 状态：
    bash scripts/clean_dirty_migration.sh --show
  交互式修复（含备份、dry-run、repair 模式）：
    bash scripts/clean_dirty_migration.sh --dry-run
    bash scripts/clean_dirty_migration.sh --repair

【禁止行为】
  × 不要直接执行 DELETE FROM schema_migrations WHERE version=N;（N 为具体版本号，会跳过失败迁移）
  × 不要执行 docker compose down -v（会删除所有业务数据）
  × 不要忽略此错误继续启动（可能导致数据损坏）`, version, version, migrationFile, dbTarget)
}

// repairDirtyMigration 修复 dirty 状态（仅 MIGRATION_REPAIR_DIRTY=true 时调用）
// 流程：
//  1. 定位 dirty 版本对应的迁移文件
//  2. 重新执行该迁移（必须幂等，否则会再次失败）
//  3. 成功 → 标记 dirty=false
//  4. 失败 → 保留 dirty=true，返回错误
func repairDirtyMigration(db *gorm.DB, version int, migrationsDir, dbTarget string) error {
	// 1. 定位迁移文件
	pattern := filepath.Join(migrationsDir, fmt.Sprintf("%03d_*.up.sql", version))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("查找迁移文件失败 pattern=%s: %w", pattern, err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("未找到 version=%d 的迁移文件（pattern=%s），无法修复", version, pattern)
	}

	m := Migration{
		Version: version,
		Name:    strings.TrimSuffix(filepath.Base(matches[0]), ".up.sql"),
		UpFile:  matches[0],
	}

	log.Printf("[MIGRATE-REPAIR] 重新执行迁移 %d: %s（幂等迁移，已有对象会被跳过）", m.Version, m.Name)
	log.Printf("[MIGRATE-REPAIR] 数据库目标：%s", dbTarget)
	log.Printf("[MIGRATE-REPAIR] 迁移文件：%s", m.UpFile)

	// 2. 重新执行迁移
	// 关键：applyMigration 内部会先尝试 UPDATE dirty=true（已有记录），失败则 INSERT
	// 这里调用 repairApplyMigration，跳过 INSERT，直接重试 SQL
	if err := repairApplyMigration(db, m); err != nil {
		// 修复失败：dirty=true 已保留（在 repairApplyMigration 中确保）
		log.Printf("[MIGRATE-REPAIR] 修复失败，dirty=true 已保留： %v", err)
		return err
	}

	// 3. 修复成功：标记 dirty=false
	if err := db.Model(&SchemaMigration{}).
		Where("version = ?", m.Version).
		Updates(map[string]interface{}{
			"dirty":      false,
			"applied_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("更新 schema_migrations 状态失败（SQL 已执行成功，但 dirty 标记未更新）: %w", err)
	}

	log.Printf("[MIGRATE-REPAIR] 迁移 %d 修复成功，dirty=false 已标记", m.Version)
	return nil
}

// repairApplyMigration 修复模式下执行迁移 SQL
// 与 applyMigration 的区别：
//  - 不创建新的 schema_migrations 记录（已存在 dirty=true）
//  - 确保 dirty=true 状态（防止意外被改为 false）
//  - SQL 执行失败时保留 dirty=true
func repairApplyMigration(db *gorm.DB, m Migration) error {
	// 0. 确认记录存在且 dirty=true
	var existing SchemaMigration
	if err := db.Where("version = ?", m.Version).First(&existing).Error; err != nil {
		return fmt.Errorf("schema_migrations 中未找到 version=%d 记录: %w", m.Version, err)
	}
	if !existing.Dirty {
		log.Printf("[MIGRATE-REPAIR] version=%d 已不是 dirty 状态，跳过 SQL 执行", m.Version)
		return nil
	}

	// 1. 读取 SQL 文件
	content, err := os.ReadFile(m.UpFile)
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %w", err)
	}

	// 2. 确保 dirty=true（防止被其他流程改回 false）
	if err := db.Model(&SchemaMigration{}).
		Where("version = ?", m.Version).
		Update("dirty", true).Error; err != nil {
		return fmt.Errorf("锁定 dirty=true 失败: %w", err)
	}

	// 3. 执行 SQL（事务包裹；MySQL DDL 会隐式提交，但非 DDL 仍受事务保护）
	// 注：迁移 SQL 必须幂等（CREATE TABLE IF NOT EXISTS / INSERT ON DUPLICATE KEY UPDATE 等）
	txErr := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("执行 SQL 失败: %w", err)
		}
		return nil
	})

	if txErr != nil {
		// 失败：确保 dirty=true（DDL 隐式提交可能已改部分 schema，但 dirty 仍需保留）
		_ = db.Model(&SchemaMigration{}).
			Where("version = ?", m.Version).
			Update("dirty", true).Error
		return txErr
	}

	return nil
}

// parseMigrations 解析迁移文件列表，返回按版本号排序的迁移
func parseMigrations(files []string) ([]Migration, error) {
	var migrations []Migration
	for _, f := range files {
		base := filepath.Base(f)
		// 文件名格式：001_init_schema.up.sql
		// 取第一个 _ 之前的部分作为版本号
		idx := strings.Index(base, "_")
		if idx <= 0 {
			log.Printf("[MIGRATE] 跳过不符合命名规范的文件: %s", base)
			continue
		}
		versionStr := base[:idx]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			log.Printf("[MIGRATE] 跳过版本号非数字的文件: %s", base)
			continue
		}
		migrations = append(migrations, Migration{
			Version: version,
			Name:    strings.TrimSuffix(base, ".up.sql"),
			UpFile:  f,
		})
	}

	if len(migrations) == 0 {
		return nil, fmt.Errorf("未找到有效的迁移文件")
	}

	// 检查版本号唯一
	seen := make(map[int]bool)
	for _, m := range migrations {
		if seen[m.Version] {
			return nil, fmt.Errorf("版本号 %d 重复", m.Version)
		}
		seen[m.Version] = true
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// applyMigration 执行单个迁移文件（事务保护）
// 流程：
//  1. 标记该版本为 dirty（INSERT 新记录）
//  2. 读取 SQL 文件内容
//  3. 在事务中执行 SQL（注意：MySQL DDL 会隐式提交，事务对 DDL 无效）
//  4. 事务成功 → 标记为已完成（dirty=false）
//  5. 事务失败 → 保留 dirty=true，返回错误（不吞错误）
func applyMigration(db *gorm.DB, m Migration) error {
	// 1. 标记 dirty（INSERT）
	sm := SchemaMigration{
		Version:   m.Version,
		AppliedAt: time.Now(),
		Dirty:     true,
	}
	if err := db.Create(&sm).Error; err != nil {
		// INSERT 失败可能是因为记录已存在（非 dirty 流程中重新执行）
		// 此时改为 UPDATE dirty=true
		if err := db.Model(&SchemaMigration{}).
			Where("version = ?", m.Version).
			Update("dirty", true).Error; err != nil {
			return fmt.Errorf("写入 schema_migrations 失败: %w", err)
		}
	}

	// 2. 读取 SQL 文件
	content, err := os.ReadFile(m.UpFile)
	if err != nil {
		// 文件读取失败：保留 dirty=true，返回错误
		return fmt.Errorf("读取迁移文件失败（dirty=true 已保留）: %w", err)
	}

	// 3. 事务执行
	// 注意：MySQL DDL（CREATE/ALTER TABLE）会触发隐式 COMMIT，事务回滚对 DDL 无效
	// 因此迁移 SQL 必须幂等（CREATE TABLE IF NOT EXISTS / INSERT ON DUPLICATE KEY UPDATE 等）
	// 这样即使部分 DDL 已落库，重新执行也能成功
	txErr := db.Transaction(func(tx *gorm.DB) error {
		// db.Exec 支持多语句（DSN 已配置 multiStatements=true）
		if err := tx.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("执行 SQL 失败: %w", err)
		}
		return nil
	})

	if txErr != nil {
		// 事务回滚：DDL 隐式提交的部分已落库无法回滚
		// 确保 dirty=true 持久化（不吞错误，返回详细错误信息）
		_ = db.Model(&SchemaMigration{}).
			Where("version = ?", m.Version).
			Update("dirty", true).Error
		return txErr
	}

	// 4. 标记完成
	if err := db.Model(&SchemaMigration{}).
		Where("version = ?", m.Version).
		Updates(map[string]interface{}{
			"dirty":      false,
			"applied_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("更新 schema_migrations 状态失败（SQL 已执行成功，但 dirty 标记未更新）: %w", err)
	}

	return nil
}
