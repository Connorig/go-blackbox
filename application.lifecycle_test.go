package appbox

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// TestApplicationLifecycleRejectsRepeatedStart 验证同一应用实例不能重复启动。
func TestApplicationLifecycleRejectsRepeatedStart(t *testing.T) {
	lifecycle := &applicationLifecycle{}
	if err := lifecycle.beginStart(); err != nil {
		t.Fatalf("begin first application start failed: %v", err)
	}
	if err := lifecycle.beginStart(); err == nil {
		t.Fatal("repeated application start must return an error")
	}
}

// TestApplicationLifecycleShutsDownInReverseOrder 验证已初始化资源会严格按注册逆序关闭。
func TestApplicationLifecycleShutsDownInReverseOrder(t *testing.T) {
	lifecycle := &applicationLifecycle{}
	if err := lifecycle.beginStart(); err != nil {
		t.Fatalf("begin application start failed: %v", err)
	}

	closed := make([]string, 0, 3)
	for _, name := range []string{"database", "web", "worker"} {
		resourceName := name
		if err := lifecycle.registerShutdown(resourceName, func(context.Context) error {
			closed = append(closed, resourceName)
			return nil
		}); err != nil {
			t.Fatalf("register %s shutdown failed: %v", resourceName, err)
		}
	}
	if err := lifecycle.markRunning(); err != nil {
		t.Fatalf("mark application running failed: %v", err)
	}
	if err := lifecycle.shutdown(context.Background(), time.Second); err != nil {
		t.Fatalf("shutdown application failed: %v", err)
	}

	expected := []string{"worker", "web", "database"}
	if !reflect.DeepEqual(closed, expected) {
		t.Fatalf("unexpected shutdown order: want=%v got=%v", expected, closed)
	}
}

// TestApplicationLifecycleAggregatesShutdownErrors 验证多个资源关闭失败时错误链会全部保留。
func TestApplicationLifecycleAggregatesShutdownErrors(t *testing.T) {
	lifecycle := &applicationLifecycle{}
	if err := lifecycle.beginStart(); err != nil {
		t.Fatalf("begin application start failed: %v", err)
	}

	firstError := errors.New("database close failed")
	secondError := errors.New("worker close failed")
	if err := lifecycle.registerShutdown("database", func(context.Context) error { return firstError }); err != nil {
		t.Fatalf("register database shutdown failed: %v", err)
	}
	if err := lifecycle.registerShutdown("worker", func(context.Context) error { return secondError }); err != nil {
		t.Fatalf("register worker shutdown failed: %v", err)
	}

	err := lifecycle.shutdown(context.Background(), time.Second)
	if !errors.Is(err, firstError) || !errors.Is(err, secondError) {
		t.Fatalf("shutdown errors were not aggregated: %v", err)
	}
}

// TestApplicationLifecycleProvidesShutdownDeadline 验证关闭函数收到具有上限的 Context。
func TestApplicationLifecycleProvidesShutdownDeadline(t *testing.T) {
	lifecycle := &applicationLifecycle{}
	if err := lifecycle.beginStart(); err != nil {
		t.Fatalf("begin application start failed: %v", err)
	}

	deadlineFound := false
	if err := lifecycle.registerShutdown("deadline observer", func(ctx context.Context) error {
		_, deadlineFound = ctx.Deadline()
		return nil
	}); err != nil {
		t.Fatalf("register deadline observer failed: %v", err)
	}
	if err := lifecycle.shutdown(context.Background(), time.Second); err != nil {
		t.Fatalf("shutdown application failed: %v", err)
	}
	if !deadlineFound {
		t.Fatal("shutdown function did not receive a deadline")
	}
}

// TestBuilderRegistersShutdownConfiguration 验证 Builder 保存有效关闭函数并处理非法超时。
func TestBuilderRegistersShutdownConfiguration(t *testing.T) {
	builder := (&ApplicationBuild{}).
		OnShutdown("worker", func(context.Context) error { return nil }).
		OnShutdown("", func(context.Context) error { return nil }).
		OnShutdown("ignored", nil).
		WithShutdownTimeout(0)

	if len(builder.shutdownHooks) != 1 {
		t.Fatalf("unexpected shutdown hook count: %d", len(builder.shutdownHooks))
	}
	if builder.shutdownTimeout != defaultShutdownTimeout {
		t.Fatalf("unexpected default shutdown timeout: %s", builder.shutdownTimeout)
	}
}
