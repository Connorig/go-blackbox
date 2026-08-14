package cronjobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Connorig/go-blackbox/server/zaplog"
	"github.com/robfig/cron/v3"
)

// TaskFunc 定义定时任务执行函数，任务应响应 ctx 取消。
type TaskFunc func(ctx context.Context) error

// Task 描述一个已注册的具名定时任务。
type Task struct {
	Name    string         // 任务名称，注册后不可重复
	Spec    string         // Cron 表达式
	EntryID cron.EntryID   // 调度器条目 ID
	AddedAt time.Time      // 注册时间
}

var (
	taskMu sync.RWMutex
	tasks  = make(map[string]*Task)
)

// Register 注册具名定时任务。
// 相同名称重复注册返回错误；任务执行时自动附带 panic 恢复与结构化日志。
// 任务需要由 CronInstance().Start() 启动调度后才会执行。
func Register(name, spec string, function TaskFunc) (cron.EntryID, error) {
	if strings.TrimSpace(name) == "" {
		return 0, errors.New("cron task name is empty")
	}
	if strings.TrimSpace(spec) == "" {
		return 0, fmt.Errorf("cron task %q spec is empty", name)
	}
	if function == nil {
		return 0, fmt.Errorf("cron task %q function is nil", name)
	}

	taskMu.Lock()
	defer taskMu.Unlock()
	if _, exists := tasks[name]; exists {
		return 0, fmt.Errorf("cron task %q already registered", name)
	}

	entryID, err := CronInstance().AddFunc(spec, func() {
		executeTask(name, function)
	})
	if err != nil {
		return 0, fmt.Errorf("add cron task %q: %w", name, err)
	}
	tasks[name] = &Task{Name: name, Spec: spec, EntryID: entryID, AddedAt: time.Now()}
	return entryID, nil
}

// List 返回全部已注册任务（按注册顺序）。
func List() []Task {
	taskMu.RLock()
	defer taskMu.RUnlock()
	result := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, *task)
	}
	return result
}

// Remove 移除已注册任务并停止其调度。
// 任务不存在时返回错误。
func Remove(name string) error {
	taskMu.Lock()
	defer taskMu.Unlock()
	task, exists := tasks[name]
	if !exists {
		return fmt.Errorf("cron task %q not found", name)
	}
	CronInstance().Remove(task.EntryID)
	delete(tasks, name)
	zaplog.WithComponent("cron").Infow("cron task removed", "task", name)
	return nil
}

// executeTask 是任务执行包装器：panic 恢复 + 结构化日志 + 执行时长。
// 任务返回错误或 panic 都不会影响调度器与其他任务。
func executeTask(name string, function TaskFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	defer func() {
		if recovered := recover(); recovered != nil {
			zaplog.WithComponent("cron").Errorw("cron task panicked",
				"task", name, "panic", recovered, "duration", time.Since(start).String())
		}
	}()

	if err := function(ctx); err != nil {
		zaplog.WithComponent("cron").Errorw("cron task failed",
			"task", name, "error", err, "duration", time.Since(start).String())
		return
	}
	zaplog.WithComponent("cron").Infow("cron task completed",
		"task", name, "duration", time.Since(start).String())
}
