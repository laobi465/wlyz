// Package heartbeat 心跳保活单元测试
// 用 miniredis 内存 Redis 测试，避免依赖真实 Redis
package heartbeat

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMiniRedis 启动 miniredis + 返回 redis.Client
func setupMiniRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { _ = rdb.Close() })

	return rdb, mr
}

// ============== Record ==============

func TestRecord_Success(t *testing.T) {
	rdb, mr := setupMiniRedis(t)
	ctx := context.Background()

	err := Record(ctx, rdb, 1, 100, "1.2.3.4", "test-ua")
	require.NoError(t, err)

	// 校验 ZSET 中应有 member=100
	score, err := rdb.ZScore(ctx, onlineKey(1), "100").Result()
	require.NoError(t, err)
	assert.Greater(t, score, 0.0)

	// 校验详情 Hash
	detail, err := rdb.HGetAll(ctx, detailKey(1, 100)).Result()
	require.NoError(t, err)
	assert.Equal(t, "1.2.3.4", detail["ip"])
	assert.Equal(t, "test-ua", detail["user_agent"])
	assert.NotEmpty(t, detail["last_ts"])

	// miniredis 应有 2 个 key（online ZSET + detail Hash）
	assert.Equal(t, 2, len(mr.Keys()))
}

func TestRecord_NilRedis(t *testing.T) {
	err := Record(context.Background(), nil, 1, 100, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis 未初始化")
}

func TestRecord_UpdateExistingDevice(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()

	// 第一次记录
	require.NoError(t, Record(ctx, rdb, 1, 100, "1.1.1.1", "ua1"))
	time.Sleep(time.Second) // 确保 score 递增
	// 第二次记录（同一设备）
	require.NoError(t, Record(ctx, rdb, 1, 100, "2.2.2.2", "ua2"))

	// 应仍只有 1 个 ZSET member
	count, _ := rdb.ZCard(ctx, onlineKey(1)).Result()
	assert.Equal(t, int64(1), count)

	// 详情应更新为最新值
	detail, _ := rdb.HGetAll(ctx, detailKey(1, 100)).Result()
	assert.Equal(t, "2.2.2.2", detail["ip"])
	assert.Equal(t, "ua2", detail["user_agent"])
}

// ============== IsOnline ==============

func TestIsOnline_Online(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()
	require.NoError(t, Record(ctx, rdb, 1, 100, "", ""))

	// 1 秒前的心跳，180 秒超时 → 应在线
	online, err := IsOnline(ctx, rdb, 1, 100, 180)
	require.NoError(t, err)
	assert.True(t, online)
}

func TestIsOnline_Offline(t *testing.T) {
	rdb, mr := setupMiniRedis(t)
	ctx := context.Background()
	require.NoError(t, Record(ctx, rdb, 1, 100, "", ""))

	// 推进 miniredis 时间 200 秒（超过 180 秒超时）
	mr.FastForward(200 * time.Second)

	online, err := IsOnline(ctx, rdb, 1, 100, 180)
	require.NoError(t, err)
	assert.False(t, online)
}

func TestIsOnline_NeverHeartbeat(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()

	// 从未心跳的设备 → redis.Nil → 返回 (false, nil)
	online, err := IsOnline(ctx, rdb, 1, 999, 180)
	require.NoError(t, err)
	assert.False(t, online)
}

func TestIsOnline_DefaultTimeout(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()
	require.NoError(t, Record(ctx, rdb, 1, 100, "", ""))

	// heartbeatTimeout=0 应默认使用 180 秒
	online, err := IsOnline(ctx, rdb, 1, 100, 0)
	require.NoError(t, err)
	assert.True(t, online)
}

func TestIsOnline_NilRedis(t *testing.T) {
	online, err := IsOnline(context.Background(), nil, 1, 100, 180)
	require.Error(t, err)
	assert.False(t, online)
}

// ============== Remove ==============

func TestRemove_Success(t *testing.T) {
	rdb, mr := setupMiniRedis(t)
	ctx := context.Background()
	require.NoError(t, Record(ctx, rdb, 1, 100, "", ""))
	require.NoError(t, Record(ctx, rdb, 1, 200, "", ""))

	// 删除 device 100
	require.NoError(t, Remove(ctx, rdb, 1, 100))

	// 100 应被移除
	online, _ := IsOnline(ctx, rdb, 1, 100, 180)
	assert.False(t, online)
	// 200 应仍在线
	online, _ = IsOnline(ctx, rdb, 1, 200, 180)
	assert.True(t, online)

	// 详情 Hash 也应被删除
	exists, _ := rdb.Exists(ctx, detailKey(1, 100)).Result()
	assert.Equal(t, int64(0), exists)
}

func TestRemove_NonExistent(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	// 删除不存在的设备应不报错
	err := Remove(context.Background(), rdb, 1, 999)
	require.NoError(t, err)
}

func TestRemove_NilRedis(t *testing.T) {
	// nil rdb 应静默返回 nil（不报错）
	err := Remove(context.Background(), nil, 1, 100)
	require.NoError(t, err)
}

// ============== CountOnline ==============

func TestCountOnline(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()

	// 记录 3 个设备
	for _, dev := range []uint64{100, 200, 300} {
		require.NoError(t, Record(ctx, rdb, 1, dev, "", ""))
	}

	// 应返回 3
	count, err := CountOnline(ctx, rdb, 1, 180)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestCountOnline_ExcludesOffline(t *testing.T) {
	rdb, mr := setupMiniRedis(t)
	ctx := context.Background()
	require.NoError(t, Record(ctx, rdb, 1, 100, "", ""))
	require.NoError(t, Record(ctx, rdb, 1, 200, "", ""))

	// 推进 200 秒，使所有设备离线
	mr.FastForward(200 * time.Second)

	count, err := CountOnline(ctx, rdb, 1, 180)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "超时设备不应计入在线数")
}

func TestCountOnline_DefaultTimeout(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()
	require.NoError(t, Record(ctx, rdb, 1, 100, "", ""))

	count, err := CountOnline(ctx, rdb, 1, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestCountOnline_NilRedis(t *testing.T) {
	_, err := CountOnline(context.Background(), nil, 1, 180)
	require.Error(t, err)
}

// ============== ListOnline ==============

func TestListOnline(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()
	for _, dev := range []uint64{100, 200, 300} {
		require.NoError(t, Record(ctx, rdb, 1, dev, "", ""))
	}

	ids, err := ListOnline(ctx, rdb, 1, 180, 0, 100)
	require.NoError(t, err)
	assert.Len(t, ids, 3)
	// 应包含全部 3 个设备
	idSet := map[uint64]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	assert.True(t, idSet[100])
	assert.True(t, idSet[200])
	assert.True(t, idSet[300])
}

func TestListOnline_Pagination(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()
	for i := uint64(1); i <= 10; i++ {
		require.NoError(t, Record(ctx, rdb, 1, i, "", ""))
	}

	// 取前 5 个
	ids, err := ListOnline(ctx, rdb, 1, 180, 0, 5)
	require.NoError(t, err)
	assert.Len(t, ids, 5)
}

func TestListOnline_NilRedis(t *testing.T) {
	_, err := ListOnline(context.Background(), nil, 1, 180, 0, 100)
	require.Error(t, err)
}

// ============== GetLastHeartbeatAt ==============

func TestGetLastHeartbeatAt_Success(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()
	before := time.Now().Unix()
	require.NoError(t, Record(ctx, rdb, 1, 100, "", ""))
	after := time.Now().Unix()

	t1, err := GetLastHeartbeatAt(ctx, rdb, 1, 100)
	require.NoError(t, err)
	ts := t1.Unix()
	assert.GreaterOrEqual(t, ts, before)
	assert.LessOrEqual(t, ts, after)
}

func TestGetLastHeartbeatAt_NeverHeartbeat(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	ctx := context.Background()

	// 从未心跳的设备 → 返回零时间 + nil error
	t1, err := GetLastHeartbeatAt(ctx, rdb, 1, 999)
	require.NoError(t, err)
	assert.True(t, t1.IsZero())
}

func TestGetLastHeartbeatAt_NilRedis(t *testing.T) {
	_, err := GetLastHeartbeatAt(context.Background(), nil, 1, 100)
	require.Error(t, err)
}

// ============== Redis Key 规范 ==============

func TestOnlineKey_Format(t *testing.T) {
	assert.Equal(t, "heartbeat:online:42", onlineKey(42))
	assert.Equal(t, "heartbeat:online:0", onlineKey(0))
}

func TestDetailKey_Format(t *testing.T) {
	assert.Equal(t, "heartbeat:detail:42:100", detailKey(42, 100))
	assert.Equal(t, "heartbeat:detail:0:0", detailKey(0, 0))
}

// ============== 端到端：Record → IsOnline → Remove 闭环 ==============

func TestHeartbeat_RoundTrip(t *testing.T) {
	rdb, mr := setupMiniRedis(t)
	ctx := context.Background()

	// 1. 记录心跳
	require.NoError(t, Record(ctx, rdb, 5, 500, "10.0.0.1", "Chrome/100"))
	online, _ := IsOnline(ctx, rdb, 5, 500, 60)
	assert.True(t, online)

	// 2. 推进时间 100 秒（超过 60 秒超时）→ 离线
	mr.FastForward(100 * time.Second)
	online, _ = IsOnline(ctx, rdb, 5, 500, 60)
	assert.False(t, online)

	// 3. 再次心跳 → 在线
	require.NoError(t, Record(ctx, rdb, 5, 500, "10.0.0.2", "Chrome/101"))
	online, _ = IsOnline(ctx, rdb, 5, 500, 60)
	assert.True(t, online)

	// 4. 移除 → 离线
	require.NoError(t, Remove(ctx, rdb, 5, 500))
	online, _ = IsOnline(ctx, rdb, 5, 500, 60)
	assert.False(t, online)

	// 5. GetLastHeartbeatAt 应返回零时间
	t1, _ := GetLastHeartbeatAt(ctx, rdb, 5, 500)
	assert.True(t, t1.IsZero())
}
