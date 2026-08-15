package appbox

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Connorig/go-blackbox/framework/log"
	"go.uber.org/zap"
)

// TestStartWebServiceWithoutWebReturnsImmediately 验证 Worker 模式不会等待 Web Ready 信号。
func TestStartWebServiceWithoutWebReturnsImmediately(t *testing.T) {
	application := &application{builder: &ApplicationBuild{}}
	if err := application.startWebService(context.Background()); err != nil {
		t.Fatalf("start service without Web failed: %v", err)
	}
}

// TestStartWebServiceWaitsForReady 验证 Starter 收到 Ready 后返回，并把 Context 取消传递给 Web。
func TestStartWebServiceWaitsForReady(t *testing.T) {
	restoreLogger := useNopApplicationLogger()
	t.Cleanup(restoreLogger)

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
		IsEnableWeb: true,
		irisApp:     fake,
	}}

	ctx, cancel := context.WithCancel(context.Background())
	if err := application.startWebService(ctx); err != nil {
		cancel()
		t.Fatalf("start ready Web service failed: %v", err)
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("fake Web service did not receive context cancellation")
	}
}

// TestStartWebServiceReturnsStartupError 验证 Web 在 Ready 前失败时错误会同步返回。
func TestStartWebServiceReturnsStartupError(t *testing.T) {
	restoreLogger := useNopApplicationLogger()
	t.Cleanup(restoreLogger)

	expected := errors.New("listener failed")
	fake := &fakeWebService{
		ready: make(chan struct{}),
		run: func(context.Context) error {
			return expected
		},
	}
	application := &application{builder: &ApplicationBuild{
		IsEnableWeb: true,
		irisApp:     fake,
	}}

	err := application.startWebService(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected startup error %v, got: %v", expected, err)
	}
}

// TestStartWebServiceReturnsStaticSourceError 验证静态资源注册失败时不会继续启动 Web。
func TestStartWebServiceReturnsStaticSourceError(t *testing.T) {
	restoreLogger := useNopApplicationLogger()
	t.Cleanup(restoreLogger)

	expected := errors.New("static source failed")
	fake := &fakeWebService{
		ready:     make(chan struct{}),
		staticErr: expected,
		run: func(context.Context) error {
			t.Fatal("Run must not be called after static source configuration failed")
			return nil
		},
	}
	application := &application{builder: &ApplicationBuild{
		IsEnableWeb:       true,
		irisApp:           fake,
		isLoadingStaticFs: true,
	}}

	err := application.startWebService(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected static source error %v, got: %v", expected, err)
	}
}

// fakeWebService 模拟 WebBaseFunc 和 Ready 能力，用于隔离 Starter 生命周期测试。
type fakeWebService struct {
	ready     chan struct{}               // ready 由测试场景控制启动完成时机。
	staticErr error                       // staticErr 是静态资源注册的预设结果。
	run       func(context.Context) error // run 是 Web 主循环的预设行为。
}

// Run 执行测试场景注入的启动行为。
func (f *fakeWebService) Run(ctx context.Context) error {
	return f.run(ctx)
}

// StaticSource 返回测试场景预设的静态资源错误。
func (f *fakeWebService) StaticSource(http.FileSystem) error {
	return f.staticErr
}

// Ready 返回测试控制的启动完成信号。
func (f *fakeWebService) Ready() <-chan struct{} {
	return f.ready
}

// useNopApplicationLogger 临时安装无输出 Zap Logger，避免测试写入真实日志文件。
// 返回函数用于恢复全局 Logger，调用方必须通过 t.Cleanup 注册。
func useNopApplicationLogger() func() {
	oldLogger := zaplog.Logger
	oldSugaredLogger := zaplog.SugaredLogger

	logger := zap.NewNop()
	zaplog.Logger = logger
	zaplog.SugaredLogger = logger.Sugar()

	return func() {
		zaplog.Logger = oldLogger
		zaplog.SugaredLogger = oldSugaredLogger
	}
}
