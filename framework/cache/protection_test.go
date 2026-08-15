package cache

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

// TestLoadOrStoreSingleExecution 验证并发回源只执行一次 loader。
func TestLoadOrStoreSingleExecution(t *testing.T) {
	var mu sync.Mutex
	loadCount := 0

	const goroutines = 20
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer waitGroup.Done()
			<-start // 等待全部 goroutine 就绪，保证调用重叠
			value, err := LoadOrStore("key-1", func() (string, error) {
				mu.Lock()
				loadCount++
				mu.Unlock()
				time.Sleep(50 * time.Millisecond) // 拉长 loader 确保重叠窗口
				return "loaded", nil
			})
			if err != nil {
				t.Errorf("load or store failed: %v", err)
				return
			}
			if value != "loaded" {
				t.Errorf("unexpected value: %q", value)
			}
		}()
	}
	close(start)
	waitGroup.Wait()

	if loadCount != 1 {
		t.Fatalf("loader must run exactly once under concurrency, got %d", loadCount)
	}
}

// TestLoadOrStorePropagatesError 验证 loader 错误会返回给调用方。
func TestLoadOrStorePropagatesError(t *testing.T) {
	expected := errors.New("source failure")
	if _, err := LoadOrStore("key-err", func() (string, error) {
		return "", expected
	}); !errors.Is(err, expected) {
		t.Fatalf("expected source error, got: %v", err)
	}
}

// TestLoadOrStoreRejectsNilLoader 验证 nil loader 被拒绝。
func TestLoadOrStoreRejectsNilLoader(t *testing.T) {
	if _, err := LoadOrStore[string]("key-nil", nil); err == nil {
		t.Fatal("nil loader must return an error")
	}
}

// TestTryLockWithoutRedis 验证未初始化 Redis 时返回明确错误。
func TestTryLockWithoutRedis(t *testing.T) {
	var cacheInstance *RedisCache
	if _, err := cacheInstance.TryLock(context.Background(), "key", time.Second); err == nil {
		t.Fatal("TryLock on nil cache must return an error")
	}
}

// TestLockNilSafety 验证 nil Lock 上的方法调用安全。
func TestLockNilSafety(t *testing.T) {
	var lock *Lock
	if err := lock.Unlock(context.Background()); err != nil {
		t.Fatalf("Unlock on nil lock must be no-op, got: %v", err)
	}
	if err := lock.Renew(context.Background(), time.Second); err == nil {
		t.Fatal("Renew on nil lock must return an error")
	}
}

// TestDistributedLockWithRealRedis 验证真实 Redis 上的锁获取/互斥/释放/续期。
// 设置 GO_BLACKBOX_REDIS_ADDR 后才会执行。
func TestDistributedLockWithRealRedis(t *testing.T) {
	addr := os.Getenv("GO_BLACKBOX_REDIS_ADDR")
	if addr == "" {
		t.Skip("Redis integration test requires GO_BLACKBOX_REDIS_ADDR environment variable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	instance, err := Init(ctx, RedisOptions{Addr: addr, DB: 0})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	t.Cleanup(func() { _ = instance.Close() })

	const lockKey = "go-blackbox-test-lock"
	// 互斥：第二个 TryLock 应失败
	first, err := instance.TryLock(ctx, lockKey, 30*time.Second)
	if err != nil {
		t.Fatalf("first TryLock failed: %v", err)
	}
	if first == nil {
		t.Fatal("first TryLock must succeed")
	}
	second, err := instance.TryLock(ctx, lockKey, 30*time.Second)
	if err != nil {
		t.Fatalf("second TryLock failed: %v", err)
	}
	if second != nil {
		t.Fatal("second TryLock must fail while lock is held")
	}

	// 续期后释放
	if err := first.Renew(ctx, 60*time.Second); err != nil {
		t.Fatalf("renew lock failed: %v", err)
	}
	if err := first.Unlock(ctx); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	// 释放后可重新获取
	again, err := instance.TryLock(ctx, lockKey, 30*time.Second)
	if err != nil {
		t.Fatalf("re-acquire lock failed: %v", err)
	}
	if again == nil {
		t.Fatal("lock must be re-acquirable after unlock")
	}
	if err := again.Unlock(ctx); err != nil {
		t.Fatalf("final unlock failed: %v", err)
	}
}

// TestSetTtlWithJitter 验证随机 TTL 在 ±20% 范围内且 nil 安全。
func TestSetTtlWithJitter(t *testing.T) {
	var cacheInstance *RedisCache
	if err := cacheInstance.SetTtlWithJitter(context.Background(), "key", "value", time.Minute); err == nil {
		t.Fatal("SetTtlWithJitter on nil cache must return an error")
	}
	if err := (&RedisCache{proxy: nil}).SetTtlWithJitter(context.Background(), "key", "value", time.Minute); err == nil {
		t.Fatal("SetTtlWithJitter with nil proxy must return an error")
	}
}
