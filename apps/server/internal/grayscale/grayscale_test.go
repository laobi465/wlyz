// Package grayscale 灰度发布核心逻辑单元测试
// v0.4.0：覆盖 Match / HashBucket / ParseList 三大核心路径
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package grayscale

import (
	"context"
	"testing"

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

// setupTestDB 启动 SQLite 内存库 + AutoMigrate sys_config
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SysConfig{}))
	db.Exec("DELETE FROM sys_config")
	return db
}

// setupTestCfgCache 启动 miniredis + ConfigCache + 预置灰度配置
func setupTestCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) *config.ConfigCache {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	defaults := map[string]string{
		CfgKeyEnabled:     "1",
		CfgKeyDefaultRate: "10.00",
		CfgKeyHashSalt:    "test-salt-v040",
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
			ConfigGroup: "app",
		}).Error)
	}
	return config.NewConfigCache(db, rdb)
}

// newVersion 构造测试用 AppVersion
func newVersion(appID uint64, strategy string, rate float64, platforms, regions, channels string) *model.AppVersion {
	return &model.AppVersion{
		BaseModel:           model.BaseModel{ID: 1},
		TenantID:            100,
		AppID:               appID,
		Version:             "1.0.0",
		Channel:             "stable",
		ReleaseStrategy:     strategy,
		GrayscaleRate:       rate,
		GrayscalePlatforms:  platforms,
		GrayscaleRegions:    regions,
		GrayscaleChannels:   channels,
		Status:              "active",
	}
}

// ============== 1. Match 全量策略 ==============

func TestMatch_FullStrategy_AlwaysMatched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyFull, 0, "", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:   ver,
		ClientID:  "client-001",
		Platform:  "windows",
		Channel:   "stable",
		Region:    "beijing",
	})
	assert.True(t, result.Matched, "full 策略应始终命中")
	assert.Equal(t, "full strategy", result.Reason)
}

func TestMatch_EmptyStrategy_DefaultFull(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, "", 0, "", "", "")
	result := Match(context.Background(), cfg, MatchRequest{Version: ver})
	assert.True(t, result.Matched, "空策略应默认为 full 命中")
}

func TestMatch_NilVersion_NotMatched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	result := Match(context.Background(), cfg, MatchRequest{Version: nil})
	assert.False(t, result.Matched)
	assert.Equal(t, "version is nil", result.Reason)
}

// ============== 2. Match 全局开关 ==============

func TestMatch_GrayscaleDisabledGlobally_FallbackToFull(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, map[string]string{
		CfgKeyEnabled: "0",
	})

	ver := newVersion(1, StrategyGrayscale, 10.0, "", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "any-client",
	})
	assert.True(t, result.Matched, "全局开关关闭时应降级为全量命中")
	assert.Contains(t, result.Reason, "fallback to full")
}

// ============== 3. Match 平台过滤 ==============

func TestMatch_PlatformInList_Matched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "windows,macos", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Platform: "windows",
	})
	assert.True(t, result.Matched, "windows 在列表中应命中")
}

func TestMatch_PlatformNotInList_NotMatched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "windows,macos", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Platform: "linux",
	})
	assert.False(t, result.Matched, "linux 不在列表中应不命中")
	assert.Contains(t, result.Reason, "platform not in grayscale list")
}

func TestMatch_PlatformCaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "Windows,MacOS", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Platform: "WINDOWS",
	})
	assert.True(t, result.Matched, "平台名应大小写不敏感")
}

func TestMatch_EmptyPlatform_NotMatchedWhenLimited(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "windows", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Platform: "", // 客户端未传 platform
	})
	assert.False(t, result.Matched, "版本限制了 platform 但客户端未传应不命中")
}

// ============== 4. Match 渠道过滤 ==============

func TestMatch_ChannelInList_Matched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "", "", "stable,beta")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Channel:  "beta",
	})
	assert.True(t, result.Matched, "beta 在渠道列表中应命中")
}

func TestMatch_ChannelNotInList_NotMatched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "", "", "stable")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Channel:  "dev",
	})
	assert.False(t, result.Matched, "dev 不在渠道列表中应不命中")
}

func TestMatch_EmptyChannelDefaultsToStable(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "", "", "stable")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Channel:  "", // 客户端未传 channel，应默认 stable
	})
	assert.True(t, result.Matched, "空 channel 应默认 stable 命中")
}

// ============== 5. Match 地区过滤 ==============

func TestMatch_RegionInList_Matched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "", "beijing,shanghai", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Region:   "beijing",
	})
	assert.True(t, result.Matched, "beijing 在地区列表中应命中")
}

func TestMatch_RegionNotInList_NotMatched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "", "beijing,shanghai", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Region:   "guangzhou",
	})
	assert.False(t, result.Matched, "guangzhou 不在地区列表中应不命中")
}

func TestMatch_EmptyRegion_NotMatchedWhenLimited(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "", "beijing", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Region:   "",
	})
	assert.False(t, result.Matched, "版本限制了 region 但客户端未传应不命中")
}

// ============== 6. Match 灰度比例 ==============

func TestMatch_RateZero_NotMatched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 0.0, "", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
	})
	assert.False(t, result.Matched, "灰度比例为 0 应不命中")
	assert.Contains(t, result.Reason, "rate <= 0")
}

func TestMatch_RateFull_Matched(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
	})
	assert.True(t, result.Matched, "灰度比例为 100 应全量命中")
	assert.Contains(t, result.Reason, "rate >= 100")
}

func TestMatch_RatePartial_BucketHit(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, map[string]string{
		CfgKeyHashSalt: "fixed-salt-for-test",
	})

	ver := newVersion(1, StrategyGrayscale, 50.0, "", "", "")
	// 通过 HashBucket 算出 client-001 在 salt=fixed-salt-for-test 下的桶
	bucket := HashBucket("fixed-salt-for-test", 1, "client-001")
	t.Logf("client-001 bucket = %d", bucket)

	result := Match(context.Background(), cfg, MatchRequest{
		Version:   ver,
		ClientID:  "client-001",
		Platform:  "",
		Region:    "",
		Channel:   "",
	})
	assert.Equal(t, bucket < 50, result.Matched, "命中应与桶 < 50 一致")
	assert.Equal(t, bucket, result.Bucket)
}

func TestMatch_RatePartial_BucketMiss(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, map[string]string{
		CfgKeyHashSalt: "fixed-salt-for-test",
	})

	// 用一个桶 >= 50 的 clientID（通过尝试找到这样的 ID）
	ver := newVersion(1, StrategyGrayscale, 50.0, "", "", "")
	// 尝试多个 clientID，验证至少存在不命中的情况
	var missFound bool
	for i := 0; i < 100; i++ {
		clientID := "client-miss-" + string(rune('a'+i))
		bucket := HashBucket("fixed-salt-for-test", 1, clientID)
		if bucket >= 50 {
			result := Match(context.Background(), cfg, MatchRequest{
				Version:  ver,
				ClientID: clientID,
			})
			assert.False(t, result.Matched, "桶 %d >= 50 应不命中", bucket)
			missFound = true
			break
		}
	}
	assert.True(t, missFound, "100 个 clientID 中应至少有一个桶 >= 50")
}

// ============== 7. HashBucket 稳定性 ==============

func TestHashBucket_Stable(t *testing.T) {
	// 同一 (salt, appID, clientID) 三元组应返回相同桶
	b1 := HashBucket("salt-a", 100, "client-x")
	b2 := HashBucket("salt-a", 100, "client-x")
	assert.Equal(t, b1, b2, "同一参数应返回相同桶")
}

func TestHashBucket_Range0to99(t *testing.T) {
	// 1000 个 clientID 的桶都应在 [0, 99]
	for i := 0; i < 1000; i++ {
		clientID := "client-" + string(rune('a'+i%26)) + string(rune('a'+i/26%26))
		b := HashBucket("salt-a", 100, clientID)
		assert.GreaterOrEqual(t, b, 0)
		assert.Less(t, b, 100)
	}
}

func TestHashBucket_DifferentSaltDifferentBucket(t *testing.T) {
	// 修改 salt 应导致桶变化（用于紧急回滚灰度）
	// 不强求一定不同（理论有 1% 概率相同），但 100 个 clientID 中至少有 50 个不同
	var diffCount int
	for i := 0; i < 100; i++ {
		clientID := "client-" + string(rune('a'+i%26)) + string(rune('a'+i/26%26))
		if HashBucket("salt-a", 100, clientID) != HashBucket("salt-b", 100, clientID) {
			diffCount++
		}
	}
	assert.Greater(t, diffCount, 50, "修改 salt 应导致大多数 clientID 桶变化")
}

func TestHashBucket_DifferentAppIDDifferentBucket(t *testing.T) {
	// 同一 clientID 在不同 app 下的桶应独立
	// 不强求一定不同，但 100 个 clientID 中至少有 50 个不同
	var diffCount int
	for i := 0; i < 100; i++ {
		clientID := "client-" + string(rune('a'+i%26)) + string(rune('a'+i/26%26))
		if HashBucket("salt-a", 100, clientID) != HashBucket("salt-a", 200, clientID) {
			diffCount++
		}
	}
	assert.Greater(t, diffCount, 50, "不同 appID 应独立分桶")
}

// ============== 8. ParseList ==============

func TestParseList_Empty(t *testing.T) {
	assert.Nil(t, ParseList(""))
}

func TestParseList_Single(t *testing.T) {
	assert.Equal(t, []string{"windows"}, ParseList("windows"))
}

func TestParseList_Multiple(t *testing.T) {
	assert.Equal(t, []string{"windows", "macos", "linux"}, ParseList("windows,macos,linux"))
}

func TestParseList_WithSpaces(t *testing.T) {
	assert.Equal(t, []string{"windows", "macos"}, ParseList(" windows , macOS "))
}

func TestParseList_MixedCase(t *testing.T) {
	assert.Equal(t, []string{"windows", "macos", "linux"}, ParseList("Windows,MacOS,LINUX"))
}

func TestParseList_OnlyCommas(t *testing.T) {
	assert.Empty(t, ParseList(" , , , "))
}

// ============== 9. DefaultRate / IsEnabled ==============

func TestDefaultRate_ReadFromConfig(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, map[string]string{
		CfgKeyDefaultRate: "25.50",
	})
	assert.Equal(t, 25.50, DefaultRate(context.Background(), cfg))
}

func TestDefaultRate_FallbackWhenMissing(t *testing.T) {
	// 不预置 default_rate，应返回 fallback 10.00
	db := setupTestDB(t)
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	// 只预置其他配置，不预置 default_rate
	require.NoError(t, db.Create(&model.SysConfig{
		ConfigKey: CfgKeyEnabled, ConfigValue: "1", ConfigType: "string", ConfigGroup: "app",
	}).Error)
	require.NoError(t, db.Create(&model.SysConfig{
		ConfigKey: CfgKeyHashSalt, ConfigValue: "s", ConfigType: "string", ConfigGroup: "app",
	}).Error)

	cfg := config.NewConfigCache(db, rdb)
	assert.Equal(t, 10.00, DefaultRate(context.Background(), cfg))
}

func TestIsEnabled_TrueByDefault(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)
	assert.True(t, IsEnabled(context.Background(), cfg))
}

func TestIsEnabled_FalseWhenDisabled(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, map[string]string{
		CfgKeyEnabled: "0",
	})
	assert.False(t, IsEnabled(context.Background(), cfg))
}

// ============== 10. 边界场景 ==============

func TestMatch_AnonymousClientIDWhenEmpty(t *testing.T) {
	// 客户端未传 clientID 时应使用 "anonymous" 兜底，不应 panic
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "",
	})
	assert.True(t, result.Matched, "rate=100 应命中，clientID 为空走 anonymous 兜底")
}

func TestMatch_CanaryStrategy_SameAsGrayscale(t *testing.T) {
	// canary 策略与 grayscale 同逻辑（语义用于内部测试）
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyCanary, 100.0, "", "", "")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
	})
	assert.True(t, result.Matched, "canary 策略 rate=100 应命中")
}

func TestMatch_MultiFilter_AllPass(t *testing.T) {
	// 同时限制 platform + channel + region，全部满足应命中
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "windows", "beijing", "stable")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Platform: "windows",
		Region:   "beijing",
		Channel:  "stable",
	})
	assert.True(t, result.Matched, "多维过滤全通过应命中")
}

func TestMatch_MultiFilter_OneFail(t *testing.T) {
	// 多维过滤中任一不满足都不命中
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ver := newVersion(1, StrategyGrayscale, 100.0, "windows", "beijing", "stable")
	result := Match(context.Background(), cfg, MatchRequest{
		Version:  ver,
		ClientID: "c1",
		Platform: "windows",
		Region:   "shanghai", // 不在 beijing 列表
		Channel:  "stable",
	})
	assert.False(t, result.Matched, "region 不匹配应不命中")
	assert.Contains(t, result.Reason, "region not in grayscale list")
}
