package datasource

import (
	"context"
	"testing"
	"time"
)

// TestPoolStats 连接池统计(SQLite 内存库真实初始化)。
func TestPoolStats(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	instance, err := New(ctx, &Config{Driver: DriverSQLite, DSN: ":memory:"})
	if err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	defer instance.Close()

	stats := instance.PoolStats()
	if stats == nil {
		t.Fatal("stats must not be nil")
	}
	if stats.MaxOpenConnections <= 0 {
		t.Fatalf("max open = %d", stats.MaxOpenConnections)
	}
	utilization := instance.PoolUtilization()
	if utilization < 0 || utilization > 1 {
		t.Fatalf("utilization = %f", utilization)
	}
}

// TestPoolStatsNilSafe nil/关闭实例安全。
func TestPoolStatsNilSafe(t *testing.T) {
	var instance *Instance
	if instance.PoolStats() != nil || instance.PoolUtilization() != -1 {
		t.Fatal("nil instance must be safe")
	}
	empty := &Instance{}
	if empty.PoolStats() != nil || empty.PoolUtilization() != -1 {
		t.Fatal("empty instance must be safe")
	}
}
