package taskqueue

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestSubmitImmediate 立即执行任务。
func TestSubmitImmediate(t *testing.T) {
	queue := NewQueue(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go queue.Run(ctx)

	done := make(chan struct{})
	if _, err := queue.Submit(0, func() { close(done) }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("immediate task not executed")
	}
}

// TestSubmitDelay 延迟执行顺序与时长。
func TestSubmitDelay(t *testing.T) {
	queue := NewQueue(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go queue.Run(ctx)

	var order []string
	var mu sync.Mutex
	record := func(name string) func() {
		return func() {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}
	_, _ = queue.Submit(150*time.Millisecond, record("slow"))
	_, _ = queue.Submit(20*time.Millisecond, record("fast"))

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		count := len(order)
		mu.Unlock()
		if count == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != "fast" || order[1] != "slow" {
		t.Fatalf("order = %v", order)
	}
}

// TestPanicCaptured panic 被捕获并回调。
func TestPanicCaptured(t *testing.T) {
	queue := NewQueue(1)
	var panicErr error
	var mu sync.Mutex
	queue.OnError(func(id string, err error) {
		mu.Lock()
		panicErr = err
		mu.Unlock()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go queue.Run(ctx)

	_, _ = queue.Submit(0, func() { panic("boom") })
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		got := panicErr != nil
		mu.Unlock()
		if got || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if panicErr == nil {
		t.Fatal("panic must be captured")
	}
}

// TestNilSafe nil 安全。
func TestNilSafe(t *testing.T) {
	var queue *Queue
	if _, err := queue.Submit(0, func() {}); err == nil {
		t.Fatal("nil queue must fail")
	}
	if queue.Pending() != 0 {
		t.Fatal("nil pending must be 0")
	}
	queue.Run(context.Background()) // 不阻塞不 panic
}

// TestNilFn fn 校验。
func TestNilFn(t *testing.T) {
	queue := NewQueue(1)
	if _, err := queue.Submit(0, nil); err == nil {
		t.Fatal("nil fn must fail")
	}
}

// TestPending 等待任务计数。
func TestPending(t *testing.T) {
	queue := NewQueue(1)
	_, _ = queue.Submit(time.Hour, func() {})
	_, _ = queue.Submit(time.Hour, func() {})
	if queue.Pending() != 2 {
		t.Fatalf("pending = %d", queue.Pending())
	}
}
