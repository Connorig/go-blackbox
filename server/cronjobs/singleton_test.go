package cronjobs

import (
	"context"
	"sync"
	"testing"
)

// TestSingletonGuardSkipsConcurrentRun 验证单例任务在上一轮执行未结束时跳过再次触发。
func TestSingletonGuardSkipsConcurrentRun(t *testing.T) {
	cleanupTasks(t)

	// 第一次执行被阻塞，第二次触发应被跳过
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var runs int
	var mu sync.Mutex

	guarded := singletonGuard("singleton-task", func(context.Context) error {
		mu.Lock()
		runs++
		mu.Unlock()
		close(firstStarted)
		<-releaseFirst
		return nil
	})

	// 由于 executeTask 会记录日志，这里直接验证 guard 行为
	done := make(chan struct{})
	go func() {
		guarded()
		close(done)
	}()

	<-firstStarted
	guarded() // 第一轮未结束，应被跳过

	close(releaseFirst)
	<-done

	mu.Lock()
	defer mu.Unlock()
	if runs != 1 {
		t.Fatalf("singleton task must run only once while previous run is active, runs=%d", runs)
	}
}

// TestSingletonGuardAllowsSequentialRuns 验证单例任务结束后可以再次执行。
func TestSingletonGuardAllowsSequentialRuns(t *testing.T) {
	cleanupTasks(t)

	var runs int
	guarded := singletonGuard("sequential-task", func(context.Context) error {
		runs++
		return nil
	})
	guarded()
	guarded()
	if runs != 2 {
		t.Fatalf("sequential runs must both execute, runs=%d", runs)
	}
}

// TestRegisterSingletonRejectsInvalidInput 验证单例注册的输入校验。
func TestRegisterSingletonRejectsInvalidInput(t *testing.T) {
	cleanupTasks(t)

	if _, err := RegisterSingleton("", "@every 1m", func(context.Context) error { return nil }); err == nil {
		t.Fatal("empty name must be rejected")
	}
	if _, err := RegisterSingleton("task", "", func(context.Context) error { return nil }); err == nil {
		t.Fatal("empty spec must be rejected")
	}
	if _, err := RegisterSingleton("task", "@every 1m", nil); err == nil {
		t.Fatal("nil function must be rejected")
	}
}

// TestRegisterSingletonDuplicateName 验证单例任务重名拒绝。
func TestRegisterSingletonDuplicateName(t *testing.T) {
	cleanupTasks(t)

	if _, err := RegisterSingleton("dup-task", "@every 1m", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if _, err := RegisterSingleton("dup-task", "@every 1m", func(context.Context) error { return nil }); err == nil {
		t.Fatal("duplicate singleton task name must be rejected")
	}
	// 单例任务与普通任务共用注册表命名空间
	if _, err := Register("dup-task", "@every 1m", func(context.Context) error { return nil }); err == nil {
		t.Fatal("singleton task name must not collide with regular registration")
	}
}
