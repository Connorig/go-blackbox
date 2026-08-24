package eventbus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestSubscribeRetrySuccessOnFirstTry 首试成功不重试。
func TestSubscribeRetrySuccessOnFirstTry(t *testing.T) {
	bus := New(false)
	var calls int
	bus.SubscribeRetry("order.created", func(ctx context.Context, event Event) error {
		calls++
		return nil
	}, 3, time.Millisecond)

	if err := bus.Publish(context.Background(), Event{Name: "order.created"}); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("success on first try must not retry, calls=%d", calls)
	}
}

// TestSubscribeRetryRecovers 失败后重试成功。
func TestSubscribeRetryRecovers(t *testing.T) {
	bus := New(false)
	var calls int
	bus.SubscribeRetry("order.paid", func(ctx context.Context, event Event) error {
		calls++
		if calls < 3 {
			return errors.New("transient failure")
		}
		return nil
	}, 5, time.Millisecond)

	if err := bus.Publish(context.Background(), Event{Name: "order.paid"}); err != nil {
		t.Fatalf("publish failed after retry: %v", err)
	}
	if calls != 3 {
		t.Fatalf("must recover on 3rd attempt, calls=%d", calls)
	}
}

// TestSubscribeRetryExhausted 全部失败返回最后一次错误。
func TestSubscribeRetryExhausted(t *testing.T) {
	bus := New(false)
	var calls int
	const maxRetries = 2
	bus.SubscribeRetry("order.fail", func(ctx context.Context, event Event) error {
		calls++
		return errors.New("always fails")
	}, maxRetries, time.Millisecond)

	err := bus.Publish(context.Background(), Event{Name: "order.fail"})
	if err == nil {
		t.Fatal("publish must return error after retries exhausted")
	}
	if calls != maxRetries+1 {
		t.Fatalf("total attempts must be maxRetries+1=%d, got %d", maxRetries+1, calls)
	}
}

// TestSubscribeRetryContextCancel 重试等待期间 ctx 取消立即返回。
func TestSubscribeRetryContextCancel(t *testing.T) {
	bus := New(false)
	bus.SubscribeRetry("order.slow", func(ctx context.Context, event Event) error {
		return errors.New("fails forever")
	}, 10, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := bus.Publish(ctx, Event{Name: "order.slow"})
	if err == nil {
		t.Fatal("must return error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("must return context error, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("ctx cancel must interrupt retry backoff promptly")
	}
}

// TestSubscribeRetryUnsubscribeAndNil 取消订阅后不再触发;nil 安全;参数边界。
func TestSubscribeRetryUnsubscribeAndNil(t *testing.T) {
	bus := New(false)
	var calls int
	unsubscribe := bus.SubscribeRetry("order.x", func(ctx context.Context, event Event) error {
		calls++
		return errors.New("fails")
	}, 0, 0) // maxRetries 0 = 仅一次;backoff 0 = 默认
	unsubscribe()
	_ = bus.Publish(context.Background(), Event{Name: "order.x"})
	if calls != 0 {
		t.Fatalf("unsubscribed handler must not run, calls=%d", calls)
	}

	// nil 安全
	var nilBus *Bus
	if fn := nilBus.SubscribeRetry("e", func(ctx context.Context, event Event) error { return nil }, 1, time.Millisecond); fn == nil {
		t.Fatal("nil bus must return noop unsubscribe")
	} else {
		fn()
	}
	bus.SubscribeRetry("e", nil, 1, time.Millisecond) // nil handler 不 panic

	// 并发发布安全
	var waitGroup sync.WaitGroup
	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_ = bus.Publish(context.Background(), Event{Name: "other"})
		}()
	}
	waitGroup.Wait()
}
