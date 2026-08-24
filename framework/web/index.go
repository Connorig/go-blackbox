package webiris

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"

	"github.com/kataras/iris/v12"

	zaplog "github.com/Connorig/go-blackbox/framework/log"
)

// PartyComponent 用于向 Iris Application 注册路由、中间件和错误处理器。
// 回调仅在 WebIris 初始化时执行一次，不应在其中启动无退出机制的 goroutine。
type PartyComponent func(app *iris.Application)

// WebBaseFunc 是 Application Starter 启动 Web 服务所依赖的最小接口。
type WebBaseFunc interface {
	// Run 启动 Web 服务并阻塞到启动失败、运行失败或 Context 被取消。
	Run(ctx context.Context) error
	// StaticSource 在 Web 启动前注册静态文件系统。
	StaticSource(fs http.FileSystem) error
}

// WebIris 封装 Iris Application、运行配置和生命周期状态。
// 同一实例只允许运行一次，关闭后需要创建新实例才能再次启动。
type WebIris struct {
	app       *iris.Application // app 是实际处理 HTTP 请求的 Iris 实例。
	config    Config            // config 保存完成默认值处理后的运行配置。
	initErr   error             // initErr 保存兼容构造入口产生的延迟校验错误。
	mu        sync.Mutex        // mu 保护 started 状态，避免并发重复启动。
	started   bool              // started 标识 Run 是否已经被调用。
	ready     chan struct{}     // ready 在 Iris Host 真正开始 Serve 时关闭。
	readyOnce sync.Once         // readyOnce 保证 Ready 信号只发布一次。
}

// New 创建并校验一个 Iris Web 服务。
// 新代码应优先使用该方法，在应用启动前处理配置错误。
func New(config Config, components PartyComponent) (*WebIris, error) {
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}

	return &WebIris{
		app:    newApplication(normalized.LogLevel, components),
		config: normalized,
		ready:  make(chan struct{}),
	}, nil
}

// Init 保留原有参数式构造 API，供已有依赖项目平滑升级。
// 无效配置不会被忽略，而是在后续 Run 调用时记录并返回。
func Init(timeFormat, port, logLevel string, components PartyComponent) *WebIris {
	return InitWithConfig(Config{
		Address:    port,
		TimeFormat: timeFormat,
		LogLevel:   logLevel,
	}, components)
}

// InitWithConfig 保留单返回值和链式调用风格。
// 与 New 不同，该方法把配置错误保存在实例中，由 Run 统一处理。
func InitWithConfig(config Config, components PartyComponent) *WebIris {
	web, err := New(config, components)
	if err == nil {
		return web
	}

	fallback := Config{
		Address:         ":0",
		TimeFormat:      DefaultTimeFormat,
		LogLevel:        DefaultLogLevel,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
	return &WebIris{
		app:     newApplication(DefaultLogLevel, components),
		config:  fallback,
		initErr: err,
		ready:   make(chan struct{}),
	}
}

// StaticSource 将文件系统注册到根路径，用于提供 SPA 或其他静态资源。
// 该方法必须在 Run 前调用；启动后修改路由会直接返回错误。
func (w *WebIris) StaticSource(fs http.FileSystem) error {
	if w == nil {
		return errors.New("webiris: configure static source on nil WebIris")
	}
	if w.app == nil {
		return errors.New("webiris: iris application is nil")
	}
	if w.initErr != nil {
		w.app.Logger().Errorf("configure static source with invalid web config failed: %v", w.initErr)
		return fmt.Errorf("webiris: invalid configuration: %w", w.initErr)
	}
	if isNilFileSystem(fs) {
		err := errors.New("webiris: static file system is nil")
		w.app.Logger().Error(err)
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		err := errors.New("webiris: static source must be configured before Run")
		w.app.Logger().Error(err)
		return err
	}

	w.app.HandleDir("/", fs)
	return nil
}

// Application 返回底层 Iris Application，供依赖方使用 Iris 原生高级能力。
// 路由和中间件仍需在 Run 前完成注册。
func (w *WebIris) Application() *iris.Application {
	if w == nil {
		return nil
	}
	return w.app
}

// Ready 返回只读启动信号。
// Iris Host 真正进入 Serve 阶段后该 Channel 会关闭，调用方无需固定 Sleep。
func (w *WebIris) Ready() <-chan struct{} {
	if w == nil {
		return nil
	}
	return w.ready
}

// newApplication 创建 Iris 实例并安装默认 PanicRecovery 中间件
// (业务 handler panic 返回统一 500 B0001 JSON 并记录结构化日志)。
// components 用于集中注册业务路由和中间件，nil 表示不注册额外组件。
func newApplication(logLevel string, components PartyComponent) *iris.Application {
	application := iris.New()
	application.Logger().Handle(zaplog.GologHandler("web"))
	application.Logger().SetOutput(zaplog.GologWriter("web"))
	application.Use(PanicRecovery())
	application.Logger().SetLevel(logLevel)

	if components != nil {
		components(application)
	}

	return application
}

// markStarted 原子地标记 WebIris 已进入启动流程。
// 重复调用返回错误，防止 Iris Application 被二次运行。
func (w *WebIris) markStarted() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return errors.New("webiris: Run can only be called once per WebIris instance")
	}
	w.started = true
	return nil
}

// isNilFileSystem 同时识别普通 nil 和接口中包装的类型化 nil。
// 类型化 nil 若直接传入 Iris，可能在请求阶段触发难以定位的 panic。
func isNilFileSystem(fileSystem http.FileSystem) bool {
	if fileSystem == nil {
		return true
	}

	value := reflect.ValueOf(fileSystem)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
