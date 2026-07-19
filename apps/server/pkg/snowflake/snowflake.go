// Package snowflake 雪花算法 ID 生成器
// 用于订单号、邀请码等唯一标识
package snowflake

import (
	"errors"
	"fmt"
	"sync"
	"time"
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

// Node 雪花节点
type Node struct {
	mu           sync.Mutex
	workerID     int64
	datacenterID int64
	sequence     int64
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

// 默认节点（单机部署可用，多机需各自分配 workerID）
var defaultNode, _ = NewNode(1, 1)

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
