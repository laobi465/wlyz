// Package config 负责配置加载与依赖容器初始化
// 严格遵循铁律 04/05：所有可变参数从环境变量 / 配置文件 / sys_config 读取，禁止硬编码
package config

import (
	"fmt"
	"os"
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
	Port string `yaml:"port"` // 监听端口
	Mode string `yaml:"mode"` // debug / release / test
}

type MySQLConfig struct {
	Host         string `yaml:"host"`
	Port         string `yaml:"port"`
	Database     string `yaml:"database"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
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
		App:       AppConfig{Port: "8080", Mode: "debug"},
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
	Redis       *redis.Client
	Crypto      *crypto.Manager
	Config      *Config
	configCache *ConfigCache // sys_config 缓存（铁律 05）
}

var (
	containerOnce sync.Once
	containerInst *Container
)

// InitContainer 初始化依赖容器
func InitContainer(cfg *Config) (*Container, error) {
	// 1. MySQL
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

	// 2. Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

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
		Redis:       rdb,
		Crypto:      cryptoMgr,
		Config:      cfg,
		configCache: configCache,
	}
	return containerInst, nil
}

// Close 释放依赖
func (c *Container) Close() {
	if c.DB != nil {
		if sqlDB, err := c.DB.DB(); err == nil {
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
