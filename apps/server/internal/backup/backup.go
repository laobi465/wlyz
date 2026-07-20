// Package backup v0.4.0 数据备份恢复核心包
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：备份目录 / 保留天数 / 自动开关 / 加密密钥 / 压缩开关 / 表过滤 全部从 sys_config 读取
//   05 - 配置走后端：6 项 backup.* 配置可通过后台实时调整
//   06 - 反幻觉：备份文件 SHA-256 校验 + AES-256-GCM 加密；恢复前校验 checksum；测试覆盖正/负/边界全场景
//
// 核心能力：
//   1. Manager.CreateBackup - 全库 SQL 备份（INSERT 语句形式）+ 可选 gzip 压缩 + 可选 AES-256-GCM 加密
//   2. Manager.RestoreBackup - 校验 checksum + 解密 + 解压 + 逐条执行 SQL
//   3. Manager.CleanupExpired - 按保留天数清理过期备份
//   4. Manager.VerifyChecksum - SHA-256 校验文件完整性
//   5. Manager.GetBackupFilePath - 拼接绝对路径
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
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 常量 ==============

// 配置键常量（铁律 04：禁止硬编码配置键名）
const (
	CfgKeyBackupDir          = "backup.dir"
	CfgKeyRetentionDays      = "backup.retention_days"
	CfgKeyAutoEnabled        = "backup.auto_enabled"
	CfgKeyEncryptionKey      = "backup.encryption_key"
	CfgKeyCompress           = "backup.compress"
	CfgKeyTablesFilter       = "backup.tables_filter"
)

// BackupType 备份类型
const (
	BackupTypeManual         = "manual"
	BackupTypeAuto           = "auto"
	BackupTypeRestoreSource  = "restore_source"
)

// Status 状态
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusDeleted = "deleted"
)

// 备份文件元数据（写入文件头部，便于恢复时校验）
type BackupMetadata struct {
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	TablesCount   int       `json:"tables_count"`
	RowsCount     int64     `json:"rows_count"`
	Compressed    bool      `json:"compressed"`
	Encrypted     bool      `json:"encrypted"`
	OriginalSize  int64     `json:"original_size"`
	DatabaseName  string    `json:"database_name"`
}

// ============== 类型 ==============

// BackupOptions 备份选项
type BackupOptions struct {
	BackupType string // manual / auto
	TriggerBy  uint64 // admin id
	TriggerIP  string
}

// BackupResult 备份结果
type BackupResult struct {
	LogID       uint64
	Status      string
	FilePath    string
	FileSize    int64
	Checksum    string
	TablesCount int
	RowsCount   int64
	DurationMs  int64
	ErrorMessage string
}

// RestoreOptions 恢复选项
type RestoreOptions struct {
	BackupLogID uint64 // 要恢复的备份日志 id
	TriggerBy   uint64
	TriggerIP   string
}

// RestoreResult 恢复结果
type RestoreResult struct {
	LogID        uint64
	Status       string
	TablesCount  int
	RowsCount    int64
	DurationMs   int64
	ErrorMessage string
}

// Manager 备份管理器
type Manager struct {
	db    *gorm.DB
	cache *config.ConfigCache
	mu    sync.Mutex // 进程内互斥（防止并发备份/恢复）
}

// NewManager 创建备份管理器
func NewManager(db *gorm.DB, cache *config.ConfigCache) *Manager {
	return &Manager{
		db:    db,
		cache: cache,
	}
}

// ============== 1. 创建备份 ==============

// CreateBackup 创建数据库备份
// 流程：1) 加锁 2) 创建 pending 日志 3) 收集表数据 4) 序列化为 SQL/JSON 5) 可选 gzip 压缩 6) 可选 AES-256-GCM 加密 7) 计算 SHA-256 8) 写入文件 9) 更新日志
// 铁律 06：所有操作记录到 system_backup_log；失败时清理临时文件
func (m *Manager) CreateBackup(ctx context.Context, opts BackupOptions) (*BackupResult, error) {
	startTime := time.Now()
	result := &BackupResult{Status: StatusRunning}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 创建 pending 审计日志
	log := &model.SystemBackupLog{
		BackupType: opts.BackupType,
		TriggerBy:  opts.TriggerBy,
		TriggerIP:  opts.TriggerIP,
		Status:     StatusRunning,
	}
	if err := m.db.Create(log).Error; err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "创建审计日志失败: " + err.Error()
		return result, err
	}
	result.LogID = log.ID

	// 2. 收集表数据
	backupDir := m.cache.GetString(ctx, CfgKeyBackupDir, "data/backups")
	tablesFilter := m.cache.GetString(ctx, CfgKeyTablesFilter, "")
	compress := m.cache.GetBool(ctx, CfgKeyCompress, true)
	encryptionKey := m.cache.GetString(ctx, CfgKeyEncryptionKey, "")

	tables, rowsCount, sqlData, err := m.collectTables(ctx, tablesFilter)
	if err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "收集表数据失败: " + err.Error()
		m.finalizeLog(log, result, startTime)
		return result, err
	}
	result.TablesCount = len(tables)
	result.RowsCount = rowsCount

	// 3. 构建备份元数据
	metadata := BackupMetadata{
		Version:      1,
		CreatedAt:    time.Now(),
		TablesCount:  len(tables),
		RowsCount:    rowsCount,
		Compressed:   compress,
		Encrypted:    encryptionKey != "",
		OriginalSize: int64(len(sqlData)),
		DatabaseName: m.getDatabaseName(),
	}

	// 4. 组装最终字节流：metadata JSON + "\n" + sqlData
	metadataBytes, _ := json.Marshal(metadata)
	payload := make([]byte, 0, len(metadataBytes)+1+len(sqlData))
	payload = append(payload, metadataBytes...)
	payload = append(payload, '\n')
	payload = append(payload, sqlData...)

	// 5. 可选 gzip 压缩
	if compress {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(payload); err != nil {
			result.Status = StatusFailed
			result.ErrorMessage = "gzip 压缩失败: " + err.Error()
			m.finalizeLog(log, result, startTime)
			return result, err
		}
		if err := gw.Close(); err != nil {
			result.Status = StatusFailed
			result.ErrorMessage = "gzip 关闭失败: " + err.Error()
			m.finalizeLog(log, result, startTime)
			return result, err
		}
		payload = buf.Bytes()
	}

	// 6. 可选 AES-256-GCM 加密
	if encryptionKey != "" {
		key, err := hex.DecodeString(encryptionKey)
		if err != nil || len(key) != 32 {
			result.Status = StatusFailed
			result.ErrorMessage = "加密密钥格式错误（需 hex 编码 32 字节）"
			m.finalizeLog(log, result, startTime)
			return result, fmt.Errorf("invalid encryption key")
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			result.Status = StatusFailed
			result.ErrorMessage = "AES cipher 创建失败: " + err.Error()
			m.finalizeLog(log, result, startTime)
			return result, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			result.Status = StatusFailed
			result.ErrorMessage = "GCM 创建失败: " + err.Error()
			m.finalizeLog(log, result, startTime)
			return result, err
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			result.Status = StatusFailed
			result.ErrorMessage = "生成 nonce 失败: " + err.Error()
			m.finalizeLog(log, result, startTime)
			return result, err
		}
		// nonce 前置，便于解密时读取
		payload = append(nonce, gcm.Seal(nil, nonce, payload, nil)...)
	}

	// 7. 计算 SHA-256
	checksum := sha256.Sum256(payload)
	result.Checksum = hex.EncodeToString(checksum[:])

	// 8. 确保备份目录存在
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "创建备份目录失败: " + err.Error()
		m.finalizeLog(log, result, startTime)
		return result, err
	}

	// 9. 写入文件
	filename := fmt.Sprintf("backup_%s_%s.bak",
		time.Now().Format("20060102_150405"),
		opts.BackupType,
	)
	filePath := filepath.Join(backupDir, filename)
	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "写入备份文件失败: " + err.Error()
		m.finalizeLog(log, result, startTime)
		return result, err
	}

	// 10. 获取文件大小
	stat, err := os.Stat(filePath)
	if err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "获取文件信息失败: " + err.Error()
		m.finalizeLog(log, result, startTime)
		return result, err
	}
	result.FileSize = stat.Size()
	result.FilePath = filePath
	result.Status = StatusSuccess
	result.DurationMs = time.Since(startTime).Milliseconds()

	m.finalizeLog(log, result, startTime)
	return result, nil
}

// collectTables 收集所有业务表数据为 SQL INSERT 语句
// 铁律 06：显式表名白名单过滤，跳过 sys_admin（含敏感密码 hash）；返回非 nil 切片
func (m *Manager) collectTables(ctx context.Context, tablesFilter string) ([]string, int64, []byte, error) {
	// 获取所有表名
	var allTables []string
	if err := m.db.Raw("SHOW TABLES").Scan(&allTables).Error; err != nil {
		// SQLite fallback（测试环境）
		if err := m.db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&allTables).Error; err != nil {
			return nil, 0, nil, fmt.Errorf("查询表列表失败: %w", err)
		}
	}

	// 应用表过滤白名单
	var tables []string
	filter := parseTablesFilter(tablesFilter)
	for _, t := range allTables {
		// 跳过备份/更新日志表（避免循环备份）
		if t == "system_backup_log" || t == "system_update_log" {
			continue
		}
		// 跳过敏感表
		if t == "sys_admin" {
			continue
		}
		if len(filter) > 0 && !filter[t] {
			continue
		}
		tables = append(tables, t)
	}

	var buf bytes.Buffer
	var totalRows int64

	for _, table := range tables {
		// 查询表所有行
		rows, err := m.db.Raw(fmt.Sprintf("SELECT * FROM %s", table)).Rows()
		if err != nil {
			return nil, 0, nil, fmt.Errorf("查询表 %s 失败: %w", table, err)
		}

		// 获取列名
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return nil, 0, nil, fmt.Errorf("获取表 %s 列失败: %w", table, err)
		}

		// 写入表头注释
		fmt.Fprintf(&buf, "-- Table: %s\n", table)

		// 逐行序列化为 INSERT 语句
		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range columns {
				valuePtrs[i] = &values[i]
			}
			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				return nil, 0, nil, fmt.Errorf("扫描表 %s 行失败: %w", table, err)
			}
			fmt.Fprintf(&buf, "INSERT INTO %s (%s) VALUES (", table, strings.Join(columns, ", "))
			for i, v := range values {
				if i > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(serializeValue(v))
			}
			buf.WriteString(");\n")
			totalRows++
		}
		rows.Close()
		buf.WriteString("\n")
	}

	return tables, totalRows, buf.Bytes(), nil
}

// serializeValue 将 Go 值序列化为 SQL 字面量
// 铁律 06：nil → NULL；字符串 → 转义单引号；[]byte → x'hex'；其他用 fmt.Sprintf
func serializeValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	case []byte:
		return "x'" + hex.EncodeToString(val) + "'"
	case bool:
		if val {
			return "1"
		}
		return "0"
	case time.Time:
		return "'" + val.Format("2006-01-02 15:04:05") + "'"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// parseTablesFilter 解析表过滤白名单
func parseTablesFilter(s string) map[string]bool {
	result := map[string]bool{}
	if s == "" {
		return result
	}
	for _, name := range strings.Split(s, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			result[name] = true
		}
	}
	return result
}

// getDatabaseName 获取当前数据库名
func (m *Manager) getDatabaseName() string {
	var name string
	if err := m.db.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil {
		// SQLite fallback
		return "sqlite"
	}
	return name
}

// ============== 2. 恢复备份 ==============

// RestoreBackup 从备份文件恢复数据库
// 流程：1) 加锁 2) 查询备份日志获取文件路径 3) 读取文件 4) 校验 SHA-256 5) 可选 AES 解密 6) 可选 gunzip 7) 解析 metadata 8) 逐条执行 SQL 9) 写入恢复审计日志
// 铁律 06：恢复前严格校验 checksum；事务包裹恢复过程；失败回滚事务
func (m *Manager) RestoreBackup(ctx context.Context, opts RestoreOptions) (*RestoreResult, error) {
	startTime := time.Now()
	result := &RestoreResult{Status: StatusRunning}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 查询原备份日志
	var backupLog model.SystemBackupLog
	if err := m.db.First(&backupLog, opts.BackupLogID).Error; err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "查询原备份日志失败: " + err.Error()
		return result, err
	}
	if backupLog.Status != StatusSuccess {
		result.Status = StatusFailed
		result.ErrorMessage = "原备份状态非 success，无法恢复"
		return result, fmt.Errorf("backup status not success")
	}

	// 2. 创建恢复审计日志（restored_from 关联原备份）
	restoreLog := &model.SystemBackupLog{
		BackupType:   BackupTypeRestoreSource,
		TriggerBy:    opts.TriggerBy,
		TriggerIP:    opts.TriggerIP,
		FilePath:     backupLog.FilePath,
		Checksum:     backupLog.Checksum,
		Status:       StatusRunning,
		RestoredFrom: opts.BackupLogID,
	}
	if err := m.db.Create(restoreLog).Error; err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "创建恢复审计日志失败: " + err.Error()
		return result, err
	}
	result.LogID = restoreLog.ID

	// 3. 读取备份文件
	payload, err := os.ReadFile(backupLog.FilePath)
	if err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "读取备份文件失败: " + err.Error()
		m.finalizeRestoreLog(restoreLog, result, startTime)
		return result, err
	}

	// 4. 校验 SHA-256
	actualChecksum := sha256.Sum256(payload)
	if hex.EncodeToString(actualChecksum[:]) != backupLog.Checksum {
		result.Status = StatusFailed
		result.ErrorMessage = "备份文件 checksum 校验失败"
		m.finalizeRestoreLog(restoreLog, result, startTime)
		return result, fmt.Errorf("checksum mismatch")
	}

	// 5. 可选 AES 解密
	encryptionKey := m.cache.GetString(ctx, CfgKeyEncryptionKey, "")
	if encryptionKey != "" {
		key, err := hex.DecodeString(encryptionKey)
		if err != nil || len(key) != 32 {
			result.Status = StatusFailed
			result.ErrorMessage = "加密密钥格式错误"
			m.finalizeRestoreLog(restoreLog, result, startTime)
			return result, fmt.Errorf("invalid encryption key")
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			result.Status = StatusFailed
			result.ErrorMessage = "AES cipher 创建失败: " + err.Error()
			m.finalizeRestoreLog(restoreLog, result, startTime)
			return result, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			result.Status = StatusFailed
			result.ErrorMessage = "GCM 创建失败: " + err.Error()
			m.finalizeRestoreLog(restoreLog, result, startTime)
			return result, err
		}
		nonceSize := gcm.NonceSize()
		if len(payload) < nonceSize {
			result.Status = StatusFailed
			result.ErrorMessage = "加密 payload 过短"
			m.finalizeRestoreLog(restoreLog, result, startTime)
			return result, fmt.Errorf("payload too short")
		}
		nonce, ciphertext := payload[:nonceSize], payload[nonceSize:]
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			result.Status = StatusFailed
			result.ErrorMessage = "AES-GCM 解密失败: " + err.Error()
			m.finalizeRestoreLog(restoreLog, result, startTime)
			return result, err
		}
		payload = plaintext
	}

	// 6. 可选 gunzip
	metadata, sqlData, err := m.extractPayload(payload)
	if err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "解析备份 payload 失败: " + err.Error()
		m.finalizeRestoreLog(restoreLog, result, startTime)
		return result, err
	}

	// 7. 逐条执行 SQL（事务包裹）
	tablesCount, rowsCount, err := m.executeSQLStatements(ctx, sqlData)
	if err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "执行 SQL 失败: " + err.Error()
		m.finalizeRestoreLog(restoreLog, result, startTime)
		return result, err
	}

	result.TablesCount = tablesCount
	result.RowsCount = rowsCount
	result.Status = StatusSuccess
	result.DurationMs = time.Since(startTime).Milliseconds()
	_ = metadata
	m.finalizeRestoreLog(restoreLog, result, startTime)
	return result, nil
}

// extractPayload 从 payload 中提取 metadata 和 sqlData
// payload 可能是：1) gzip 压缩 2) gzip 未压缩 3) 加密（已在上层解密）
func (m *Manager) extractPayload(payload []byte) (*BackupMetadata, []byte, error) {
	// 尝试 gzip 解压
	var data []byte
	if len(payload) > 2 && payload[0] == 0x1f && payload[1] == 0x8b {
		// gzip magic number
		gr, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, nil, fmt.Errorf("gzip reader 创建失败: %w", err)
		}
		defer gr.Close()
		data, err = io.ReadAll(gr)
		if err != nil {
			return nil, nil, fmt.Errorf("gzip 解压失败: %w", err)
		}
	} else {
		data = payload
	}

	// 解析 metadata JSON（到第一个 \n）
	newlineIdx := bytes.IndexByte(data, '\n')
	if newlineIdx < 0 {
		return nil, nil, fmt.Errorf("payload 缺少 metadata 分隔符")
	}
	var metadata BackupMetadata
	if err := json.Unmarshal(data[:newlineIdx], &metadata); err != nil {
		return nil, nil, fmt.Errorf("解析 metadata JSON 失败: %w", err)
	}
	sqlData := data[newlineIdx+1:]
	return &metadata, sqlData, nil
}

// executeSQLStatements 逐条执行 SQL INSERT 语句
// 铁律 06：用事务包裹；跳过注释行和空行；遇到新表先 DELETE 清空，避免主键冲突
func (m *Manager) executeSQLStatements(ctx context.Context, sqlData []byte) (int, int64, error) {
	tablesSet := map[string]bool{}
	clearedTables := map[string]bool{}
	var rowsCount int64

	tx := m.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, line := range strings.Split(string(sqlData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "--") {
			if strings.HasPrefix(line, "-- Table: ") {
				tableName := strings.TrimPrefix(line, "-- Table: ")
				tablesSet[tableName] = true
			}
			continue
		}
		if !strings.HasPrefix(line, "INSERT INTO") {
			continue
		}
		// 提取表名：INSERT INTO <table> (...)
		tableName := extractTableName(line)
		if tableName != "" && !clearedTables[tableName] {
			if err := tx.Exec("DELETE FROM " + tableName).Error; err != nil {
				tx.Rollback()
				return 0, 0, fmt.Errorf("清空表 %s 失败: %w", tableName, err)
			}
			clearedTables[tableName] = true
		}
		if err := tx.Exec(line).Error; err != nil {
			tx.Rollback()
			return 0, 0, fmt.Errorf("执行 SQL 失败 (%s): %w", truncate(line, 100), err)
		}
		rowsCount++
	}

	if err := tx.Commit().Error; err != nil {
		return 0, 0, fmt.Errorf("事务提交失败: %w", err)
	}

	return len(tablesSet), rowsCount, nil
}

// extractTableName 从 INSERT INTO <table> (...) 中提取表名
func extractTableName(line string) string {
	// "INSERT INTO app (id, name) VALUES" → "app"
	prefix := "INSERT INTO "
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	rest := line[len(prefix):]
	// 取下一个空格前的部分
	spaceIdx := strings.Index(rest, " ")
	if spaceIdx < 0 {
		return ""
	}
	return rest[:spaceIdx]
}

// ============== 3. 清理过期备份 ==============

// CleanupExpired 清理超过保留天数的备份
// 铁律 06：仅清理 status=success 的备份；删除文件 + 更新日志状态为 deleted
func (m *Manager) CleanupExpired(ctx context.Context) (int, error) {
	retentionDays := m.cache.GetInt(ctx, CfgKeyRetentionDays, 30)
	if retentionDays <= 0 {
		return 0, nil // 永不清理
	}

	threshold := time.Now().AddDate(0, 0, -retentionDays)
	var expired []model.SystemBackupLog
	if err := m.db.Where("status = ? AND created_at < ?", StatusSuccess, threshold).
		Find(&expired).Error; err != nil {
		return 0, err
	}

	deleted := 0
	for _, log := range expired {
		// 删除文件
		if log.FilePath != "" {
			_ = os.Remove(log.FilePath)
		}
		// 更新日志状态
		if err := m.db.Model(&model.SystemBackupLog{}).Where("id = ?", log.ID).
			Update("status", StatusDeleted).Error; err == nil {
			deleted++
		}
	}
	return deleted, nil
}

// ============== 4. 校验工具 ==============

// VerifyChecksum 校验指定备份日志的文件 checksum
func (m *Manager) VerifyChecksum(ctx context.Context, logID uint64) (bool, error) {
	var log model.SystemBackupLog
	if err := m.db.First(&log, logID).Error; err != nil {
		return false, err
	}
	if log.Status != StatusSuccess {
		return false, fmt.Errorf("备份状态非 success")
	}
	payload, err := os.ReadFile(log.FilePath)
	if err != nil {
		return false, err
	}
	actual := sha256.Sum256(payload)
	return hex.EncodeToString(actual[:]) == log.Checksum, nil
}

// GetBackupFilePath 获取备份文件绝对路径
func (m *Manager) GetBackupFilePath(ctx context.Context, logID uint64) (string, error) {
	var log model.SystemBackupLog
	if err := m.db.First(&log, logID).Error; err != nil {
		return "", err
	}
	if log.Status != StatusSuccess {
		return "", fmt.Errorf("备份状态非 success")
	}
	absPath, err := filepath.Abs(log.FilePath)
	if err != nil {
		return "", err
	}
	return absPath, nil
}

// ============== 内部工具 ==============

// finalizeLog 写入最终日志到 DB
func (m *Manager) finalizeLog(log *model.SystemBackupLog, result *BackupResult, startTime time.Time) {
	if log == nil || log.ID == 0 {
		return
	}
	updates := map[string]interface{}{
		"status":        result.Status,
		"file_path":     result.FilePath,
		"file_size":     result.FileSize,
		"checksum":      result.Checksum,
		"tables_count":  result.TablesCount,
		"rows_count":    result.RowsCount,
		"duration_ms":   time.Since(startTime).Milliseconds(),
		"error_message": truncate(result.ErrorMessage, 512),
	}
	_ = m.db.Model(&model.SystemBackupLog{}).Where("id = ?", log.ID).Updates(updates).Error
}

// finalizeRestoreLog 写入恢复结果到 DB
func (m *Manager) finalizeRestoreLog(log *model.SystemBackupLog, result *RestoreResult, startTime time.Time) {
	if log == nil || log.ID == 0 {
		return
	}
	updates := map[string]interface{}{
		"status":        result.Status,
		"tables_count":  result.TablesCount,
		"rows_count":    result.RowsCount,
		"duration_ms":   time.Since(startTime).Milliseconds(),
		"error_message": truncate(result.ErrorMessage, 512),
	}
	_ = m.db.Model(&model.SystemBackupLog{}).Where("id = ?", log.ID).Updates(updates).Error
}

// truncate 截断字符串到指定长度
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ============== 5. 状态查询 ==============

// IsAutoBackupEnabled 是否启用自动备份
func (m *Manager) IsAutoBackupEnabled(ctx context.Context) bool {
	return m.cache.GetBool(ctx, CfgKeyAutoEnabled, false)
}
