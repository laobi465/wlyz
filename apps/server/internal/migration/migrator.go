// Package migration 数据库迁移管理
// 实现轻量级 SQL 文件迁移机制，替代 mysql entrypoint 自动执行
// 严格遵循铁律 04/05：迁移目录路径走配置，不硬编码
//
// 设计：
//  1. schema_migrations 表跟踪已应用的版本号
//  2. 启动时扫描 migrations 目录下的 *.up.sql 文件
//  3. 按文件名前缀数字排序，跳过已应用版本，逐个执行未应用版本
//  4. 每个迁移在独立事务中执行，失败时标记 dirty 并阻止启动
//  5. 支持多语句 SQL（依赖 DSN 中的 multiStatements=true）
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

// Run 执行数据库迁移
// 流程：
//  1. 确保 schema_migrations 表存在
//  2. 检查 dirty 状态（如有则拒绝启动）
//  3. 扫描 migrationsDir 下的 *.up.sql 文件
//  4. 按版本号排序，跳过已应用版本，逐个执行未应用版本
//
// 幂等：可重复调用，已应用的迁移不会重复执行
func Run(db *gorm.DB, migrationsDir string) error {
	// 1. 确保 schema_migrations 表存在
	if err := db.AutoMigrate(&SchemaMigration{}); err != nil {
		return fmt.Errorf("创建 schema_migrations 表失败: %w", err)
	}

	// 2. 检查 dirty 状态
	var dirty SchemaMigration
	if err := db.Where("dirty = ?", true).First(&dirty).Error; err == nil {
		return fmt.Errorf(`数据库迁移处于 dirty 状态，version=%d
成因：之前启动时迁移版本 %d 执行到一半失败（事务回滚但 dirty 标记已持久化）
修复方式（任选其一）：
  方式 A（推荐，重试部署）：docker compose exec mysql mysql -uroot -p<root密码> <数据库名> -e "DELETE FROM schema_migrations WHERE dirty=1;"
  方式 B（彻底重置，仅首次部署可用）：docker compose down -v && docker compose up -d --build
  方式 C（强制跳过该版本，风险高）：DELETE FROM schema_migrations WHERE version=%d;
清理后重启 server 容器：docker compose restart server`, dirty.Version, dirty.Version, dirty.Version)
	}

	// 3. 扫描 .up.sql 文件
	if info, err := os.Stat(migrationsDir); err != nil {
		return fmt.Errorf("迁移目录不存在 %s: %w", migrationsDir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("迁移路径不是目录: %s", migrationsDir)
	}

	pattern := filepath.Join(migrationsDir, "*.up.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("扫描迁移文件失败 %s: %w", pattern, err)
	}
	if len(files) == 0 {
		log.Printf("[MIGRATE] 迁移目录 %s 无 .up.sql 文件，跳过", migrationsDir)
		return nil
	}

	// 4. 解析版本号并排序
	migrations, err := parseMigrations(files)
	if err != nil {
		return err
	}

	// 5. 逐个执行未应用的迁移
	applied := 0
	for _, m := range migrations {
		var existing SchemaMigration
		result := db.Where("version = ?", m.Version).First(&existing)
		if result.Error == nil {
			// 已应用
			continue
		}

		log.Printf("[MIGRATE] 应用迁移 %d: %s", m.Version, m.Name)
		if err := applyMigration(db, m); err != nil {
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
//  1. 标记该版本为 dirty
//  2. 读取 SQL 文件内容
//  3. 在事务中执行 SQL
//  4. 事务成功 → 标记为已完成（dirty=false）
//  5. 事务失败 → 保留 dirty=true，返回错误
func applyMigration(db *gorm.DB, m Migration) error {
	// 1. 标记 dirty
	if err := db.Create(&SchemaMigration{
		Version:   m.Version,
		AppliedAt: time.Now(),
		Dirty:     true,
	}).Error; err != nil {
		return fmt.Errorf("写入 schema_migrations 失败: %w", err)
	}

	// 2. 读取 SQL 文件
	content, err := os.ReadFile(m.UpFile)
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %w", err)
	}

	// 3. 事务执行
	txErr := db.Transaction(func(tx *gorm.DB) error {
		// db.Exec 支持多语句（DSN 已配置 multiStatements=true）
		if err := tx.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("执行 SQL 失败: %w", err)
		}
		return nil
	})

	if txErr != nil {
		// 事务回滚，但 schema_migrations 的 dirty 记录需要保留（在事务外）
		// 由于 schema_migrations.Create 不在事务内，dirty 记录已持久化
		return txErr
	}

	// 4. 标记完成
	if err := db.Model(&SchemaMigration{}).
		Where("version = ?", m.Version).
		Updates(map[string]interface{}{
			"dirty":      false,
			"applied_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("更新 schema_migrations 状态失败: %w", err)
	}

	return nil
}
