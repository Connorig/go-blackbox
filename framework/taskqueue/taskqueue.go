// Package taskqueue 提供进程内异步任务队列:延迟执行、并发控制、优雅退出。
// 场景:抽奖开奖延迟、定时下播、消息撤回、异步通知。
// 注意:进程内队列不持久化(重启丢失);需持久化/分布式场景用 cron 模块或 MQ。
package taskqueue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Task 队列任务。
type task struct {
	id      string
	runAt   time.Time
	execute func()
}

// Queue 异步任务队列(延迟任务按时间排序执行)。
type Queue struct {
	mu       sync.Mutex
	pending  []*task
	wake     chan struct{}
	workers  int
	running  int32
	nextID   atomic.Uint64
	onError  func(taskID string, err error)
}

// NewQueue 创建队列。workers 为并发执行数(默认 2)。
func NewQueue(workers int) *Queue {
	if workers <= 0 {
		workers = 2
	}
	return &Queue{workers: workers, wake: make(chan struct{}, 1)}
}

// Submit 提交任务:delay 为延迟执行时长(0 = 立即);返回任务 ID。
// fn 在独立 goroutine 执行,panic 会被捕获并回调 OnError。
func (q *Queue) Submit(delay time.Duration, fn func()) (string, error) {
	if q == nil {
		return "", errors.New("task queue is nil")
	}
	if fn == nil {
		return "", errors.New("task fn is nil")
	}
	if delay < 0 {
		delay = 0
	}
	id := fmt.Sprintf("task-%d", q.nextID.Add(1))
	q.mu.Lock()
	q.pending = append(q.pending, &task{id: id, runAt: time.Now().Add(delay), execute: fn})
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return id, nil
}

// OnError 注册任务 panic 回调(默认静默忽略)。
func (q *Queue) OnError(fn func(taskID string, err error)) *Queue {
	if q != nil {
		q.onError = fn
	}
	return q
}

// Pending 返回等待中的任务数(不含执行中)。
func (q *Queue) Pending() int {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

// Run 启动调度循环,阻塞直到 ctx 取消。
func (q *Queue) Run(ctx context.Context) {
	if q == nil {
		return
	}
	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-q.wake:
			q.dispatchDue(&wg)
		case <-time.After(200 * time.Millisecond):
			q.dispatchDue(&wg)
		}
	}
}

// dispatchDue 派发到期的任务。
func (q *Queue) dispatchDue(wg *sync.WaitGroup) {
	now := time.Now()
	for {
		t := q.popDue(now)
		if t == nil {
			return
		}
		// 并发限制:执行中任务超过 workers 时阻塞等待(简单信号量)
		for int(q.running) >= q.workers {
			time.Sleep(5 * time.Millisecond)
		}
		atomic.AddInt32(&q.running, 1)
		wg.Add(1)
		go func(task *task) {
			defer wg.Done()
			defer atomic.AddInt32(&q.running, -1)
			q.executeTask(task)
		}(t)
	}
}

// popDue 取出最早到期(runAt 最小)的任务(锁内操作)。
func (q *Queue) popDue(now time.Time) *task {
	q.mu.Lock()
	defer q.mu.Unlock()
	earliest := -1
	for i, t := range q.pending {
		if t.runAt.After(now) {
			continue
		}
		if earliest < 0 || t.runAt.Before(q.pending[earliest].runAt) {
			earliest = i
		}
	}
	if earliest < 0 {
		return nil
	}
	t := q.pending[earliest]
	q.pending = append(q.pending[:earliest], q.pending[earliest+1:]...)
	return t
}

// executeTask 执行任务并捕获 panic。
func (q *Queue) executeTask(t *task) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if q.onError != nil {
				q.onError(t.id, fmt.Errorf("task panic: %v", recovered))
			}
		}
	}()
	t.execute()
}
