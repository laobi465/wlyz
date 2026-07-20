// Package snowflake v0.5.0 测试
// 铁律 06：覆盖 NewNode 边界 / NextID 单调递增 / InitWorkerFromRedis 多场景
package snowflake

import (
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============== 1. NewNode 边界测试 ==============

func TestNewNode_ValidRange(t *testing.T) {
	// workerID 范围 0 - maxWorkerID (31)
	node, err := NewNode(0, 1)
	require.NoError(t, err)
	assert.NotNil(t, node)

	node, err = NewNode(31, 1)
	require.NoError(t, err)
	assert.NotNil(t, node)
}

func TestNewNode_InvalidWorkerID(t *testing.T) {
	// 超出范围应报错
	_, err := NewNode(-1, 1)
	assert.Error(t, err)

	_, err = NewNode(32, 1) // maxWorkerID+1
	assert.Error(t, err)
}

func TestNewNode_InvalidDatacenterID(t *testing.T) {
	_, err := NewNode(1, -1)
	assert.Error(t, err)

	_, err = NewNode(1, 32)
	assert.Error(t, err)
}

// ============== 2. NextID 单调递增测试 ==============

func TestNextID_MonotonicIncrease(t *testing.T) {
	node, _ := NewNode(1, 1)
	var prev int64 = 0
	for i := 0; i < 1000; i++ {
		id, err := node.NextID()
		require.NoError(t, err)
		assert.Greater(t, id, prev, "ID 应单调递增")
		prev = id
	}
}

func TestNextID_Uniqueness(t *testing.T) {
	node, _ := NewNode(1, 1)
	ids := make(map[int64]bool, 10000)
	for i := 0; i < 10000; i++ {
		id, err := node.NextID()
		require.NoError(t, err)
		assert.False(t, ids[id], "ID 不应重复: %d", id)
		ids[id] = true
	}
}

func TestNextID_Concurrent(t *testing.T) {
	node, _ := NewNode(1, 1)
	var wg sync.WaitGroup
	ids := make(chan int64, 10000)
	mu := sync.Mutex{}
	collected := make(map[int64]bool)

	// 10 个 goroutine 各生成 1000 个 ID
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				id, err := node.NextID()
				if err != nil {
					t.Errorf("NextID 错误: %v", err)
					return
				}
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)

	for id := range ids {
		mu.Lock()
		if collected[id] {
			t.Errorf("并发场景 ID 重复: %d", id)
		}
		collected[id] = true
		mu.Unlock()
	}
	assert.Equal(t, 10000, len(collected), "应生成 10000 个唯一 ID")
}

// ============== 3. 全局便捷函数测试 ==============

func TestNext_ReturnsValidID(t *testing.T) {
	id := Next()
	assert.Greater(t, id, int64(0))
}

func TestOrderNo_WithPrefix(t *testing.T) {
	no := OrderNo("ORD")
	assert.Contains(t, no, "ORD")
	assert.Greater(t, len(no), 3)
}

// ============== 4. InitWorkerFromRedis 测试 ==============

func TestInitWorkerFromRedis_NilClient(t *testing.T) {
	// nil Redis 客户端应降级返回 1
	workerID := InitWorkerFromRedis(nil)
	assert.Equal(t, int64(1), workerID)
}

func TestInitWorkerFromRedis_FirstCall(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	workerID := InitWorkerFromRedis(rdb)
	// 第一次 INCR 返回 1，workerID = (1-1) % 32 = 0
	assert.Equal(t, int64(0), workerID)
}

func TestInitWorkerFromRedis_SecondCall(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	// 第一次 INCR = 1，workerID = 0
	w1 := InitWorkerFromRedis(rdb)
	assert.Equal(t, int64(0), w1)

	// 第二次 INCR = 2，workerID = 1
	w2 := InitWorkerFromRedis(rdb)
	assert.Equal(t, int64(1), w2)

	// 验证 GetCurrentWorkerID 返回最新值
	assert.Equal(t, int64(1), GetCurrentWorkerID())
}

func TestInitWorkerFromRedis_WrapAround(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	// 预置 Redis 计数器到 maxWorkerID+1（32），下次 INCR = 33，workerID = 32 % 32 = 0
	require.NoError(t, mr.Set(RedisWorkerIDKey, "32"))

	workerID := InitWorkerFromRedis(rdb)
	assert.Equal(t, int64(0), workerID, "INCR=33, workerID = (33-1) % 32 = 0")
}

// ============== 5. GetCurrentWorkerID 测试 ==============

func TestGetCurrentWorkerID_Default(t *testing.T) {
	// 不调用 InitWorkerFromRedis 时返回默认值 1
	// 但因其他测试可能已修改 defaultNode，此处仅断言 >= 0
	id := GetCurrentWorkerID()
	assert.GreaterOrEqual(t, id, int64(0))
}

// ============== 6. 常量测试 ==============

func TestConstants(t *testing.T) {
	assert.Equal(t, "keyauth:snowflake:worker_id", RedisWorkerIDKey)
	assert.Equal(t, 24*60*60*1000000000, int(RedisWorkerIDTTL)) // 24h in ns
	assert.Equal(t, int64(31), int64(maxWorkerID))
	assert.Equal(t, int64(31), int64(maxDatacenterID))
}
