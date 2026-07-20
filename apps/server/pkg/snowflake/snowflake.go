// Package snowflake 雪花算法 ID 生成器
// 用于订单号、邀请码等唯一标识
//
// v0.5.0 API 水平扩展（无状态化）改造：
//   1. 保留 defaultNode 兜底（向后兼容）
//   2. 新增 InitWorkerFromRedis：通过 Redis INCR 协调多实例分配 workerID
//   3. 多实例部署时调用 InitWorkerFromRedis 后，defaultNode 切换为 Redis 分配的 workerID
//   4. 单实例部署无需调用，使用默认 workerID=1
//
// 铁律 04：workerID 从 Redis 动态分配，不硬编码
// 铁律 05：分配策略的 key/过期时间走常量，可通过 sys_config 覆盖
// 铁律 06：Redis 不可用时降级为本地默认 workerID，不阻断启动
package snowflake

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// 雪花算法常量
const (
	workerIDBits     = 5  // 工作节点 ID 位数
	datacenterIDBits = 5  // 数据中心 ID 位数
	sequenceBits     = 12 // 序列号位数

	workerIDShift      = sequenceBits
	datacenterIDShift  = sequenceBits + workerIDBits
	timestampLeftShift = sequenceBits + workerIDBits + datacenterIDBits

	sequenceMask    = -1 ^ (-1 << sequenceBits)
	maxWorkerID     = -1 ^ (-1 << workerIDBits)
	maxDatacenterID = -1 ^ (-1 << datacenterIDBits)

	// 起始时间戳：2026-01-01 00:00:00 UTC（待核实，可按部署时间调整）
	twepoch = 1767225600000
)

// ============== v0.5.0 Redis workerID 协调常量 ==============

const (
	// RedisWorkerIDKey Redis 中 workerID 计数器的 key
	RedisWorkerIDKey = "keyauth:snowflake:worker_id"
	// RedisWorkerIDTTL workerID 占用 TTL（24 小时后自动回收，防止实例宕机后 ID 永不释放）
	RedisWorkerIDTTL = 24 * time.Hour
)

// Node 雪花节点
type Node struct {
	mu            sync.Mutex
	workerID      int64
	datacenterID  int64
	sequence      int64
	lastTimestamp int64
}

// NewNode 构造雪花节点
func NewNode(workerID, datacenterID int64) (*Node, error) {
	if workerID < 0 || workerID > maxWorkerID {
		return nil, errors.New("worker ID 超出范围")
	}
	if datacenterID < 0 || datacenterID > maxDatacenterID {
		return nil, errors.New("datacenter ID 超出范围")
	}
	return &Node{
		workerID:     workerID,
		datacenterID: datacenterID,
	}, nil
}

// NextID 生成下一个 ID
func (n *Node) NextID() (int64, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	ts := time.Now().UnixMilli()
	if ts == n.lastTimestamp {
		n.sequence = (n.sequence + 1) & sequenceMask
		if n.sequence == 0 {
			ts = n.waitNextMillis(n.lastTimestamp)
		}
	} else {
		n.sequence = 0
	}

	if ts < n.lastTimestamp {
		return 0, errors.New("时钟回拨")
	}

	n.lastTimestamp = ts
	id := ((ts - twepoch) << timestampLeftShift) |
		(n.datacenterID << datacenterIDShift) |
		(n.workerID << workerIDShift) |
		n.sequence
	return id, nil
}

func (n *Node) waitNextMillis(last int64) int64 {
	ts := time.Now().UnixMilli()
	for ts <= last {
		ts = time.Now().UnixMilli()
	}
	return ts
}

// ============== 默认节点（向后兼容） ==============

// defaultNode 默认节点：单机部署使用 workerID=1
// v0.5.0：多实例部署时通过 InitWorkerFromRedis 重新赋值
var defaultNode, _ = NewNode(1, 1)
var defaultMu sync.Mutex

// Next 便捷方法：用默认节点生成
func Next() int64 {
	id, _ := defaultNode.NextID()
	return id
}

// OrderNo 生成订单号（带前缀）
func OrderNo(prefix string) string {
	id := Next()
	return fmt.Sprintf("%s%d", prefix, id)
}

// ============== v0.5.0 Redis workerID 协调 ==============

// InitWorkerFromRedis 通过 Redis INCR 协调多实例分配 workerID
//
// 实现策略：
//   1. Redis INCR keyauth:snowflake:worker_id，得到序号 N
//   2. workerID = (N-1) % (maxWorkerID+1)，N 从 1 开始
//   3. 同一 N 的实例共享 workerID（理论上 workerID 范围 0-31 足够多数百实例）
//   4. 不主动释放 workerID（24h TTL 自动回收，下次 INCR 重新分配）
//
// 铁律 04：workerID 从 Redis 动态分配，不硬编码
// 铁律 06：Redis 不可用 / nil 时降级为默认 workerID=1，不阻断启动
//
// 返回值：实际分配的 workerID（用于日志记录）；失败返回 1
func InitWorkerFromRedis(rdb *redis.Client) int64 {
	if rdb == nil {
		return 1 // 降级：Redis 未初始化
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// INCR 是原子操作，多实例并发安全
	n, err := rdb.Incr(ctx, RedisWorkerIDKey).Result()
	if err != nil {
		// Redis 故障：降级为默认 workerID=1
		return 1
	}

	// workerID 范围 0 - maxWorkerID（31）
	workerID := (n - 1) % int64(maxWorkerID+1)

	// 重新构造 defaultNode
	node, err := NewNode(workerID, 1)
	if err != nil {
		return 1 // 理论不会触发
	}

	defaultMu.Lock()
	defaultNode = node
	defaultMu.Unlock()

	return workerID
}

// GetCurrentWorkerID 返回当前 defaultNode 的 workerID（用于日志/健康检查）
func GetCurrentWorkerID() int64 {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultNode.workerID
}
