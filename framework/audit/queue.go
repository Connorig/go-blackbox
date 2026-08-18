// Package oplog 提供异步操作日志队列：
// 请求/业务动作日志先入内存队列，由后台消费者批量写入 Sink，
// 不阻塞请求链路（go-admin 内存队列思路）。
package oplog

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kataras/iris/v12"
)

// Entry 是单条操作日志。
type Entry struct {
	Time      time.Time `json:"time"`
	UserID    int64 `json:"user_id"`
	UserEmail string `json:"user_email"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int `json:"status"`
	Duration  time.Duration `json:"duration"`
	RequestID string `json:"request_id"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
}

// Sink 是操作日志的落库目标；业务方实现（写数据库、ES、文件等）。
type Sink interface {
	// Write 批量写入条目；返回错误时该批条目被丢弃并计入失败计数。
	Write(ctx context.Context, entries []Entry) error
}

// Queue 是异步操作日志队列。
// Enqueue 非阻塞：队列满时丢弃新条目并计数，保证请求链路不被日志拖慢。
type Queue struct {
	ch      chan Entry
	sink    Sink
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	dropped int64
	failed  int64
	mu      sync.Mutex
	closed  bool
}

// NewQueue 创建异步队列并启动后台消费者。
// bufferSize 非正数时使用默认 1024。
func NewQueue(sink Sink, bufferSize int) *Queue {
	if sink == nil {
		return nil
	}
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	ctx, cancel := context.WithCancel(context.Background())
	queue := &Queue{
		ch:     make(chan Entry, bufferSize),
		sink:   sink,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go queue.consume()
	return queue
}

// Enqueue 非阻塞入队；队列已满或已关闭时丢弃并计数。
func (q *Queue) Enqueue(entry Entry) {
	if q == nil {
		return
	}
	q.mu.Lock()
	closed := q.closed
	q.mu.Unlock()
	if closed {
		atomic.AddInt64(&q.dropped, 1)
		return
	}
	select {
	case q.ch <- entry:
	default:
		atomic.AddInt64(&q.dropped, 1)
	}
}

// Close 停止接收新条目并等待消费者排空队列（幂等）。
func (q *Queue) Close(ctx context.Context) error {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	close(q.ch)
	q.mu.Unlock()

	select {
	case <-q.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dropped 返回因队列满或关闭而被丢弃的条目数。
func (q *Queue) Dropped() int64 {
	if q == nil {
		return 0
	}
	return atomic.LoadInt64(&q.dropped)
}

// Failed 返回写入 Sink 失败的批次条目数。
func (q *Queue) Failed() int64 {
	if q == nil {
		return 0
	}
	return atomic.LoadInt64(&q.failed)
}

// consume 批量消费队列并写入 Sink。
func (q *Queue) consume() {
	defer close(q.done)
	const maxBatch = 100
	batch := make([]Entry, 0, maxBatch)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := q.sink.Write(q.ctx, batch); err != nil {
			atomic.AddInt64(&q.failed, int64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-q.ctx.Done():
			return
		case entry, ok := <-q.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= maxBatch {
				flush()
			}
		case <-time.After(100 * time.Millisecond):
			flush()
		}
	}
}

// Middleware 返回 Iris 操作日志中间件，把请求信息入队。
// 注册顺序建议在 Auth 之后（可读取用户身份）。
func (q *Queue) Middleware() iris.Handler {
	if q == nil {
		return func(ctx iris.Context) { ctx.Next() }
	}
	return func(ctx iris.Context) {
		start := time.Now()
		ctx.Next()
		var userID int64
		if value, ok := ctx.Values().Get("user_id").(int64); ok {
			userID = value
		}
		q.Enqueue(Entry{
			Time:      start,
			UserID:    userID,
			UserEmail: ctx.Values().GetString("user_email"),
			Method:    ctx.Method(),
			Path:      ctx.Path(),
			Status:    ctx.GetStatusCode(),
			Duration:  time.Since(start),
			RequestID: ctx.Values().GetString("request_id"),
		})
	}
}
