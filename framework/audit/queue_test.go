package oplog

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

// memorySink 是测试用内存 Sink。
type memorySink struct {
	mu      sync.Mutex
	entries []Entry
}

// Write 追加条目。
func (s *memorySink) Write(_ context.Context, entries []Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entries...)
	return nil
}

// count 返回已写入条目数。
func (s *memorySink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// TestQueueFlushesAllEntries 验证 Close 后已入队条目全部写入 Sink（不丢失）。
func TestQueueFlushesAllEntries(t *testing.T) {
	sink := &memorySink{}
	queue := NewQueue(sink, 16)
	if queue == nil {
		t.Fatal("queue must not be nil for valid sink")
	}

	const total = 10 // 小于队列容量，全部入队
	for index := 0; index < total; index++ {
		queue.Enqueue(Entry{Path: "/test", Status: 200})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := queue.Close(ctx); err != nil {
		t.Fatalf("close queue failed: %v", err)
	}
	if got := sink.count(); got != total {
		t.Fatalf("expected %d entries flushed, got %d", total, got)
	}
	if queue.Dropped() != 0 {
		t.Fatalf("unexpected dropped count: %d", queue.Dropped())
	}
}

// TestQueueDropsWhenFull 验证队列满时丢弃并计数（不阻塞调用方）。
func TestQueueDropsWhenFull(t *testing.T) {
	sink := &memorySink{}
	queue := NewQueue(sink, 4)

	// 填满队列;消费者 100ms 刷新,连续入队会触发丢弃
	for index := 0; index < 100; index++ {
		queue.Enqueue(Entry{Path: "/burst"})
	}
	if queue.Dropped() == 0 {
		t.Fatal("expected dropped entries when queue is full")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := queue.Close(ctx); err != nil {
		t.Fatalf("close queue failed: %v", err)
	}
	// 至少部分条目成功写入
	if sink.count() == 0 {
		t.Fatal("sink must receive at least some entries")
	}
}

// TestQueueCloseIdempotent 验证重复 Close 安全。
func TestQueueCloseIdempotent(t *testing.T) {
	queue := NewQueue(&memorySink{}, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := queue.Close(ctx); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := queue.Close(ctx); err != nil {
		t.Fatalf("second close must be idempotent: %v", err)
	}
}

// TestMiddlewareRecordsRequests 验证中间件把请求信息入队。
func TestMiddlewareRecordsRequests(t *testing.T) {
	sink := &memorySink{}
	queue := NewQueue(sink, 32)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = queue.Close(ctx)
	}()

	app := iris.New()
	app.Use(queue.Middleware())
	app.Get("/orders", func(ctx iris.Context) {
		ctx.StatusCode(200)
		ctx.WriteString("ok")
	})

	e := httptest.New(t, app)
	e.GET("/orders").Expect().Status(200)

	// 等待消费者写入
	deadline := time.Now().Add(3 * time.Second)
	for sink.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if sink.count() == 0 {
		t.Fatal("middleware must enqueue request entries")
	}
	sink.mu.Lock()
	entry := sink.entries[0]
	sink.mu.Unlock()
	if entry.Method != "GET" || entry.Path != "/orders" || entry.Status != 200 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

// TestMiddlewareWithoutQueueIsNoop 验证 nil 队列中间件不 panic。
func TestMiddlewareWithoutQueueIsNoop(t *testing.T) {
	var queue *Queue
	handler := queue.Middleware()
	app := iris.New()
	app.Use(handler)
	app.Get("/ok", func(ctx iris.Context) { ctx.WriteString("ok") })

	e := httptest.New(t, app)
	e.GET("/ok").Expect().Status(200)
}

// TestFailedSinkCounts 验证 Sink 写入失败计入失败计数。
func TestFailedSinkCounts(t *testing.T) {
	var failed int64
	failingSink := SinkFunc(func(_ context.Context, entries []Entry) error {
		atomic.AddInt64(&failed, int64(len(entries)))
		return context.DeadlineExceeded
	})
	queue := NewQueue(failingSink, 8)
	for index := 0; index < 20; index++ {
		queue.Enqueue(Entry{Path: "/fail"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = queue.Close(ctx)
	if queue.Failed() == 0 {
		t.Fatal("failed sink must be counted")
	}
}

// SinkFunc 是 Sink 接口的函数适配器。
type SinkFunc func(ctx context.Context, entries []Entry) error

// Write 调用函数实现。
func (f SinkFunc) Write(ctx context.Context, entries []Entry) error {
	return f(ctx, entries)
}
