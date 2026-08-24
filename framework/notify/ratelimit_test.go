package notify

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRateLimiterWindow 验证窗口内超限拒绝、滑动后恢复。
func TestRateLimiterWindow(t *testing.T) {
	limiter := NewRateLimiter(200*time.Millisecond, 2)
	if !limiter.Allow("sms:13800138000") {
		t.Fatal("first send must be allowed")
	}
	if !limiter.Allow("sms:13800138000") {
		t.Fatal("second send must be allowed")
	}
	if limiter.Allow("sms:13800138000") {
		t.Fatal("third send within window must be rejected")
	}
	// 不同 target 不受影响
	if !limiter.Allow("sms:13900139000") {
		t.Fatal("different target must be allowed")
	}
	// 窗口滑动后恢复
	time.Sleep(250 * time.Millisecond)
	if !limiter.Allow("sms:13800138000") {
		t.Fatal("send after window must be allowed")
	}
}

// TestRateLimiterAllowSend 验证 AllowSend 错误信息。
func TestRateLimiterAllowSend(t *testing.T) {
	limiter := NewRateLimiter(time.Minute, 1)
	if err := limiter.AllowSend(context.Background(), "sms", "13800138000", Content{}); err != nil {
		t.Fatalf("first send must pass: %v", err)
	}
	err := limiter.AllowSend(context.Background(), "sms", "13800138000", Content{})
	if err == nil {
		t.Fatal("second send must be rejected")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRateLimiterConcurrent 并发安全:总放行数不超过上限。
func TestRateLimiterConcurrent(t *testing.T) {
	limiter := NewRateLimiter(time.Minute, 10)
	var allowed int32
	var waitGroup sync.WaitGroup
	for i := 0; i < 50; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if limiter.Allow("key:1") {
				atomic.AddInt32(&allowed, 1)
			}
		}()
	}
	waitGroup.Wait()
	if allowed != 10 {
		t.Fatalf("exactly 10 must pass, got %d", allowed)
	}
}

// TestRateLimiterDisabledAndNil 频控关闭与 nil 安全。
func TestRateLimiterDisabledAndNil(t *testing.T) {
	disabled := NewRateLimiter(0, 0)
	if !disabled.Allow("any") {
		t.Fatal("disabled limiter must allow everything")
	}
	var nilLimiter *RateLimiter
	if !nilLimiter.Allow("any") {
		t.Fatal("nil limiter must allow everything")
	}
	if err := nilLimiter.AllowSend(context.Background(), "sms", "t", Content{}); err != nil {
		t.Fatalf("nil limiter AllowSend must pass: %v", err)
	}
	nilLimiter.Clean() // 不 panic
	disabled.Clean()   // 不 panic
}

// TestRateLimiterClean 验证 Clean 清理过期 key。
func TestRateLimiterClean(t *testing.T) {
	limiter := NewRateLimiter(100*time.Millisecond, 5)
	_ = limiter.Allow("sms:1")
	time.Sleep(150 * time.Millisecond)
	limiter.Clean()
	limiter.mu.Lock()
	remaining := len(limiter.hits)
	limiter.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("clean must remove expired keys, got %d", remaining)
	}
}

