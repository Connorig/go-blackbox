// Package id 提供脚手架内置的 ID 生成器:雪花算法(数值型)与 UUID v4(字符串型)。
// 零第三方依赖,与公共 Model 体系(SnowflakeModel/StringIDModel)配合使用。
package id

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 雪花算法位分配(对齐主流实现):
//
//	| 1 bit 符号位 | 41 bit 毫秒时间戳 | 10 bit 节点 ID | 12 bit 序列号 |
//
//   - 时间戳:从 epoch 起的毫秒数,支持约 69 年(41 位)
//   - 节点 ID:0-1023,通过 SetNode 配置(或环境变量 GOBLACKBOX_SNOWFLAKE_NODE)
//   - 序列号:同一毫秒内 0-4095,超出则等待下一毫秒
const (
	snowflakeEpoch    int64 = 1704067200000 // 2024-01-01T00:00:00Z
	snowflakeNodeBits       = 10
	snowflakeSeqBits        = 12
	snowflakeMaxNode        = 1<<snowflakeNodeBits - 1
	snowflakeMaxSeq         = 1<<snowflakeSeqBits - 1
	// 时钟回拨等待上限:超过该值直接拒绝生成(返回错误),避免长时间阻塞。
	maxClockBackoff = 5 * time.Millisecond
)

var (
	// ErrNodeOutOfRange 节点 ID 超出 0-1023 范围。
	ErrNodeOutOfRange = fmt.Errorf("snowflake: node must be in [0, %d]", snowflakeMaxNode)
	// ErrClockBackward 时钟回拨超过等待上限,拒绝生成。
	ErrClockBackward = errors.New("snowflake: clock moved backwards beyond limit")
)

// snowflake 是并发安全的雪花生成器。
type snowflake struct {
	mu        sync.Mutex
	node      int64
	lastStamp int64
	sequence  int64
}

// defaultSnowflake 是包级默认生成器(节点 0)。
var defaultSnowflake = &snowflake{}

// SetNode 设置默认生成器的节点 ID(0-1023)。
// 业务进程启动时调用一次即可;多个服务实例必须分配不同节点 ID,
// 也可通过环境变量 GOBLACKBOX_SNOWFLAKE_NODE 在启动前配置。
func SetNode(node int64) error {
	if node < 0 || node > snowflakeMaxNode {
		return ErrNodeOutOfRange
	}
	defaultSnowflake.mu.Lock()
	defaultSnowflake.node = node
	defaultSnowflake.mu.Unlock()
	return nil
}

// NextID 生成下一个雪花 ID(int64,恒为正)。
// 使用默认节点配置;时钟回拨时短暂等待,超过上限返回 ErrClockBackward。
func NextID() (int64, error) {
	return defaultSnowflake.nextID()
}

// nextID 生成流程:毫秒时间戳 + 节点 + 序列号,含时钟回拨处理。
func (s *snowflake) nextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stamp := nowMillis()
	if stamp < s.lastStamp {
		// 时钟回拨:等待差值,超限拒绝
		backoff := s.lastStamp - stamp
		if backoff > int64(maxClockBackoff/time.Millisecond) {
			return 0, ErrClockBackward
		}
		time.Sleep(time.Duration(backoff) * time.Millisecond)
		stamp = nowMillis()
		if stamp < s.lastStamp {
			return 0, ErrClockBackward
		}
	}

	if stamp == s.lastStamp {
		s.sequence = (s.sequence + 1) & snowflakeMaxSeq
		if s.sequence == 0 {
			// 当前毫秒序列耗尽,等待下一毫秒
			for stamp <= s.lastStamp {
				stamp = nowMillis()
			}
		}
	} else {
		s.sequence = 0
	}
	s.lastStamp = stamp

	return (stamp-snowflakeEpoch)<<(snowflakeNodeBits+snowflakeSeqBits) |
		s.node<<snowflakeSeqBits |
		s.sequence, nil
}

// nowMillis 返回当前 Unix 毫秒。
func nowMillis() int64 {
	return time.Now().UnixMilli()
}

// ParseSnowflake 解析雪花 ID,返回(时间戳, 节点, 序列号, 错误)。
// 可用于日志排查与时间反推。
func ParseSnowflake(value int64) (timestamp time.Time, node int64, sequence int64, err error) {
	if value <= 0 {
		return time.Time{}, 0, 0, errors.New("snowflake: value must be positive")
	}
	rawStamp := value >> (snowflakeNodeBits + snowflakeSeqBits)
	node = (value >> snowflakeSeqBits) & snowflakeMaxNode
	sequence = value & snowflakeMaxSeq
	timestamp = time.UnixMilli(rawStamp + snowflakeEpoch)
	return timestamp, node, sequence, nil
}
