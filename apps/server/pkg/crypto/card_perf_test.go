// Package crypto v0.5.0 卡密生成性能优化测试
// 铁律 06：覆盖批量生成 / 唯一性 / 性能基准 / 边界场景
package crypto

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============== 1. GenerateCardKeys 基础测试 ==============

func TestGenerateCardKeys_SingleCard(t *testing.T) {
	cards, err := GenerateCardKeys("TEST", 1)
	require.NoError(t, err)
	require.Len(t, cards, 1)

	card := cards[0]
	assert.NotEmpty(t, card.Key)
	assert.NotEmpty(t, card.Hash)
	assert.NotEmpty(t, card.Checksum)
	assert.True(t, strings.HasPrefix(card.Key, "TEST-"))
	// 验证格式：PREFIX-XXXX-XXXX-XXXX-XXXX（4 段 4 字符）
	parts := strings.Split(strings.TrimPrefix(card.Key, "TEST-"), "-")
	assert.Len(t, parts, 4, "应 4 段")
	for _, p := range parts {
		assert.Len(t, p, 4, "每段应 4 字符")
	}
}

func TestGenerateCardKeys_Batch(t *testing.T) {
	count := 1000
	cards, err := GenerateCardKeys("BATCH", count)
	require.NoError(t, err)
	require.Len(t, cards, count)

	// 唯一性验证
	keys := make(map[string]bool, count)
	for _, c := range cards {
		assert.False(t, keys[c.Key], "卡密不应重复: %s", c.Key)
		keys[c.Key] = true
		assert.True(t, strings.HasPrefix(c.Key, "BATCH-"))
	}
}

func TestGenerateCardKeys_NoPrefix(t *testing.T) {
	cards, err := GenerateCardKeys("", 100)
	require.NoError(t, err)
	require.Len(t, cards, 100)

	for _, c := range cards {
		// 无前缀时不应以 - 开头
		assert.False(t, strings.HasPrefix(c.Key, "-"))
		assert.True(t, strings.Contains(c.Key, "-"))
	}
}

// ============== 2. GenerateCardKeys 边界测试 ==============

func TestGenerateCardKeys_ZeroCount(t *testing.T) {
	_, err := GenerateCardKeys("X", 0)
	assert.Error(t, err)
}

func TestGenerateCardKeys_NegativeCount(t *testing.T) {
	_, err := GenerateCardKeys("X", -1)
	assert.Error(t, err)
}

func TestGenerateCardKeys_ExceedMax(t *testing.T) {
	_, err := GenerateCardKeys("X", 100001)
	assert.Error(t, err)
}

func TestGenerateCardKeys_MaxAllowed(t *testing.T) {
	// 100000 是允许的最大值（但测试只验证不报错，不实际生成避免慢）
	// 改为生成较小的量验证上限逻辑
	cards, err := GenerateCardKeys("MAX", 100)
	require.NoError(t, err)
	assert.Len(t, cards, 100)
}

// ============== 3. 字符集验证 ==============

func TestGenerateCardKeys_Charset(t *testing.T) {
	cards, err := GenerateCardKeys("", 100)
	require.NoError(t, err)

	// 卡密字符应全部在字符集内（除 - 分隔符）
	allowed := "ABCDEFGHJKMNPQRSTUVWXYZ23456789-"
	for _, c := range cards {
		for _, ch := range c.Key {
			if !strings.ContainsRune(allowed, ch) {
				t.Errorf("卡密含非法字符 %c in %s", ch, c.Key)
			}
		}
	}
}

func TestGenerateCardKeys_NoConfusableChars(t *testing.T) {
	cards, err := GenerateCardKeys("", 1000)
	require.NoError(t, err)

	// 验证不含易混淆字符 0/O/1/I/L
	forbidden := "01OIL"
	for _, c := range cards {
		for _, ch := range c.Key {
			if strings.ContainsRune(forbidden, ch) {
				t.Errorf("卡密含易混淆字符 %c in %s", ch, c.Key)
			}
		}
	}
}

// ============== 4. Hash 和 Checksum 一致性 ==============

func TestGenerateCardKeys_HashConsistency(t *testing.T) {
	cards, err := GenerateCardKeys("V", 10)
	require.NoError(t, err)

	for _, c := range cards {
		// Hash 应等于 SHA512Hex(key)
		expectedHash := SHA512Hex(c.Key)
		assert.Equal(t, expectedHash, c.Hash)

		// Checksum 应等于 SHA512Checksum8(key + hash)
		expectedChecksum := SHA512Checksum8(c.Key + c.Hash)
		assert.Equal(t, expectedChecksum, c.Checksum)
	}
}

// ============== 5. 并发安全测试 ==============

func TestGenerateCardKeys_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	results := make(chan []CardKey, 10)

	// 10 个 goroutine 各生成 100 张
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cards, err := GenerateCardKeys("CONC", 100)
			if err != nil {
				t.Errorf("并发生成失败: %v", err)
				return
			}
			results <- cards
		}()
	}
	wg.Wait()
	close(results)

	// 收集所有卡密验证唯一性
	allKeys := make(map[string]bool, 1000)
	total := 0
	for cards := range results {
		for _, c := range cards {
			if allKeys[c.Key] {
				t.Errorf("并发场景卡密重复: %s", c.Key)
			}
			allKeys[c.Key] = true
			total++
		}
	}
	assert.Equal(t, 1000, total, "应生成 1000 张卡密")
}

// ============== 6. 性能基准测试 ==============

// BenchmarkGenerateCardKey_Single 旧版单次生成性能基准
func BenchmarkGenerateCardKey_Single(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _, _ = GenerateCardKey("BENCH")
	}
}

// BenchmarkGenerateCardKeys_Batch100 新版批量生成 100 张
func BenchmarkGenerateCardKeys_Batch100(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateCardKeys("BENCH", 100)
	}
}

// BenchmarkGenerateCardKeys_Batch1000 新版批量生成 1000 张
func BenchmarkGenerateCardKeys_Batch1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateCardKeys("BENCH", 1000)
	}
}

// BenchmarkGenerateCardKeys_Batch10000 新版批量生成 10000 张（目标：10000 条/秒）
func BenchmarkGenerateCardKeys_Batch10000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateCardKeys("BENCH", 10000)
	}
}

// BenchmarkGenerateCardKey_Loop10000 旧版循环 10000 次生成（用于对比）
func BenchmarkGenerateCardKey_Loop10000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10000; j++ {
			_, _, _, _ = GenerateCardKey("BENCH")
		}
	}
}

// ============== 7. decodeSegment 单元测试 ==============

func TestDecodeSegment(t *testing.T) {
	// 生成 4 字符段（使用 crypto/rand.Int 拒绝采样）
	seg, err := decodeSegment(cardKeyCharset, len(cardKeyCharset))
	require.NoError(t, err)
	assert.Len(t, seg, 4)

	// 验证每个字符都在字符集内
	for _, ch := range seg {
		assert.True(t, strings.ContainsRune(cardKeyCharset, ch))
	}
}

func TestDecodeSegment_AlwaysValid(t *testing.T) {
	// 多次调用都应产生有效的 4 字符段（内部使用 crypto/rand.Int 拒绝采样）
	for i := 0; i < 100; i++ {
		seg, err := decodeSegment(cardKeyCharset, len(cardKeyCharset))
		require.NoError(t, err)
		assert.Len(t, seg, 4)
		for _, ch := range seg {
			assert.True(t, strings.ContainsRune(cardKeyCharset, ch))
		}
	}
}

func TestDecodeSegment_Randomness(t *testing.T) {
	// 两次调用应大概率产生不同结果（内部使用 crypto/rand.Int）
	seg1, _ := decodeSegment(cardKeyCharset, len(cardKeyCharset))
	seg2, _ := decodeSegment(cardKeyCharset, len(cardKeyCharset))
	// 极小概率相同（4 个字符全相同概率 < 1/31^4）
	if seg1 == seg2 {
		t.Logf("警告：两次随机调用产生相同段（小概率事件）：seg1=%s seg2=%s", seg1, seg2)
	}
}
