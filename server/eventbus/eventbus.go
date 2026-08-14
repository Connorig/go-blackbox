// Package eventbus 提供进程内事件总线（发布/订阅）。
// 支持同步与异步两种投递模式；异步模式下发布不阻塞调用方。
// 典型用途：业务模块解耦（订单创建后通知库存/通知/审计）、进程内观察者模式。
package eventbus

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Event 是事件负载。
type Event struct {
	// Name 是事件名（如 order.created），订阅者按名称匹配。
	Name string
	// Data 是事件附带数据。
	Data interface{}
}

// Handler 是事件处理函数；返回错误会中止同事件后续订阅者（同步模式）。
type Handler func(ctx context.Context, event Event) error

// handlerEntry 保存单个订阅。
type handlerEntry struct {
	handler Handler
}

// Bus 是进程内事件总线。
type Bus struct {
	async bool

	mu       sync.RWMutex
	handlers map[string][]handlerEntry
	all      []handlerEntry // 通配订阅（SubscribeAll）
}

// New 创建事件总线；async 为 true 时 Publish 异步投递（每个订阅者独立 goroutine）。
func New(async bool) *Bus {
	return &Bus{
		async:    async,
		handlers: make(map[string][]handlerEntry),
	}
}

// Subscribe 订阅指定事件；返回取消订阅函数（幂等）。
func (b *Bus) Subscribe(event string, handler Handler) func() {
	if b == nil || handler == nil {
		return func() {}
	}
	b.mu.Lock()
	b.handlers[event] = append(b.handlers[event], handlerEntry{handler: handler})
	b.mu.Unlock()

	return func() {
		b.unsubscribe(event, handler)
	}
}

// SubscribeAll 订阅全部事件（用于日志、审计等横切关注点）。
func (b *Bus) SubscribeAll(handler Handler) func() {
	if b == nil || handler == nil {
		return func() {}
	}
	b.mu.Lock()
	b.all = append(b.all, handlerEntry{handler: handler})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for index, entry := range b.all {
			if sameHandler(entry.handler, handler) {
				b.all = append(b.all[:index], b.all[index+1:]...)
				break
			}
		}
	}
}

// Publish 发布事件。
// 同步模式：按订阅顺序执行，任一订阅者返回错误即停止并返回该错误。
// 异步模式：每个订阅者在独立 goroutine 中执行，错误仅记录到日志输出（不返回）。
func (b *Bus) Publish(ctx context.Context, event Event) error {
	if b == nil {
		return errors.New("eventbus: bus is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if event.Name == "" {
		return errors.New("eventbus: event name is empty")
	}

	entries := b.entriesFor(event.Name)
	if b.async {
		for _, entry := range entries {
			go func(handler Handler) {
				if err := handler(ctx, event); err != nil {
					fmt.Printf("eventbus async handler failed, event=%s error=%v\n", event.Name, err)
				}
			}(entry.handler)
		}
		return nil
	}

	for _, entry := range entries {
		if err := entry.handler(ctx, event); err != nil {
			return fmt.Errorf("eventbus handler failed for event %q: %w", event.Name, err)
		}
	}
	return nil
}

// entriesFor 返回事件订阅者（通配订阅排在最前）。
func (b *Bus) entriesFor(event string) []handlerEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	entries := make([]handlerEntry, 0, len(b.all)+len(b.handlers[event]))
	entries = append(entries, b.all...)
	entries = append(entries, b.handlers[event]...)
	return entries
}

// unsubscribe 移除指定事件的订阅。
func (b *Bus) unsubscribe(event string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	entries := b.handlers[event]
	for index, entry := range entries {
		if sameHandler(entry.handler, handler) {
			b.handlers[event] = append(entries[:index], entries[index+1:]...)
			return
		}
	}
}

// sameHandler 比较处理函数是否指向同一代码位置。
func sameHandler(left, right Handler) bool {
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}
