package redqueue

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// testClient 连接测试 Redis;未设置环境变量时跳过。
func testClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("GO_BLACKBOX_REDIS_ADDR")
	if addr == "" {
		t.Skip("Redis integration test requires GO_BLACKBOX_REDIS_ADDR environment variable")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 0})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping redis failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// TestValidation 验证参数校验(nil 客户端/nil ctx/空 payload/nil handler)。
func TestValidation(t *testing.T) {
	var queue *Queue
	if err := queue.Submit(context.Background(), []byte("x"), 0); err == nil {
		t.Fatal("nil queue must return error")
	}
	queue = NewQueue(nil, "test")
	if err := queue.Submit(context.Background(), []byte("x"), 0); err == nil {
		t.Fatal("nil client must return error")
	}
	queue = NewQueue(testClient(t), "validation")
	if err := queue.Submit(nil, []byte("x"), 0); err == nil {
		t.Fatal("nil ctx must return error")
	}
	if err := queue.Submit(context.Background(), nil, 0); err == nil {
		t.Fatal("empty payload must return error")
	}
	if err := queue.Consume(context.Background(), nil); err == nil {
		t.Fatal("nil handler must return error")
	}
}

// TestImmediateTaskRoundTrip 验证即时任务提交与消费。
func TestImmediateTaskRoundTrip(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "immediate")

	if err := queue.Submit(context.Background(), []byte("task-1"), 0); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	pending, err := queue.Pending(context.Background())
	if err != nil {
		t.Fatalf("pending failed: %v", err)
	}
	if pending != 1 {
		t.Fatalf("expected 1 pending task, got %d", pending)
	}

	received := make(chan string, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		_ = queue.Consume(ctx, func(_ context.Context, payload []byte) error {
			received <- string(payload)
			return nil
		})
	}()
	select {
	case payload := <-received:
		if payload != "task-1" {
			t.Fatalf("unexpected payload: %s", payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for task consumption")
	}
}

// TestDelayedTaskExecutesAfterDueTime 验证延迟任务在到期后才执行。
func TestDelayedTaskExecutesAfterDueTime(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "delayed")

	if err := queue.Submit(context.Background(), []byte("delayed-1"), 1500*time.Millisecond); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	received := make(chan string, 1)
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		_ = queue.Consume(ctx, func(_ context.Context, payload []byte) error {
			received <- string(payload)
			return nil
		})
	}()
	select {
	case <-received:
		elapsed := time.Since(start)
		if elapsed < 1400*time.Millisecond {
			t.Fatalf("delayed task executed too early: %s", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for delayed task")
	}
}

// TestFailedHandlerRequeues 验证 handler 失败后任务重新入队并可再次消费。
func TestFailedHandlerRequeues(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "requeue")

	if err := queue.Submit(context.Background(), []byte("retry-me"), 0); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	attempts := 0
	var mu sync.Mutex
	received := make(chan struct{}, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go func() {
		_ = queue.Consume(ctx, func(_ context.Context, _ []byte) error {
			mu.Lock()
			attempts++
			count := attempts
			mu.Unlock()
			if count < 2 {
				return context.DeadlineExceeded // 第一次失败
			}
			received <- struct{}{}
			return nil
		})
	}()
	select {
	case <-received:
		mu.Lock()
		defer mu.Unlock()
		if attempts != 2 {
			t.Fatalf("expected 2 attempts (fail once + retry), got %d", attempts)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("timed out waiting for requeued task")
	}
}

// TestConcurrentConsumers 验证多实例(多消费者)并行消费不重复。
func TestConcurrentConsumers(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "concurrent")

	const total = 20
	for index := 0; index < total; index++ {
		payload := []byte{byte('a' + index)}
		if err := queue.Submit(context.Background(), payload, 0); err != nil {
			t.Fatalf("submit %d failed: %v", index, err)
		}
	}

	var mu sync.Mutex
	seen := make(map[byte]bool)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var waitGroup sync.WaitGroup
	for worker := 0; worker < 3; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_ = queue.Consume(ctx, func(_ context.Context, payload []byte) error {
				mu.Lock()
				if seen[payload[0]] {
					t.Errorf("duplicate delivery: %c", payload[0])
				}
				seen[payload[0]] = true
				mu.Unlock()
				return nil
			})
		}()
	}
	// 等待全部消费(消费完 pending=0 后仍阻塞,直接取消)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		pending, _ := queue.Pending(context.Background())
		mu.Lock()
		done := len(seen) >= total
		mu.Unlock()
		if pending == 0 && done {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	waitGroup.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != total {
		t.Fatalf("expected %d unique tasks consumed, got %d", total, len(seen))
	}
}
