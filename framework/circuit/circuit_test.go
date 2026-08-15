package circuit

import (
	"errors"
	"sync"
	"testing"
	"time"
)

var errDown = errors.New("downstream down")

// failN 构造连续失败调用。
func failN(breaker *Breaker, n int) {
	for i := 0; i < n; i++ {
		_ = breaker.Execute(func() error { return errDown }, nil)
	}
}

// TestClosedToOpen 失败率超阈值触发熔断。
func TestClosedToOpen(t *testing.T) {
	breaker := New(Config{
		FailureThreshold: 0.5,
		MinRequests:      10,
		Window:           60 * time.Second,
		Cooldown:         30 * time.Second,
	})
	// 4 成功 + 6 失败 = 10 请求,失败率 60% >= 50%
	for i := 0; i < 4; i++ {
		if err := breaker.Execute(func() error { return nil }, nil); err != nil {
			t.Fatalf("success call failed: %v", err)
		}
	}
	failN(breaker, 6)
	if breaker.State() != StateOpen {
		t.Fatalf("state = %s, want open", breaker.State())
	}
	// 打开后快速失败,不执行 fn
	called := false
	err := breaker.Execute(func() error { called = true; return nil }, nil)
	if !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
	if called {
		t.Fatal("fn must not run while open")
	}
}

// TestBelowThreshold 失败率未达阈值不熔断。
func TestBelowThreshold(t *testing.T) {
	breaker := New(Config{FailureThreshold: 0.5, MinRequests: 10, Window: 60 * time.Second})
	for i := 0; i < 6; i++ {
		_ = breaker.Execute(func() error { return nil }, nil)
	}
	failN(breaker, 4) // 40% < 50%
	if breaker.State() != StateClosed {
		t.Fatalf("state = %s, want closed", breaker.State())
	}
}

// TestBelowMinRequests 请求数不足不触发。
func TestBelowMinRequests(t *testing.T) {
	breaker := New(Config{FailureThreshold: 0.5, MinRequests: 10, Window: 60 * time.Second})
	failN(breaker, 5) // 5 < 10
	if breaker.State() != StateClosed {
		t.Fatalf("state = %s, want closed", breaker.State())
	}
}

// TestHalfOpenRecovery 冷却后试探成功恢复 closed。
func TestHalfOpenRecovery(t *testing.T) {
	breaker := New(Config{
		FailureThreshold:    0.5,
		MinRequests:         4,
		Window:              60 * time.Second,
		Cooldown:            50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	})
	failN(breaker, 4)
	if breaker.State() != StateOpen {
		t.Fatalf("state = %s, want open", breaker.State())
	}
	time.Sleep(60 * time.Millisecond)
	// 半开:第一个试探放行
	if err := breaker.Execute(func() error { return nil }, nil); err != nil {
		t.Fatalf("half-open probe failed: %v", err)
	}
	if breaker.State() != StateClosed {
		t.Fatalf("state = %s, want closed after probe success", breaker.State())
	}
}

// TestHalfOpenProbeFailure 试探失败回到 open。
func TestHalfOpenProbeFailure(t *testing.T) {
	breaker := New(Config{
		FailureThreshold:    0.5,
		MinRequests:         4,
		Window:              60 * time.Second,
		Cooldown:            50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	})
	failN(breaker, 4)
	time.Sleep(60 * time.Millisecond)
	_ = breaker.Execute(func() error { return errDown }, nil)
	if breaker.State() != StateOpen {
		t.Fatalf("state = %s, want open after probe failure", breaker.State())
	}
}

// TestClassify 业务错误不触发熔断。
func TestClassify(t *testing.T) {
	breaker := New(Config{FailureThreshold: 0.5, MinRequests: 4, Window: 60 * time.Second})
	bizErr := errors.New("400 bad request")
	classify := func(err error) bool {
		// 只把 5xx/网络错误计失败,4xx 不计
		return err == errDown
	}
	for i := 0; i < 6; i++ {
		_ = breaker.Execute(func() error { return bizErr }, classify)
	}
	if breaker.State() != StateClosed {
		t.Fatalf("business errors must not trip breaker, state = %s", breaker.State())
	}
	// 真实失败触发
	for i := 0; i < 4; i++ {
		_ = breaker.Execute(func() error { return errDown }, classify)
	}
	if breaker.State() != StateOpen {
		t.Fatalf("state = %s, want open", breaker.State())
	}
}

// TestConcurrentSafety 并发调用不 panic、状态一致。
func TestConcurrentSafety(t *testing.T) {
	breaker := New(DefaultConfig())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%3 == 0 {
				_ = breaker.Execute(func() error { return errDown }, nil)
			} else {
				_ = breaker.Execute(func() error { return nil }, nil)
			}
		}(i)
	}
	wg.Wait()
	_ = breaker.State() // 不 panic 即通过
}
