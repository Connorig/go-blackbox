package webiris

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestNewNormalizesConfig 验证 New 会清理字符串配置并补充默认值。
func TestNewNormalizesConfig(t *testing.T) {
	web, err := New(Config{
		Address:    " 127.0.0.1:0 ",
		LogLevel:   " DEBUG ",
		TimeFormat: " ",
	}, nil)
	if err != nil {
		t.Fatalf("create web service failed: %v", err)
	}

	if web.config.Address != "127.0.0.1:0" {
		t.Fatalf("unexpected normalized address: %s", web.config.Address)
	}
	if web.config.LogLevel != "debug" {
		t.Fatalf("unexpected normalized log level: %s", web.config.LogLevel)
	}
	if web.config.TimeFormat != DefaultTimeFormat {
		t.Fatalf("unexpected default time format: %s", web.config.TimeFormat)
	}
	if web.config.ShutdownTimeout != DefaultShutdownTimeout {
		t.Fatalf("unexpected shutdown timeout: %s", web.config.ShutdownTimeout)
	}
	if web.Application() == nil {
		t.Fatal("iris application must not be nil")
	}
}

// TestNewRejectsInvalidConfig 验证地址、端口和日志级别错误会在构造阶段返回。
func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "empty address", config: Config{}},
		{name: "missing port separator", config: Config{Address: "9528"}},
		{name: "non numeric port", config: Config{Address: ":web"}},
		{name: "port out of range", config: Config{Address: ":65536"}},
		{name: "unsupported log level", config: Config{Address: ":9528", LogLevel: "trace"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.config, nil); err == nil {
				t.Fatalf("expected invalid config error for %+v", test.config)
			}
		})
	}
}

// TestInitPreservesValidationErrorUntilRun 验证兼容构造入口不会静默忽略无效配置。
func TestInitPreservesValidationErrorUntilRun(t *testing.T) {
	web := Init("", "", "", nil)
	if err := web.Run(context.Background()); err == nil {
		t.Fatal("expected invalid legacy Init configuration to fail during Run")
	}
}

// TestRunRejectsCompletedContext 验证服务不会使用已经取消的 Context 启动。
func TestRunRejectsCompletedContext(t *testing.T) {
	web := mustNewWeb(t, Config{Address: "127.0.0.1:0"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := web.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got: %v", err)
	}
}

// TestRunReturnsPortConflict 验证端口被占用时 Run 会返回可识别的监听错误。
func TestRunReturnsPortConflict(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("create occupied listener failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			t.Errorf("close occupied listener failed: %v", closeErr)
		}
	})

	web := mustNewWeb(t, Config{Address: listener.Addr().String()})

	err = web.Run(context.Background())
	if err == nil {
		t.Fatal("expected port conflict error")
	}
	if !strings.Contains(err.Error(), "create listener") {
		t.Fatalf("unexpected port conflict error: %v", err)
	}
}

// TestRunPublishesReadyAndGracefullyStops 验证 Ready 信号、Context 关闭和禁止重复启动。
func TestRunPublishesReadyAndGracefullyStops(t *testing.T) {
	web := mustNewWeb(t, Config{
		Address:         "127.0.0.1:0",
		ShutdownTimeout: time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- web.Run(ctx)
	}()

	select {
	case <-web.Ready():
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("web service did not publish ready signal")
	}

	cancel()
	select {
	case runErr := <-runResult:
		if runErr != nil {
			t.Fatalf("gracefully stop web service failed: %v", runErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("web service did not stop after context cancellation")
	}

	if err := web.Run(context.Background()); err == nil {
		t.Fatal("expected the second Run call to fail")
	}
}

// TestStaticSourceRejectsNilFileSystem 验证普通 nil 和类型化 nil 都会被拒绝。
func TestStaticSourceRejectsNilFileSystem(t *testing.T) {
	web := mustNewWeb(t, Config{Address: "127.0.0.1:0"})

	if err := web.StaticSource(nil); err == nil {
		t.Fatal("expected nil file system error")
	}

	var typedNil *testFileSystem
	if err := web.StaticSource(typedNil); err == nil {
		t.Fatal("expected typed nil file system error")
	}
}

// testFileSystem 用于构造实现了 http.FileSystem 的类型化 nil 测试值。
type testFileSystem struct{}

// Open 实现 http.FileSystem；测试不访问真实文件，因此始终返回不存在错误。
func (*testFileSystem) Open(string) (http.File, error) {
	return nil, fs.ErrNotExist
}

// mustNewWeb 创建测试用 WebIris，构造失败时立即终止当前测试。
func mustNewWeb(t *testing.T, config Config) *WebIris {
	t.Helper()

	web, err := New(config, nil)
	if err != nil {
		t.Fatalf("create web service failed: %v", err)
	}
	return web
}
