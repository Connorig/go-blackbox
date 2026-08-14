package appbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestSetupRegistrationFiltersNil 验证 BeforeSetup 和 AfterSetup 不会保存 nil 回调。
func TestSetupRegistrationFiltersNil(t *testing.T) {
	first := func(context.Context) error { return nil }
	second := func(context.Context) error { return nil }
	builder := (&ApplicationBuild{}).
		BeforeSetup(nil, first, second).
		AfterSetup(first, nil, second)

	if len(builder.beforeSetups) != 2 {
		t.Fatalf("unexpected before setup count: %d", len(builder.beforeSetups))
	}
	if len(builder.afterSetups) != 2 {
		t.Fatalf("unexpected after setup count: %d", len(builder.afterSetups))
	}
}

// TestSetSeedsFiltersNilAndEnablesCron 验证有效 Seed 会被保存并自动启用 Cron。
func TestSetSeedsFiltersNilAndEnablesCron(t *testing.T) {
	seedFunc := func(context.Context) error { return nil }
	builder := (&ApplicationBuild{}).SetSeeds(nil, seedFunc)

	if len(builder.seeds) != 1 {
		t.Fatalf("unexpected Cron seed count: %d", len(builder.seeds))
	}
	if !builder.IsRunningCronJob {
		t.Fatal("SetSeeds must enable Cron when an effective seed is registered")
	}
}

// TestWebLifecycleExecutesCallbacksInOrder 验证回调顺序，并确认 BeforeSetup、AfterSetup 收到运行 Context。
func TestWebLifecycleExecutesCallbacksInOrder(t *testing.T) {
	restoreLogger := useNopApplicationLogger()
	t.Cleanup(restoreLogger)

	contextKey := struct{}{}
	const contextValue = "runtime-value"
	events := make(chan string, 3)
	ready := make(chan struct{})
	stopped := make(chan struct{})
	fake := &fakeWebService{
		ready: ready,
		run: func(ctx context.Context) error {
			events <- "web"
			close(ready)
			<-ctx.Done()
			close(stopped)
			return nil
		},
	}
	application := &application{builder: &ApplicationBuild{
		IsEnableWeb: true,
		irisApp:     fake,
		beforeSetups: []SetupFunc{func(setupCtx context.Context) error {
			if setupCtx.Value(contextKey) != contextValue {
				return errors.New("before setup did not receive runtime context")
			}
			events <- "before"
			return nil
		}},
		afterSetups: []SetupFunc{func(setupCtx context.Context) error {
			if setupCtx.Value(contextKey) != contextValue {
				return errors.New("after setup did not receive runtime context")
			}
			events <- "after"
			return nil
		}},
	}}

	parentCtx := context.WithValue(context.Background(), contextKey, contextValue)
	ctx, cancel := context.WithCancel(parentCtx)
	if err := application.startWebLifecycle(ctx); err != nil {
		cancel()
		t.Fatalf("execute Web lifecycle failed: %v", err)
	}

	for index, expected := range []string{"before", "web", "after"} {
		select {
		case actual := <-events:
			if actual != expected {
				t.Fatalf("unexpected lifecycle event at index %d: want=%s got=%s", index, expected, actual)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for lifecycle event at index %d", index)
		}
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("fake Web service did not stop after context cancellation")
	}
}

// TestWebLifecycleSkipsCallbacksWithoutWeb 验证未启用 Web 时不会误执行 Web 专属回调。
func TestWebLifecycleSkipsCallbacksWithoutWeb(t *testing.T) {
	called := false
	application := &application{builder: &ApplicationBuild{
		beforeSetups: []SetupFunc{func(context.Context) error {
			called = true
			return nil
		}},
		afterSetups: []SetupFunc{func(context.Context) error {
			called = true
			return nil
		}},
	}}

	if err := application.startWebLifecycle(context.Background()); err != nil {
		t.Fatalf("skip Web lifecycle callbacks failed: %v", err)
	}
	if called {
		t.Fatal("Web lifecycle callbacks must not run when Web is disabled")
	}
}

// TestBuildingServiceCancelsWebWhenAfterSetupFails 验证 Web Ready 后回调失败会取消运行 Context。
func TestBuildingServiceCancelsWebWhenAfterSetupFails(t *testing.T) {
	restoreLogger := useNopApplicationLogger()
	t.Cleanup(restoreLogger)

	expected := errors.New("after setup failed")
	ready := make(chan struct{})
	stopped := make(chan struct{})
	fake := &fakeWebService{
		ready: ready,
		run: func(ctx context.Context) error {
			close(ready)
			<-ctx.Done()
			close(stopped)
			return nil
		},
	}
	application := &application{builder: &ApplicationBuild{
		IsEnableZapLogs: true,
		IsEnableWeb:     true,
		irisApp:         fake,
		afterSetups: []SetupFunc{func(context.Context) error {
			return expected
		}},
	}}

	if err := application.lifecycle.beginStart(); err != nil {
		t.Fatalf("begin application start failed: %v", err)
	}

	err := application.buildingService(func(context.Context, *ApplicationBuild) error {
		return nil
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected after setup error %v, got: %v", expected, err)
	}

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Web service did not stop after AfterSetup failure")
	}
}

// TestExecuteSetupFunctionsStopsOnError 验证回调失败后不会继续执行后续函数。
func TestExecuteSetupFunctionsStopsOnError(t *testing.T) {
	restoreLogger := useNopApplicationLogger()
	t.Cleanup(restoreLogger)

	expected := errors.New("setup failed")
	nextCalled := false
	application := &application{builder: &ApplicationBuild{}}
	err := application.executeSetupFunctions(context.Background(), "test setup", []SetupFunc{
		func(context.Context) error { return expected },
		func(context.Context) error {
			nextCalled = true
			return nil
		},
	})

	if !errors.Is(err, expected) {
		t.Fatalf("expected setup error %v, got: %v", expected, err)
	}
	if nextCalled {
		t.Fatal("setup execution must stop after the first error")
	}
}

// TestRegisterSeedsReceivesRuntimeContext 验证 Seed 由脚手架内部调用并接收运行 Context。
func TestRegisterSeedsReceivesRuntimeContext(t *testing.T) {
	restoreLogger := useNopApplicationLogger()
	t.Cleanup(restoreLogger)

	contextKey := struct{}{}
	ctx := context.WithValue(context.Background(), contextKey, "runtime-value")
	received := ""
	application := &application{builder: &ApplicationBuild{
		seeds: []SetupFunc{func(seedCtx context.Context) error {
			value, ok := seedCtx.Value(contextKey).(string)
			if !ok {
				return errors.New("runtime context value is missing")
			}
			received = value
			return nil
		}},
	}}

	if err := application.registerSeedsAndStartCron(ctx); err != nil {
		t.Fatalf("register Cron seeds failed: %v", err)
	}
	if received != "runtime-value" {
		t.Fatalf("unexpected runtime context value: %s", received)
	}
}
