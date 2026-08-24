package redqueue

import (
	"context"
	"testing"
	"time"
)

// TestHandlerPanicMovesToDeadLetter 验证 handler panic 转错误并进死信(不崩溃进程)。
func TestHandlerPanicMovesToDeadLetter(t *testing.T) {
	client := testClient(t)
	queue := NewQueue(client, "dlq-panic").WithMaxRetries(1)

	if err := queue.Submit(context.Background(), []byte("panic-task"), 0); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go func() {
		_ = queue.Consume(ctx, func(_ context.Context, _ []byte) error {
			panic("business panic in handler")
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
		t.Fatalf("panic handler must move task to dead letter, got %d", total)
	}
	letters, err := queue.DeadLetters(context.Background(), 0, 5)
	if err != nil {
		t.Fatalf("dead letters failed: %v", err)
	}
	if len(letters) != 1 || string(letters[0].Payload) != "panic-task" {
		t.Fatalf("unexpected dead letters: %+v", letters)
	}
}
