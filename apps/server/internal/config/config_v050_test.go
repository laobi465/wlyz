// Package config v0.5.0 配置加载测试
// 铁律 06：覆盖读写分离配置 / Redis 多模式配置 / 工具函数
package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============== 1. splitCommaList 测试 ==============

func TestSplitCommaList_Empty(t *testing.T) {
	assert.Nil(t, splitCommaList(""))
}

func TestSplitCommaList_Single(t *testing.T) {
	result := splitCommaList("a")
	assert.Equal(t, []string{"a"}, result)
}

func TestSplitCommaList_Multiple(t *testing.T) {
	result := splitCommaList("a,b,c")
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestSplitCommaList_WithSpaces(t *testing.T) {
	result := splitCommaList(" a , b , c ")
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestSplitCommaList_TrailingComma(t *testing.T) {
	result := splitCommaList("a,b,")
	assert.Equal(t, []string{"a", "b"}, result)
}

func TestSplitCommaList_EmptyItems(t *testing.T) {
	result := splitCommaList(",,a,,b,,")
	assert.Equal(t, []string{"a", "b"}, result)
}

// ============== 2. splitColonList 测试 ==============

func TestSplitColonList_Empty(t *testing.T) {
	assert.Nil(t, splitColonList(""))
}

func TestSplitColonList_HostPort(t *testing.T) {
	result := splitColonList("host:port")
	assert.Equal(t, []string{"host", "port"}, result)
}

func TestSplitColonList_HostPortUserPass(t *testing.T) {
	result := splitColonList("host:port:user:pass")
	assert.Equal(t, []string{"host", "port", "user", "pass"}, result)
}

// ============== 3. parseMySQLSlaves 测试 ==============

func TestParseMySQLSlaves_Empty(t *testing.T) {
	result := parseMySQLSlaves("", "root", "pass")
	assert.Nil(t, result)
}

func TestParseMySQLSlaves_SingleSlave(t *testing.T) {
	result := parseMySQLSlaves("slave1:3306", "root", "pass")
	require.Len(t, result, 1)
	assert.Equal(t, "slave1", result[0].Host)
	assert.Equal(t, "3306", result[0].Port)
	assert.Equal(t, "root", result[0].Username) // 继承主库
	assert.Equal(t, "pass", result[0].Password)
}

func TestParseMySQLSlaves_MultipleSlaves(t *testing.T) {
	result := parseMySQLSlaves("slave1:3306,slave2:3307", "root", "pass")
	require.Len(t, result, 2)
	assert.Equal(t, "slave1", result[0].Host)
	assert.Equal(t, "slave2", result[1].Host)
}

func TestParseMySQLSlaves_WithCredentials(t *testing.T) {
	result := parseMySQLSlaves("slave1:3306:repl:replpass", "root", "pass")
	require.Len(t, result, 1)
	assert.Equal(t, "repl", result[0].Username)
	assert.Equal(t, "replpass", result[0].Password)
}

func TestParseMySQLSlaves_InvalidFormat(t *testing.T) {
	// 缺少 port 应被跳过
	result := parseMySQLSlaves("slave1,slave2:3306", "root", "pass")
	require.Len(t, result, 1)
	assert.Equal(t, "slave2", result[0].Host)
}

func TestParseMySQLSlaves_PartialCredentials(t *testing.T) {
	// 只提供 user 不提供 pass
	result := parseMySQLSlaves("slave1:3306:repl", "root", "pass")
	require.Len(t, result, 1)
	assert.Equal(t, "repl", result[0].Username)
	assert.Equal(t, "pass", result[0].Password) // 继承主库
}

// ============== 4. 环境变量覆盖测试 ==============

func TestApplyEnvConfig_RedisMode(t *testing.T) {
	os.Setenv("REDIS_MODE", "cluster")
	os.Setenv("REDIS_ADDRS", "h1:7000,h2:7000,h3:7000")
	os.Setenv("REDIS_MASTER_NAME", "mymaster")
	os.Setenv("REDIS_USERNAME", "default")
	defer func() {
		os.Unsetenv("REDIS_MODE")
		os.Unsetenv("REDIS_ADDRS")
		os.Unsetenv("REDIS_MASTER_NAME")
		os.Unsetenv("REDIS_USERNAME")
	}()

	cfg := &Config{}
	applyEnvConfig(cfg)

	assert.Equal(t, "cluster", cfg.Redis.Mode)
	assert.Equal(t, []string{"h1:7000", "h2:7000", "h3:7000"}, cfg.Redis.Addrs)
	assert.Equal(t, "mymaster", cfg.Redis.MasterName)
	assert.Equal(t, "default", cfg.Redis.Username)
}

func TestApplyEnvConfig_MySQLSlaves(t *testing.T) {
	os.Setenv("MYSQL_SLAVES", "slave1:3306,slave2:3306")
	defer os.Unsetenv("MYSQL_SLAVES")

	cfg := &Config{
		MySQL: MySQLConfig{
			Username: "root",
			Password: "pass",
		},
	}
	applyEnvConfig(cfg)

	require.Len(t, cfg.MySQL.Slaves, 2)
	assert.Equal(t, "slave1", cfg.MySQL.Slaves[0].Host)
	assert.Equal(t, "3306", cfg.MySQL.Slaves[0].Port)
	assert.Equal(t, "root", cfg.MySQL.Slaves[0].Username) // 继承主库
}

// ============== 5. RedisConfig 默认值测试 ==============

func TestRedisConfig_Defaults(t *testing.T) {
	cfg := RedisConfig{}
	// 默认 Mode 应为空（initRedis 中转为 "single"）
	assert.Equal(t, "", cfg.Mode)
	assert.Nil(t, cfg.Addrs)
	assert.Equal(t, "", cfg.MasterName)
}

// ============== 6. MySQLConfig Slaves 默认值测试 ==============

func TestMySQLConfig_NoSlaves(t *testing.T) {
	cfg := MySQLConfig{
		Host: "localhost",
		Port: "3306",
	}
	assert.Nil(t, cfg.Slaves)
}
