package cache

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestInitRejectsNilContext 验证空 Context 返回明确错误。
func TestInitRejectsNilContext(t *testing.T) {
	if _, err := Init(nil, RedisOptions{}); err == nil {
		t.Fatal("init with nil context must return an error")
	}
}

// TestNilCacheIsSafe 验证 nil 实例上的方法调用不会 panic，返回可识别结果。
func TestNilCacheIsSafe(t *testing.T) {
	var cacheInstance *RedisCache

	if err := cacheInstance.Close(); err != nil {
		t.Fatalf("Close on nil cache must be no-op, got: %v", err)
	}
	if err := cacheInstance.Health(context.Background()); err == nil {
		t.Fatal("Health on nil cache must return an error")
	}
	if cacheInstance.IsExists("key") {
		t.Fatal("IsExists on nil cache must return false")
	}
	if err := cacheInstance.Set("key", "value"); err == nil {
		t.Fatal("Set on nil cache must return an error")
	}
	if err := cacheInstance.Get("key", new(string)); err == nil {
		t.Fatal("Get on nil cache must return an error")
	}
	cacheInstance.SetDefaultTTL(time.Minute) // 不应 panic
}

// TestInitWithRealRedis 验证真实 Redis 的初始化、读写往返与关闭。
// 设置 GO_BLACKBOX_REDIS_ADDR（如 127.0.0.1:6379）后才会执行。
func TestInitWithRealRedis(t *testing.T) {
	addr := os.Getenv("GO_BLACKBOX_REDIS_ADDR")
	if addr == "" {
		t.Skip("Redis integration test requires GO_BLACKBOX_REDIS_ADDR environment variable")
	}

	options := RedisOptions{Addr: addr, DB: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	instance, err := Init(ctx, options)
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	if instance == nil {
		t.Fatal("init redis returned nil instance")
	}
	t.Cleanup(func() {
		if err := instance.Close(); err != nil {
			t.Errorf("close redis failed: %v", err)
		}
	})

	if err := instance.Health(ctx); err != nil {
		t.Fatalf("health check failed: %v", err)
	}

	key := "go-blackbox-test-key"
	var value string
	_ = instance.SetTtl(key, "hello", 30*time.Second)
	if err := instance.Get(key, &value); err != nil {
		t.Fatalf("get after set failed: %v", err)
	}
	if !strings.Contains(value, "hello") {
		t.Fatalf("unexpected cached value: %q", value)
	}
	if !instance.IsExists(key) {
		t.Fatal("key must exist after set")
	}
}
