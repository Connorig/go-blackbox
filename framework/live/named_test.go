package live

import (
	"sync"
	"testing"
	"time"
)

// TestSetNamedGetNamed 命名实例注册与获取。
func TestSetNamedGetNamed(t *testing.T) {
	// 避免与全局状态串扰,测试内自清理
	defer func() {
		SetGlobal(nil)
		SetNamed("cluster-a", nil)
		SetNamed("cluster-b", nil)
	}()

	defaultClient := NewClient("http://127.0.0.1:1985", 0)
	clusterA := NewClient("http://10.0.0.1:1985", 0)
	clusterB := NewClient("http://10.0.0.2:1985", 0)

	SetGlobal(defaultClient)
	SetNamed("cluster-a", clusterA)
	SetNamed("cluster-b", clusterB)

	if Get() != defaultClient {
		t.Fatal("default client mismatch")
	}
	if GetNamed("cluster-a") != clusterA {
		t.Fatal("cluster-a client mismatch")
	}
	if GetNamed("cluster-b") != clusterB {
		t.Fatal("cluster-b client mismatch")
	}
	if GetNamed("unknown") != nil {
		t.Fatal("unknown client must be nil")
	}
	// 命名实例与默认实例隔离
	if GetNamed("") != defaultClient {
		t.Fatal("empty name must resolve to default client")
	}
	// Clients 快照完整
	snapshot := Clients()
	if len(snapshot) != 3 {
		t.Fatalf("clients count = %d, want 3", len(snapshot))
	}
}

// TestSetNamedNilRemoves 传 nil 注销实例(幂等)。
func TestSetNamedNilRemoves(t *testing.T) {
	defer SetNamed("temp", nil)

	client := NewClient("http://127.0.0.1:1985", 0)
	SetNamed("temp", client)
	if GetNamed("temp") != client {
		t.Fatal("register failed")
	}
	SetNamed("temp", nil)
	if GetNamed("temp") != nil {
		t.Fatal("nil SetNamed must remove the instance")
	}
	SetNamed("temp", nil) // 幂等,不 panic
	if len(Clients()) != 0 {
		t.Fatal("registry must be empty after removal")
	}
}

// TestNamedConcurrentAccess 并发读写安全(go test -race 验证)。
func TestNamedConcurrentAccess(t *testing.T) {
	defer SetNamed("concurrent", nil)

	SetNamed("concurrent", NewClient("http://127.0.0.1:1985", time.Second))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = GetNamed("concurrent")
			_ = Clients()
		}()
		go func() {
			defer wg.Done()
			SetNamed("concurrent", NewClient("http://127.0.0.1:1985", 0))
		}()
	}
	wg.Wait()
}
