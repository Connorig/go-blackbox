package webiris

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"sync"
	"time"

	"github.com/Connorig/go-blackbox/server/zaplog"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/core/host"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultAdminListen 是管理服务默认监听地址。
const DefaultAdminListen = ":6060"

// AdminConfig 定义管理服务配置。
type AdminConfig struct {
	// Listen 是监听地址，空值使用 DefaultAdminListen。
	Listen string
	// EnablePprof 控制 /debug/pprof 诊断端点，默认开启。
	EnablePprof bool
	// EnableMetrics 控制 /metrics Prometheus 端点，默认开启。
	EnableMetrics bool
	// EnableLogLevel 控制 POST /cl 运行时日志级别切换，默认开启。
	EnableLogLevel bool
	// ShutdownTimeout 是优雅关闭超时，默认 5 秒。
	ShutdownTimeout time.Duration
}

// Admin 是独立监听的管理服务：
// /debug/pprof/*（诊断）、/metrics（Prometheus 指标）、POST /cl（运行时日志级别）、
// 以及业务通过 RegisterRoutes 注册的管理路由。
// 管理端口与业务 Web 端口分离，适合暴露给运维/监控体系。
type Admin struct {
	config AdminConfig
	app    *iris.Application


	mu        sync.Mutex
	started   bool
	ready     chan struct{}
	readyOnce sync.Once
}

// NewAdmin 创建默认配置的管理服务。
func NewAdmin() *Admin {
	return NewAdminWithConfig(AdminConfig{
		EnablePprof:    true,
		EnableMetrics:  true,
		EnableLogLevel: true,
	})
}

// NewAdminWithConfig 按配置创建管理服务。
func NewAdminWithConfig(config AdminConfig) *Admin {
	if strings.TrimSpace(config.Listen) == "" {
		config.Listen = DefaultAdminListen
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = 5 * time.Second
	}
	admin := &Admin{
		config: config,
		app:    iris.New(),
		ready:  make(chan struct{}),
	}
	// 内置 API 在构造时注册，业务路由随后注册，业务无法覆盖框架 API。
	admin.registerBuiltinRoutes()
	return admin
}

// RegisterRoutes 注册业务管理路由（不会覆盖框架内置 API）。
func (a *Admin) RegisterRoutes(register func(app *iris.Application)) *Admin {
	if a == nil {
		return a
	}
	if register != nil {
		// 立即注册（内置 API 已在构造时注册，业务路由不会覆盖）。
		register(a.app)
	}
	return a
}

// Ready 返回就绪信号：管理服务真正开始监听后关闭。
func (a *Admin) Ready() <-chan struct{} {
	if a == nil {
		return nil
	}
	return a.ready
}

// Run 启动管理监听并阻塞，直到 Context 取消。
// 内置 API 先注册，业务路由后注册，业务无法覆盖框架 API。
func (a *Admin) Run(ctx context.Context) error {
	if a == nil {
		return errors.New("webiris: run admin on nil Admin")
	}
	if ctx == nil {
		return errors.New("webiris: admin run context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("webiris: admin context already done: %w", err)
	}

	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return errors.New("webiris: admin can only run once")
	}
	a.started = true
	a.mu.Unlock()

	listener, err := net.Listen("tcp", a.config.Listen)
	if err != nil {
		return fmt.Errorf("webiris: create admin listener on %s: %w", a.config.Listen, err)
	}

	watcherDone := make(chan struct{})
	shutdownResult := a.watchShutdown(ctx, listener, watcherDone)
	listenErr := a.app.Run(iris.Listener(listener, func(supervisor *host.Supervisor) {
		supervisor.RegisterOnServe(func(host.TaskHost) {
			a.readyOnce.Do(func() { close(a.ready) })
		})
	}))
	close(watcherDone)
	shutdownErr := <-shutdownResult

	if listenErr != nil {
		return fmt.Errorf("webiris: admin listener on %s: %w", a.config.Listen, listenErr)
	}
	return shutdownErr
}

// registerBuiltinRoutes 注册框架内置管理 API。
func (a *Admin) registerBuiltinRoutes() {
	if a.config.EnablePprof {
		party := a.app.Party("/debug/pprof")
		party.Any("/", func(ctx iris.Context) { pprof.Index(ctx.ResponseWriter(), ctx.Request()) })
		party.Any("/cmdline", func(ctx iris.Context) { pprof.Cmdline(ctx.ResponseWriter(), ctx.Request()) })
		party.Any("/profile", func(ctx iris.Context) { pprof.Profile(ctx.ResponseWriter(), ctx.Request()) })
		party.Any("/symbol", func(ctx iris.Context) { pprof.Symbol(ctx.ResponseWriter(), ctx.Request()) })
		party.Any("/trace", func(ctx iris.Context) { pprof.Trace(ctx.ResponseWriter(), ctx.Request()) })
		party.Any("/{action:path}", func(ctx iris.Context) { pprof.Index(ctx.ResponseWriter(), ctx.Request()) })
	}
	if a.config.EnableMetrics {
		a.app.Get("/metrics", func(ctx iris.Context) {
			promhttp.Handler().ServeHTTP(ctx.ResponseWriter(), ctx.Request())
		})
	}
	if a.config.EnableLogLevel {
		a.app.Post("/cl", func(ctx iris.Context) {
			var request struct {
				Level string `json:"level"`
			}
			if err := ctx.ReadJSON(&request); err != nil {
				Fail(ctx, http.StatusBadRequest, http.StatusBadRequest, "invalid body, expected {\"level\":\"debug\"}")
				return
			}
			if err := zaplog.SetLevel(request.Level); err != nil {
				Fail(ctx, http.StatusBadRequest, http.StatusBadRequest, err.Error())
				return
			}
			OK(ctx, map[string]string{"level": request.Level})
		})
	}
}

// watchShutdown 监听 Context 取消并关闭监听。
func (a *Admin) watchShutdown(ctx context.Context, listener net.Listener, watcherDone <-chan struct{}) <-chan error {
	result := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
			defer cancel()
			var shutdownErr error
			if err := a.app.Shutdown(shutdownCtx); err != nil {
				shutdownErr = fmt.Errorf("webiris: shutdown admin: %w", err)
			}
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				shutdownErr = errors.Join(shutdownErr, fmt.Errorf("webiris: close admin listener: %w", err))
			}
			result <- shutdownErr
		case <-watcherDone:
			result <- nil
		}
	}()
	return result
}

