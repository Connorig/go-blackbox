package webiris

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/core/host"
)

// Run 构建路由、监听 TCP 端口并阻塞等待服务退出。
// Context 取消后执行限时优雅关闭，启动和关闭错误都会记录日志并返回调用方。
func (w *WebIris) Run(ctx context.Context) error {
	if err := w.validateRun(ctx); err != nil {
		return err
	}
	if err := w.configureAndBuild(); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", w.config.Address)
	if err != nil {
		w.app.Logger().Errorf("create web listener failed, address=%s, error=%v", w.config.Address, err)
		return fmt.Errorf("webiris: create listener on %s: %w", w.config.Address, err)
	}

	watcherDone := make(chan struct{})
	shutdownResult := w.watchShutdown(ctx, listener, watcherDone)
	listenErr := w.app.Run(w.newRunner(listener))
	close(watcherDone)
	shutdownErr := <-shutdownResult

	var runErr error
	if listenErr != nil {
		w.app.Logger().Errorf("web service stopped with listener error, address=%s, error=%v", w.config.Address, listenErr)
		runErr = fmt.Errorf("webiris: listen on %s: %w", w.config.Address, listenErr)
	}
	return errors.Join(runErr, shutdownErr)
}

// validateRun 检查实例、配置和 Context 是否满足启动条件。
// started 在构建前即被标记，任何启动失败都要求调用方创建新实例重试。
func (w *WebIris) validateRun(ctx context.Context) error {
	if w == nil {
		return errors.New("webiris: run on nil WebIris")
	}
	if w.app == nil {
		return errors.New("webiris: iris application is nil")
	}
	if w.initErr != nil {
		w.app.Logger().Errorf("web service configuration is invalid: %v", w.initErr)
		return fmt.Errorf("webiris: invalid configuration: %w", w.initErr)
	}
	if ctx == nil {
		err := errors.New("webiris: run context is nil")
		w.app.Logger().Error(err)
		return err
	}
	if err := ctx.Err(); err != nil {
		w.app.Logger().Errorf("web service context is already done before startup: %v", err)
		return fmt.Errorf("webiris: context already done before startup: %w", err)
	}
	if err := w.markStarted(); err != nil {
		w.app.Logger().Error(err)
		return err
	}
	return nil
}

// configureAndBuild 应用 Iris 运行参数并提前构建路由。
// 提前 Build 可以让路由冲突等错误在监听端口前同步返回。
func (w *WebIris) configureAndBuild() error {
	w.app.Configure(
		iris.WithoutInterruptHandler,
		iris.WithoutServerError(iris.ErrServerClosed),
		iris.WithOptimizations,
		iris.WithTimeFormat(w.config.TimeFormat),
	)
	if err := w.app.Build(); err != nil {
		w.app.Logger().Errorf("build iris application failed: %v", err)
		return fmt.Errorf("webiris: build iris application: %w", err)
	}
	return nil
}

// watchShutdown 监听应用 Context，并在取消时关闭 Iris Host 和 TCP Listener。
// 返回的 Channel 只写入一次关闭结果，调用方必须读取以避免遗漏关闭错误。
func (w *WebIris) watchShutdown(ctx context.Context, listener net.Listener, watcherDone <-chan struct{}) <-chan error {
	result := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			result <- w.shutdown(listener)
		case <-watcherDone:
			result <- nil
		}
	}()
	return result
}

// shutdown 在配置的超时内优雅关闭 Iris，并兜底关闭 Listener。
// Listener 已关闭属于正常状态；其他关闭错误会与 Iris Shutdown 错误合并返回。
func (w *WebIris) shutdown(listener net.Listener) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), w.config.ShutdownTimeout)
	defer cancel()

	var shutdownErr error
	if err := w.app.Shutdown(shutdownCtx); err != nil {
		w.app.Logger().Errorf("gracefully shutdown web service failed: %v", err)
		shutdownErr = fmt.Errorf("shutdown iris application: %w", err)
	}
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		w.app.Logger().Errorf("close web listener failed, address=%s, error=%v", w.config.Address, err)
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("close web listener: %w", err))
	}
	return shutdownErr
}

// newRunner 创建使用既有 Listener 的 Iris Runner。
// Host 进入 Serve 阶段时关闭 Ready Channel，保证启动信号只发布一次。
func (w *WebIris) newRunner(listener net.Listener) iris.Runner {
	return iris.Listener(listener, func(supervisor *host.Supervisor) {
		supervisor.RegisterOnServe(func(host.TaskHost) {
			w.readyOnce.Do(func() {
				close(w.ready)
			})
		})
	})
}
