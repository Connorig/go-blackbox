package eventbus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSyncPublishRunsInOrder 验证同步模式按订阅顺序执行。
func TestSyncPublishRunsInOrder(t *testing.T) {
	bus := New(false)
	var order []string
	var mu sync.Mutex

	bus.Subscribe("order.created", func(_ context.Context, _ Event) error {
		mu.Lock()
		order = append(order, "first")
		mu.Unlock()
		return nil
	})
	bus.Subscribe("order.created", func(_ context.Context, _ Event) error {
		mu.Lock()
		order = append(order, "second")
		mu.Unlock()
		return nil
	})

	if err := bus.Publish(context.Background(), Event{Name: "order.created"}); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("handlers must run in subscription order: %v", order)
	}
}

// TestSyncPublishStopsOnError 验证同步模式错误停止后续订阅者。
func TestSyncPublishStopsOnError(t *testing.T) {
	bus := New(false)
	nextCalled := false
	expected := errors.New("handler failed")

	bus.Subscribe("event", func(context.Context, Event) error { return expected })
	bus.Subscribe("event", func(context.Context, Event) error {
		nextCalled = true
		return nil
	})

	err := bus.Publish(context.Background(), Event{Name: "event"})
	if !errors.Is(err, expected) {
		t.Fatalf("expected handler error, got: %v", err)
	}
	if nextCalled {
		t.Fatal("subsequent handler must not run after error")
	}
}

// TestAsyncPublishDoesNotBlock 验证异步模式发布立即返回。
func TestAsyncPublishDoesNotBlock(t *testing.T) {
	bus := New(true)
	var ran int32
	started := make(chan struct{})
	bus.Subscribe("slow", func(_ context.Context, _ Event) error {
		close(started)
		time.Sleep(200 * time.Millisecond)
		atomic.AddInt32(&ran, 1)
		return nil
	})

	publishDone := make(chan struct{})
	go func() {
		if err := bus.Publish(context.Background(), Event{Name: "slow"}); err != nil {
			t.Errorf("async publish failed: %v", err)
		}
		close(publishDone)
	}()

	// 发布应立即返回（不等 handler 完成）
	select {
	case <-publishDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("async publish must return immediately")
	}
	// handler 在后台完成
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("async handler did not start")
	}
	deadline := time.Now().Add(time.Second)
	for atomic.LoadInt32(&ran) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&ran) != 1 {
		t.Fatal("async handler must eventually run")
	}
}

// TestSubscribeAllReceivesEveryEvent 验证通配订阅收到全部事件。
func TestSubscribeAllReceivesEveryEvent(t *testing.T) {
	bus := New(false)
	var received []string
	var mu sync.Mutex

	bus.SubscribeAll(func(_ context.Context, event Event) error {
		mu.Lock()
		received = append(received, event.Name)
		mu.Unlock()
		return nil
	})

	_ = bus.Publish(context.Background(), Event{Name: "a"})
	_ = bus.Publish(context.Background(), Event{Name: "b"})
	if len(received) != 2 || received[0] != "a" || received[1] != "b" {
		t.Fatalf("wildcard subscriber must receive all events: %v", received)
	}
}

// TestUnsubscribeStopsDelivery 验证退订后不再收到事件。
func TestUnsubscribeStopsDelivery(t *testing.T) {
	bus := New(false)
	calls := 0
	handler := func(context.Context, Event) error {
		calls++
		return nil
	}
	unsubscribe := bus.Subscribe("event", handler)
	_ = bus.Publish(context.Background(), Event{Name: "event"})
	unsubscribe()
	unsubscribe() // 幂等
	_ = bus.Publish(context.Background(), Event{Name: "event"})

	if calls != 1 {
		t.Fatalf("unsubscribed handler must not run again, calls=%d", calls)
	}
}

// TestPublishValidation 验证 nil 总线与空事件名。
func TestPublishValidation(t *testing.T) {
	var bus *Bus
	if err := bus.Publish(context.Background(), Event{Name: "x"}); err == nil {
		t.Fatal("publish on nil bus must return an error")
	}
	bus = New(false)
	if err := bus.Publish(context.Background(), Event{Name: ""}); err == nil {
		t.Fatal("empty event name must return an error")
	}
	if err := bus.Publish(nil, Event{Name: "x"}); err != nil {
		t.Fatalf("nil context must be tolerated: %v", err)
	}
}
