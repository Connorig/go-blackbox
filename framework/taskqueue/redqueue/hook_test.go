package redqueue

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestDeadLetterHookFires 验证死信回调触发。
func TestDeadLetterHookFires(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "dlq-hook").WithMaxRetries(1)

	var mu sync.Mutex
	got := make([]DeadLetter, 0, 2)
	queue.WithDeadLetterHook(func(_ context.Context, letter DeadLetter) {
		mu.Lock()
		got = append(got, letter)
		mu.Unlock()
	})

	if err := queue.Submit(context.Background(), []byte("hook-task"), 0); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go func() {
		_ = queue.Consume(ctx, func(_ context.Context, _ []byte) error {
			return context.DeadlineExceeded // 永远失败
		})
	}()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(got)
		mu.Unlock()
		if count >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("expected 1 hook call, got %d", len(got))
	}
	if string(got[0].Payload) != "hook-task" {
		t.Fatalf("unexpected hook payload: %s", got[0].Payload)
	}
	if got[0].Retries != 1 {
		t.Fatalf("unexpected retries in hook: %d", got[0].Retries)
	}
	if got[0].FailedAt.IsZero() {
		t.Fatal("hook letter must carry failed_at timestamp")
	}
}

// TestDeadLetterHookNilSafe 验证未设置回调时行为不变。
func TestDeadLetterHookNilSafe(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "dlq-nohook").WithMaxRetries(1)

	if err := queue.Submit(context.Background(), []byte("nohook"), 0); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go func() {
		_ = queue.Consume(ctx, func(_ context.Context, _ []byte) error {
			return context.DeadlineExceeded
		})
	}()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		total, _ := queue.DeadLetterCount(context.Background())
		if total >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()

	total, err := queue.DeadLetterCount(context.Background())
	if err != nil {
		t.Fatalf("dead letter count failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 dead letter, got %d", total)
	}
}
