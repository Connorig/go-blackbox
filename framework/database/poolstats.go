package datasource

import (
	"time"
)

// PoolStats 数据库连接池统计(源自 database/sql 的 DB.Stats)。
// 场景:连接池监控(告警阈值:InUse 接近 MaxOpen、WaitCount 增长)。
type PoolStats struct {
	// MaxOpenConnections 最大可打开连接数(配置上限)。
	MaxOpenConnections int
	// OpenConnections 当前打开的连接总数。
	OpenConnections int
	// InUse 正在使用的连接数。
	InUse int
	// Idle 空闲连接数。
	Idle int
	// WaitCount 等待连接的总请求数(持续增长说明池偏小)。
	WaitCount int64
	// WaitDuration 等待连接的总时长。
	WaitDuration time.Duration
}

// PoolStats 返回当前连接池统计;实例未初始化或已关闭返回 nil。
func (i *Instance) PoolStats() *PoolStats {
	if i == nil || i.db == nil {
		return nil
	}
	sqlDatabase, err := i.db.DB()
	if err != nil {
		return nil
	}
	stats := sqlDatabase.Stats()
	return &PoolStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
	}
}

// PoolUtilization 返回连接池利用率(InUse / MaxOpen,0~1)。
// 持续 > 0.8 建议扩容 MaxOpenConns;实例不可用返回 -1。
func (i *Instance) PoolUtilization() float64 {
	stats := i.PoolStats()
	if stats == nil || stats.MaxOpenConnections <= 0 {
		return -1
	}
	return float64(stats.InUse) / float64(stats.MaxOpenConnections)
}
