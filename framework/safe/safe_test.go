package safe

import (
	"sync"
	"testing"
	"time"
)

// TestGoPanicDoesNotCrash 验证 panic 被捕获,goroutine 不崩溃进程。
func TestGoPanicDoesNotCrash(t *testing.T) {
	done := make(chan struct{})
	Go("test-panic", func() {
		panic("boom")
	})
	Go("test-ok", func() {
		close(done)
	})
	select {
	case <-done:
		// 第二个 goroutine 正常执行完成,证明第一个的 panic 未影响进程
	case <-time.After(3 * time.Second):
		t.Fatal("normal goroutine did not complete")
	}
}

// TestGoNilSafe 验证 nil 函数安全。
func TestGoNilSafe(t *testing.T) {
	Go("nil-fn", nil) // 不 panic
}

// TestRecoverConcurrent 并发触发 panic,验证全部被恢复。
func TestRecoverConcurrent(t *testing.T) {
	var waitGroup sync.WaitGroup
	for i := 0; i < 20; i++ {
		waitGroup.Add(1)
		Go("concurrent", func() {
			defer waitGroup.Done()
			panic("concurrent boom")
		})
	}
	waitGroup.Wait() // 若未恢复,进程已崩溃
}
