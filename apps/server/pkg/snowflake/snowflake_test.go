// pkg/snowflake 单元测试
// 覆盖 NewNode 范围校验 / NextID 单调递增 / OrderNo 前缀 / 并发安全
package snowflake

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============== NewNode ==============

func TestNewNode_Valid(t *testing.T) {
	// workerID / datacenterID 范围 [0, 31]
	for w := int64(0); w <= 31; w++ {
		for d := int64(0); d <= 31; d++ {
			node, err := NewNode(w, d)
			require.NoError(t, err, "w=%d d=%d 应通过", w, d)
			require.NotNil(t, node)
		}
	}
}

func TestNewNode_InvalidWorkerID(t *testing.T) {
	_, err := NewNode(-1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worker")

	_, err = NewNode(32, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worker")
}

func TestNewNode_InvalidDatacenterID(t *testing.T) {
	_, err := NewNode(1, -1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "datacenter")

	_, err = NewNode(1, 32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "datacenter")
}

// ============== NextID ==============

func TestNextID_MonotonicIncrease(t *testing.T) {
	node, _ := NewNode(1, 1)
	var prev int64 = -1
	for i := 0; i < 1000; i++ {
		id, err := node.NextID()
		require.NoError(t, err)
		assert.True(t, id > prev, "ID 应单调递增: prev=%d cur=%d", prev, id)
		prev = id
	}
}

func TestNextID_Unique(t *testing.T) {
	node, _ := NewNode(1, 1)
	seen := make(map[int64]bool, 10000)
	for i := 0; i < 10000; i++ {
		id, err := node.NextID()
		require.NoError(t, err)
		assert.False(t, seen[id], "重复 ID: %d", id)
		seen[id] = true
	}
}

func TestNextID_Positive(t *testing.T) {
	node, _ := NewNode(1, 1)
	id, err := node.NextID()
	require.NoError(t, err)
	assert.True(t, id > 0, "ID 应为正数")
}

// ============== 并发安全 ==============

func TestNextID_ConcurrentSafe(t *testing.T) {
	node, _ := NewNode(1, 1)
	const goroutines = 50
	const perG = 200

	var mu sync.Mutex
	seen := make(map[int64]bool, goroutines*perG)
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				id, err := node.NextID()
				if err != nil {
					t.Errorf("并发 NextID 失败: %v", err)
					return
				}
				mu.Lock()
				if seen[id] {
					t.Errorf("并发重复 ID: %d", id)
					mu.Unlock()
					return
				}
				seen[id] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Len(t, seen, goroutines*perG, "应无并发重复 ID")
}

// ============== 包级便捷函数 ==============

func TestNext_PackageLevel(t *testing.T) {
	// Next() 应返回递增的 ID
	id1 := Next()
	id2 := Next()
	assert.True(t, id2 > id1, "包级 Next 应递增")
}

func TestOrderNo_Prefix(t *testing.T) {
	cases := []string{"ORD", "TOP", "REG", ""}
	for _, prefix := range cases {
		t.Run("prefix="+prefix, func(t *testing.T) {
			no := OrderNo(prefix)
			require.NotEmpty(t, no)
			expectedPrefix := prefix
			if expectedPrefix != "" {
				assert.True(t, strings.HasPrefix(no, expectedPrefix),
					"订单号应以 %s 开头，实际: %s", expectedPrefix, no)
			}
			// 后缀应为纯数字（雪花 ID）
			suffix := strings.TrimPrefix(no, expectedPrefix)
			for _, c := range suffix {
				assert.True(t, c >= '0' && c <= '9', "订单号后缀应为数字")
			}
		})
	}
}

func TestOrderNo_Unique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		no := OrderNo("ORD")
		assert.False(t, seen[no], "重复订单号: %s", no)
		seen[no] = true
	}
}

// ============== 三通道前缀测试（v0.3.6 关键路径） ==============

func TestOrderNo_ThreeChannelPrefixes(t *testing.T) {
	// v0.3.6 三通道前缀分发：ORD / TOP / REG
	// 任意重复 prefix 不应影响其他前缀的 ID 唯一性
	ord := OrderNo("ORD")
	top := OrderNo("TOP")
	reg := OrderNo("REG")

	assert.True(t, strings.HasPrefix(ord, "ORD"))
	assert.True(t, strings.HasPrefix(top, "TOP"))
	assert.True(t, strings.HasPrefix(reg, "REG"))

	// 三个 ID 的后缀（雪花 ID）应不同
	ordID := strings.TrimPrefix(ord, "ORD")
	topID := strings.TrimPrefix(top, "TOP")
	regID := strings.TrimPrefix(reg, "REG")
	assert.NotEqual(t, ordID, topID)
	assert.NotEqual(t, ordID, regID)
	assert.NotEqual(t, topID, regID)

	t.Logf("ORD=%s TOP=%s REG=%s", ord, top, reg)
}

// ============== twepoch 校验 ==============

func TestTwepoch(t *testing.T) {
	// twepoch = 1767225600000 (2026-01-01 00:00:00 UTC)
	// 这是项目固定常量，不应被改动
	assert.Equal(t, 1767225600000, twepoch,
		"twepoch 应为 2026-01-01 UTC 毫秒时间戳")

	// 生成 ID 应远大于 twepoch（否则说明位运算错误）
	node, _ := NewNode(1, 1)
	id, _ := node.NextID()
	assert.True(t, id > int64(twepoch)>>22, "ID 应大于 twepoch 对应的时间位")
}

// ============== 多节点测试 ==============

func TestNextID_DifferentWorkerID(t *testing.T) {
	// 不同 workerID 的节点应能生成不同 ID
	n1, _ := NewNode(1, 1)
	n2, _ := NewNode(2, 1)

	// 同一毫秒（不太可能精确，但概率存在）的 ID 应不同
	id1, _ := n1.NextID()
	id2, _ := n2.NextID()
	assert.NotEqual(t, id1, id2, "不同 worker 应生成不同 ID")

	t.Logf("n1=%d n2=%d", id1, id2)
}

// ============== 基准测试（可选） ==============

func BenchmarkNextID(b *testing.B) {
	node, _ := NewNode(1, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = node.NextID()
	}
}

func BenchmarkOrderNo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = OrderNo("ORD")
	}
}

func BenchmarkNextID_Parallel(b *testing.B) {
	node, _ := NewNode(1, 1)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = node.NextID()
		}
	})
}

// 占位避免 fmt 未使用
var _ = fmt.Sprintf
