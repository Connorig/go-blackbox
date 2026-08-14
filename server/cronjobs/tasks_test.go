package cronjobs

import (
	"context"
	"errors"
	"testing"
)

// TestRegisterRejectsInvalidInput 验证空名称、空表达式和 nil 函数被拒绝。
func TestRegisterRejectsInvalidInput(t *testing.T) {
	if _, err := Register("", "@every 1s", func(context.Context) error { return nil }); err == nil {
		t.Fatal("empty task name must be rejected")
	}
	if _, err := Register("task", "", func(context.Context) error { return nil }); err == nil {
		t.Fatal("empty spec must be rejected")
	}
	if _, err := Register("task", "@every 1s", nil); err == nil {
		t.Fatal("nil function must be rejected")
	}
}

// TestRegisterDuplicateName 验证同名任务重复注册被拒绝。
func TestRegisterDuplicateName(t *testing.T) {
	taskName := "duplicate-task"
	cleanupTasks(t)

	entryID, err := Register(taskName, "@every 1m", func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("register task failed: %v", err)
	}
	if entryID == 0 {
		t.Fatal("entry id must not be zero")
	}
	if _, err := Register(taskName, "@every 1m", func(context.Context) error { return nil }); err == nil {
		t.Fatal("duplicate task name must be rejected")
	}
}

// TestListAndRemove 验证任务列表与移除能力。
func TestListAndRemove(t *testing.T) {
	taskName := "list-remove-task"
	cleanupTasks(t)

	if _, err := Register(taskName, "@every 1m", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("register task failed: %v", err)
	}

	listed := List()
	if len(listed) != 1 || listed[0].Name != taskName {
		t.Fatalf("unexpected task list: %+v", listed)
	}

	if err := Remove(taskName); err != nil {
		t.Fatalf("remove task failed: %v", err)
	}
	if len(List()) != 0 {
		t.Fatal("task list must be empty after remove")
	}
	if err := Remove(taskName); err == nil {
		t.Fatal("removing missing task must return an error")
	}
}

// TestExecuteTaskRecoversPanic 验证任务 panic 不会向外传播。
func TestExecuteTaskRecoversPanic(t *testing.T) {
	// 直接调用执行包装器验证 panic 恢复
	executeTask("panic-task", func(context.Context) error {
		panic("boom")
	})
	executeTask("error-task", func(context.Context) error {
		return errors.New("business failure")
	})
	executeTask("ok-task", func(context.Context) error {
		return nil
	})
}

// cleanupTasks 清空任务注册表，避免测试间相互污染。
func cleanupTasks(t *testing.T) {
	t.Helper()
	taskMu.Lock()
	tasks = make(map[string]*Task)
	taskMu.Unlock()
	t.Cleanup(func() {
		taskMu.Lock()
		tasks = make(map[string]*Task)
		taskMu.Unlock()
	})
}

// TestRegisteredTasksShareScheduler 验证 Register 与 CronInstance 使用同一调度器。
func TestRegisteredTasksShareScheduler(t *testing.T) {
	cleanupTasks(t)
	taskName := "shared-scheduler"
	entryID, err := Register(taskName, "@every 1m", func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("register task failed: %v", err)
	}
	found := false
	for _, entry := range CronInstance().Entries() {
		if entry.ID == entryID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("registered task must appear in scheduler entries")
	}
}
