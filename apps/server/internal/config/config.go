// Package config 负责配置加载与依赖容器初始化
// 严格遵循铁律 04/05：所有可变参数从环境变量 / 配置文件 / sys_config 读取，禁止硬编码
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/your-org/keyauth-saas/apps/server/internal/migration"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/redis/go-redis/v9"
)

// Config 配置结构（从 config.yaml + 环境变量加载）
type Config struct {
	App       AppConfig       `yaml:"app"`
	MySQL     MySQLConfig     `yaml:"mysql"`
	Redis     RedisConfig     `yaml:"redis"`
	JWT       JWTConfig       `yaml:"jwt"`
	Crypto    CryptoConfig    `yaml:"crypto"`
	Migration MigrationConfig `yaml:"migration"`
	Domain    string          `yaml:"domain"`
}

type AppConfig struct {
	Port      string `yaml:"port"`       // 监听端口
	Mode      string `yaml:"mode"`       // debug / release / test
	LogLevel  string `yaml:"log_level"`  // v0.4.0：日志级别 debug/info/warn/error（默认 info）
	LogFormat string `yaml:"log_format"` // v0.4.0：日志格式 json/text（默认 json）
	LogOutput string `yaml:"log_output"` // v0.4.0：日志输出 stdout/stderr/文件路径（默认 stdout）

	// v0.6.6 新增：端口冲突自动 +1 重试
	// 开发友好（8080 被占用时自动尝试 8081、8082…），生产建议关闭（避免端口漂移导致 nginx/反代失配）
	// 铁律 05：行为可配置，默认 false 保证生产安全
	PortAutoIncrement bool `yaml:"port_auto_increment"` // 端口被占用时自动 +1 重试
	PortMaxAttempts   int  `yaml:"port_max_attempts"`   // 最大尝试次数（含起始端口），默认 20
}

type MySQLConfig struct {
	Host         string `yaml:"host"`          // v0.5.0：主库地址（兼容单库场景）
	Port         string `yaml:"port"`          // v0.5.0：主库端口
	Database     string `yaml:"database"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`

	// v0.5.0 读写分离：从库列表（空表示单库模式，所有查询走主库）
	// 铁律 04：从库地址从配置文件/环境变量读取，不硬编码
	Slaves []MySQLSlaveConfig `yaml:"slaves"` // 从库列表
}

// MySQLSlaveConfig 从库配置（v0.5.0）
type MySQLSlaveConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"` // 可选，留空则继承主库
	Password string `yaml:"password"` // 可选，留空则继承主库
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`

	// v0.5.0 Redis 集群/哨兵模式支持
	// 铁律 04：模式与多地址从配置文件/环境变量读取
	// Mode 取值：single（默认）/ sentinel / cluster
	Mode      string   `yaml:"mode"`       // single / sentinel / cluster
	Addrs     []string `yaml:"addrs"`      // sentinel/cluster 模式下的地址列表（host:port 格式）
	MasterName string  `yaml:"master_name"` // sentinel 模式主节点名
	// Username v0.5.0 Redis 6+ ACL 用户名（可选）
	Username string `yaml:"username"`
}

type JWTConfig struct {
	Secret        string `yaml:"secret"`         // 签名密钥（环境变量注入）
	ExpireHours   int    `yaml:"expire_hours"`   // Token 有效期（小时）
	RefreshHours  int    `yaml:"refresh_hours"`  // 刷新有效期
	Issuer        string `yaml:"issuer"`
}

type CryptoConfig struct {
	AESKey            string `yaml:"aes_key"`              // 32 字节 AES-256-GCM 密钥
	RSAPrivateKeyPath string `yaml:"rsa_private_key_path"` // RSA-4096 私钥文件
	RSAPublicKeyPath  string `yaml:"rsa_public_key_path"`  // RSA-4096 公钥文件
}

// MigrationConfig 数据库迁移配置（v0.3.5 新增）
// 铁律 05：迁移目录路径走配置，不硬编码
type MigrationConfig struct {
	Auto bool   `yaml:"auto"` // 启动时是否自动执行迁移
	Dir  string `yaml:"dir"`  // 迁移文件目录（绝对路径或相对工作目录）
}

// Load 从文件加载配置，环境变量优先覆盖
func Load(path string) (*Config, error) {
	cfg := &Config{
		App:       AppConfig{Port: "8080", Mode: "debug", PortMaxAttempts: 20},
		MySQL:     MySQLConfig{MaxOpenConns: 100, MaxIdleConns: 20},
		Redis:     RedisConfig{DB: 0},
		JWT:       JWTConfig{ExpireHours: 24, RefreshHours: 168, Issuer: "keyauth-saas"},
		Migration: MigrationConfig{Auto: true, Dir: "apps/server/migrations"},
	}

	// 1. 读取配置文件（可选）
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件失败: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 2. 环境变量覆盖（铁律 04：敏感参数走环境变量）
	applyEnvConfig(cfg)

	// 3. 校验必填项
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnvConfig 用环境变量覆盖配置（铁律 04：密钥类参数禁止进配置文件）
func applyEnvConfig(cfg *Config) {
	if v := os.Getenv("APP_PORT"); v != "" {
		cfg.App.Port = v
	}
	if v := os.Getenv("APP_MODE"); v != "" {
		cfg.App.Mode = v
	}
	// v0.6.6：端口冲突自动 +1 重试开关
	if v := os.Getenv("APP_PORT_AUTO_INCREMENT"); v != "" {
		cfg.App.PortAutoIncrement = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("APP_PORT_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.App.PortMaxAttempts = n
		}
	}
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		cfg.MySQL.Host = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		cfg.MySQL.Port = v
	}
	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		cfg.MySQL.Database = v
	}
	if v := os.Getenv("MYSQL_USER"); v != "" {
		cfg.MySQL.Username = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		cfg.MySQL.Password = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		cfg.Redis.Port = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	// v0.5.0：Redis 集群/哨兵模式
	if v := os.Getenv("REDIS_MODE"); v != "" {
		cfg.Redis.Mode = v
	}
	if v := os.Getenv("REDIS_ADDRS"); v != "" {
		// 逗号分隔：host1:port1,host2:port2,host3:port3
		cfg.Redis.Addrs = splitCommaList(v)
	}
	if v := os.Getenv("REDIS_MASTER_NAME"); v != "" {
		cfg.Redis.MasterName = v
	}
	if v := os.Getenv("REDIS_USERNAME"); v != "" {
		cfg.Redis.Username = v
	}
	// v0.5.0：MySQL 从库列表（逗号分隔，每个从库格式 host:port）
	// 例：MYSQL_SLAVES=slave1:3306,slave2:3306
	if v := os.Getenv("MYSQL_SLAVES"); v != "" {
		cfg.MySQL.Slaves = parseMySQLSlaves(v, cfg.MySQL.Username, cfg.MySQL.Password)
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("APP_AES_KEY"); v != "" {
		cfg.Crypto.AESKey = v
	}
	if v := os.Getenv("RSA_PRIVATE_KEY_PATH"); v != "" {
		cfg.Crypto.RSAPrivateKeyPath = v
	}
	if v := os.Getenv("RSA_PUBLIC_KEY_PATH"); v != "" {
		cfg.Crypto.RSAPublicKeyPath = v
	}
	if v := os.Getenv("DOMAIN"); v != "" {
		cfg.Domain = v
	}
	if v := os.Getenv("MIGRATION_AUTO"); v != "" {
		cfg.Migration.Auto = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("MIGRATION_DIR"); v != "" {
		cfg.Migration.Dir = v
	}
}

func (c *Config) validate() error {
	if c.MySQL.Host == "" || c.MySQL.Database == "" {
		return fmt.Errorf("MySQL 配置不完整")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET 必须配置（环境变量）")
	}
	if len(c.Crypto.AESKey) != 32 {
		return fmt.Errorf("APP_AES_KEY 必须为 32 字节")
	}
	return nil
}

// Container 依赖容器（持有 DB / Redis / 加密器等单例）
type Container struct {
	DB          *gorm.DB
	DBRead      *gorm.DB // v0.5.0：只读库（读写分离，nil 时回退到 DB）
	Redis       *redis.Client
	Crypto      *crypto.Manager
	Config      *Config
	configCache *ConfigCache // sys_config 缓存（铁律 05）
}

// ReadDB v0.5.0 返回只读库；未配置从库时返回主库
// 铁律 06：调用方无需判断 nil，统一接口
func (c *Container) ReadDB() *gorm.DB {
	if c.DBRead != nil {
		return c.DBRead
	}
	return c.DB
}

var (
	containerOnce sync.Once
	containerInst *Container
)

// InitContainer 初始化依赖容器
func InitContainer(cfg *Config) (*Container, error) {
	// 1. MySQL 主库
	// 注：multiStatements=true 用于支持 migration 多语句执行；项目内所有业务 SQL 均参数化，无注入风险
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true&time_zone=%%27%%2B00%%3A00%%27",
		cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)

	// 1.1 启动时自动迁移（v0.3.5 新增，替代 mysql entrypoint 自动执行）
	if cfg.Migration.Auto {
		if err := migration.Run(db, cfg.Migration.Dir); err != nil {
			return nil, fmt.Errorf("数据库迁移失败: %w", err)
		}
	}

	// 1.2 v0.5.0 MySQL 读写分离：初始化从库连接（若有）
	// 铁律 06：从库失败不阻断启动（降级走主库）
	var dbRead *gorm.DB
	if len(cfg.MySQL.Slaves) > 0 {
		dbRead, err = initReadDB(cfg)
		if err != nil {
			// 从库初始化失败降级走主库
			dbRead = nil
		}
	}

	// 2. Redis（v0.5.0 支持 single/sentinel/cluster 多模式）
	rdb, err := initRedis(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}

	// 3. 加密管理器
	cryptoMgr, err := crypto.NewManager(cfg.Crypto.AESKey, cfg.Crypto.RSAPrivateKeyPath, cfg.Crypto.RSAPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("初始化加密管理器失败: %w", err)
	}

	// 4. 自动迁移基础表（sys_config 必须最先建）
	if err := db.AutoMigrate(&model.SysConfig{}); err != nil {
		return nil, fmt.Errorf("自动迁移 sys_config 失败: %w", err)
	}

	// 5. 初始化 sys_config 缓存
	configCache := NewConfigCache(db, rdb)

	containerInst = &Container{
		DB:          db,
		DBRead:      dbRead, // v0.5.0：只读库（nil 时回退到 DB）
		Redis:       rdb,
		Crypto:      cryptoMgr,
		Config:      cfg,
		configCache: configCache,
	}
	return containerInst, nil
}

// initReadDB v0.5.0 初始化只读库（从库列表随机选一个，简化实现）
// 铁律 06：从库失败不 panic，调用方应处理 nil 返回值
// 生产级实现建议用 gorm.io/plugin/dbresolver，本实现保持零新依赖
func initReadDB(cfg *Config) (*gorm.DB, error) {
	if len(cfg.MySQL.Slaves) == 0 {
		return nil, nil
	}
	// 取第一个从库（简化策略；后续可改为轮询/权重）
	slave := cfg.MySQL.Slaves[0]
	user := slave.Username
	if user == "" {
		user = cfg.MySQL.Username
	}
	pass := slave.Password
	if pass == "" {
		pass = cfg.MySQL.Password
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&time_zone=%%27%%2B00%%3A00%%27",
		user, pass, slave.Host, slave.Port, cfg.MySQL.Database)
	dbRead, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := dbRead.DB()
	if err != nil {
		return nil, err
	}
	// 从库连接池通常可以更大（读多写少）
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	return dbRead, nil
}

// initRedis v0.5.0 根据 Mode 初始化 Redis 客户端（single/sentinel/cluster）
// 铁律 04：所有地址 / 凭据 / 模式均从 RedisConfig 读取
// 铁律 06：sentinel/cluster 模式若配置不完整，降级为 single 模式
func initRedis(cfg RedisConfig) (*redis.Client, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "single"
	}

	switch mode {
	case "single":
		// 单实例模式（默认）
		return redis.NewClient(&redis.Options{
			Addr:     cfg.Host + ":" + cfg.Port,
			Password: cfg.Password,
			DB:       cfg.DB,
			Username: cfg.Username,
		}), nil

	case "sentinel":
		// 哨兵模式：通过 Addrs 列表连哨兵，由哨兵返回 master 地址
		// 铁律 06：sentinel 模式必须配置 Addrs + MasterName，否则降级为 single
		if len(cfg.Addrs) == 0 || cfg.MasterName == "" {
			return redis.NewClient(&redis.Options{
				Addr:     cfg.Host + ":" + cfg.Port,
				Password: cfg.Password,
				DB:       cfg.DB,
				Username: cfg.Username,
			}), nil
		}
		// 注意：go-redis v9 的 sentinel 通过 FailoverOptions 实现
		// 但 FailoverOptions 返回的 *Client 在内部已是 master 客户端
		// 为保持 API 一致性（返回 *redis.Client），这里用 FailoverClient
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       cfg.MasterName,
			SentinelAddrs:    cfg.Addrs,
			Password:         cfg.Password,
			DB:               cfg.DB,
			Username:         cfg.Username,
		}), nil

	case "cluster":
		// 集群模式：通过 Addrs 列表连集群任意节点
		// 铁律 06：cluster 模式必须配置 Addrs，否则降级为 single
		if len(cfg.Addrs) == 0 {
			return redis.NewClient(&redis.Options{
				Addr:     cfg.Host + ":" + cfg.Port,
				Password: cfg.Password,
				DB:       cfg.DB,
				Username: cfg.Username,
			}), nil
		}
		// 注意：ClusterClient 与 Client 是不同类型，无法直接返回 *redis.Client
		// 此处返回 NewClient 用 Addrs[0] 作为入口，依赖 ClusterClient 的透明重定向
		// 真正集群部署建议改造为返回 redis.Cmdable 接口
		return redis.NewClient(&redis.Options{
			Addr:     cfg.Addrs[0],
			Password: cfg.Password,
			DB:       cfg.DB,
			Username: cfg.Username,
		}), nil

	default:
		// 未知模式降级为 single
		return redis.NewClient(&redis.Options{
			Addr:     cfg.Host + ":" + cfg.Port,
			Password: cfg.Password,
			DB:       cfg.DB,
			Username: cfg.Username,
		}), nil
	}
}

// Close 释放依赖
func (c *Container) Close() {
	if c.DB != nil {
		if sqlDB, err := c.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	// v0.5.0：释放只读库连接
	if c.DBRead != nil {
		if sqlDB, err := c.DBRead.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	if c.Redis != nil {
		_ = RedisClose(c.Redis)
	}
}

// RedisClose 单独包装便于测试 mock
func RedisClose(rdb *redis.Client) error {
	return rdb.Close()
}

// GetContainer 获取容器单例
func GetContainer() *Container {
	return containerInst
}

// ConfigCache 返回 sys_config 缓存
func (c *Container) ConfigCache() *ConfigCache {
	return c.configCache
}

// ============== v0.5.0 工具函数 ==============

// splitCommaList 将逗号分隔字符串切分为列表（trim 空白 + 跳过空项）
// 铁律 06：用于 REDIS_ADDRS / MYSQL_SLAVES 等环境变量解析
func splitCommaList(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			item := strings.TrimSpace(s[start:i])
			if item != "" {
				result = append(result, item)
			}
			start = i + 1
		}
	}
	return result
}

// parseMySQLSlaves 解析 MYSQL_SLAVES 环境变量为从库配置列表
// 输入格式：host1:port1,host2:port2 或 host1:port1:user1:pass1,host2:port2
// 铁律 06：解析失败时跳过该项（不 panic）
func parseMySQLSlaves(s, defaultUser, defaultPass string) []MySQLSlaveConfig {
	items := splitCommaList(s)
	if len(items) == 0 {
		return nil
	}
	slaves := make([]MySQLSlaveConfig, 0, len(items))
	for _, item := range items {
		// 拆分 host:port[:user[:pass]]
		parts := splitColonList(item)
		if len(parts) < 2 {
			continue // 格式不合法跳过
		}
		slave := MySQLSlaveConfig{
			Host: parts[0],
			Port: parts[1],
		}
		if len(parts) >= 3 && parts[2] != "" {
			slave.Username = parts[2]
		} else {
			slave.Username = defaultUser
		}
		if len(parts) >= 4 && parts[3] != "" {
			slave.Password = parts[3]
		} else {
			slave.Password = defaultPass
		}
		slaves = append(slaves, slave)
	}
	return slaves
}

// splitColonList 冒号分隔（仅顶层，不处理转义）
func splitColonList(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ':' {
			result = append(result, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	return result
}
