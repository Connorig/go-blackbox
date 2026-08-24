package redqueue

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestMaxRetriesMovesToDeadLetter 验证超过重试上限后进入死信队列。
func TestMaxRetriesMovesToDeadLetter(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "dlq-test").WithMaxRetries(2)

	if err := queue.Submit(context.Background(), []byte("doomed"), 0); err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	var attempts int
	var mu sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = queue.Consume(ctx, func(_ context.Context, _ []byte) error {
			mu.Lock()
			attempts++
			mu.Unlock()
			return context.DeadlineExceeded // 永远失败
		})
		close(done)
	}()

	// 等待进入死信
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		total, _ := queue.DeadLetterCount(context.Background())
		if total >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	<-done

	total, err := queue.DeadLetterCount(context.Background())
	if err != nil {
		t.Fatalf("dead letter count failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 dead letter, got %d", total)
	}
	mu.Lock()
	expectedAttempts := queue.MaxRetries() + 1 // 初始 1 次 + 重试 2 次
	if attempts != expectedAttempts {
		t.Fatalf("expected %d attempts before dead letter, got %d", expectedAttempts, attempts)
	}
	mu.Unlock()
}

// TestDeadLetterQueryAndRequeue 验证死信查询与重投。
func TestDeadLetterQueryAndRequeue(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "dlq-req").WithMaxRetries(1)

	if err := queue.Submit(context.Background(), []byte("retry-once"), 0); err != nil {
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
		total, _ := queue.DeadLetterCount(context.Background())
		if total >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	letters, err := queue.DeadLetters(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("dead letters query failed: %v", err)
	}
	if len(letters) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(letters))
	}
	if string(letters[0].Payload) != "retry-once" {
		t.Fatalf("unexpected dead letter payload: %s", letters[0].Payload)
	}
	if letters[0].Retries != 1 {
		t.Fatalf("unexpected retries in dead letter: %d", letters[0].Retries)
	}

	// 重投:进入即时队列,再次消费会再次失败进死信
	if err := queue.RequeueDeadLetter(context.Background(), 0); err != nil {
		t.Fatalf("requeue dead letter failed: %v", err)
	}
	total, err := queue.DeadLetterCount(context.Background())
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if total != 0 {
		t.Fatalf("dead letter must be removed after requeue, got %d", total)
	}
	pending, err := queue.Pending(context.Background())
	if err != nil {
		t.Fatalf("pending failed: %v", err)
	}
	if pending != 1 {
		t.Fatalf("requeued task must be pending, got %d", pending)
	}
}

// TestLegacyBarePayloadCompatibility 验证旧格式裸 payload 兼容消费。
func TestLegacyBarePayloadCompatibility(t *testing.T) {
	client := testClient(t)
	const key = "legacy-bare"
	_ = client.Del(context.Background(), key+":list", key+":zset", key+":dead")
	// 直接写入裸 payload(模拟旧版本数据)
	if err := client.LPush(context.Background(), key+":list", []byte("legacy-task")).Err(); err != nil {
		t.Fatalf("push legacy payload failed: %v", err)
	}

	queue := NewQueue(client, key)
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
		if payload != "legacy-task" {
			t.Fatalf("unexpected legacy payload: %s", payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for legacy task")
	}
}
