package appbox

import (
	"context"
	"errors"
	"fmt"
	stdlog "log"
	"sync"
	"time"

	"github.com/Connorig/go-blackbox/framework/cache"
	"github.com/Connorig/go-blackbox/framework/database"
	"github.com/Connorig/go-blackbox/framework/mongo"
	"github.com/Connorig/go-blackbox/framework/lifecycle"
	log "github.com/Connorig/go-blackbox/framework/log"
	"github.com/Connorig/go-blackbox/framework/gbxioc"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// 应用启动器使用进程级单例，保持已有依赖项目的 New 调用行为。
var (
	doOnce sync.Once
	app    *application
)

// AfterSecond 是 Web 启动结果的观察窗口，用于同步捕获端口占用等快速失败。
// 该常量为兼容现有调用保留，后续会迁移为可配置的启动超时。
const AfterSecond = time.Second

// Application 定义脚手架应用的启动入口。
type Application interface {
	// Start 执行 Builder 回调、初始化启用的组件，并等待应用退出信号。
	Start(builder func(ctx context.Context, builder *ApplicationBuild) error) error
}

// application 保存本次进程使用的组件构建配置。
type application struct {
	builder       *ApplicationBuild    // builder 保存依赖项目注册的组件与生命周期配置。
	runtimeCancel context.CancelFunc   // runtimeCancel 负责释放 Web、Setup 和 Seed 共用的运行 Context。
	lifecycle     applicationLifecycle // lifecycle 管理单次启动状态和已初始化资源的逆序关闭。
}

// New 返回进程级 Application 单例。
// 当前版本保持历史单例语义，重复配置和可重入能力将在生命周期模块中统一优化。
func New(options ...Option) Application {
	doOnce.Do(func() {
		app = &application{builder: &ApplicationBuild{}}
	})
	return app
}

// Start 初始化全部启用组件，并阻塞等待系统信号或组件主动退出。
// 任一初始化步骤失败都会立即返回，避免应用以不完整状态继续运行。
func (app *application) Start(builderFun func(ctx context.Context, builder *ApplicationBuild) error) error {
	printBanner()
	if err := app.lifecycle.beginStart(); err != nil {
		stdlog.Printf("start application failed: %v", err)
		return err
	}

	if err := app.buildingService(builderFun); err != nil {
		shutdownErr := app.shutdownResources()
		return errors.Join(err, shutdownErr)
	}
	if err := app.lifecycle.markRunning(); err != nil {
		log.SugaredLogger.Errorf("mark application ready failed: %v", err)
		shutdownErr := app.shutdownResources()
		return errors.Join(err, shutdownErr)
	}

	exitMessage := shutdown.WaitExit(&shutdown.Configuration{
		BeforeExit: func(message string) {
			log.SugaredLogger.Info(message)
		},
	})
	log.SugaredLogger.Infof("application received exit request: %s", exitMessage)
	return app.shutdownResources()
}

// buildingService 按 Builder 启用标记依次初始化基础组件和 Web 服务。
// 初始化顺序保证日志先可用，数据组件先于 Seed 和 Cron 完成注册。
func (app *application) buildingService(builderFun func(ctx context.Context, builder *ApplicationBuild) error) error {
	if builderFun == nil {
		err := errors.New("application builder function is nil")
		stdlog.Printf("configure application failed: %v", err)
		return err
	}

	if err := builderFun(gbxioc.GetContext().Ctx, app.builder); err != nil {
		stdlog.Printf("execute application builder failed: %v", err)
		return fmt.Errorf("execute application builder: %w", err)
	}

	if !app.builder.IsEnableZapLogs {
		app.builder.InitLog(".", "debug")
	}
	if app.builder.logInitErr != nil {
		stdlog.Printf("initialize application logger failed: %v", app.builder.logInitErr)
		return fmt.Errorf("initialize application logger: %w", app.builder.logInitErr)
	}
	if err := app.lifecycle.registerShutdown("logger", func(context.Context) error {
		return log.Close()
	}); err != nil {
		log.SugaredLogger.Errorf("register logger shutdown failed: %v", err)
		return fmt.Errorf("register logger shutdown: %w", err)
	}

		if app.builder.IsEnableDB {
		var dbInstance *datasource.Instance
		var err error
		if app.builder.databaseConfig != nil {
			dbInstance, err = datasource.New(gbxioc.GetContext().Ctx, app.builder.databaseConfig, app.builder.dbModels...)
		} else {
			if _, initErr := datasource.Initialize(gbxioc.GetContext().Ctx, app.builder.dbConfig, app.builder.dbModels...); initErr != nil {
				err = initErr
			} else {
				dbInstance, err = datasource.Get()
			}
		}
		if err != nil {
			log.SugaredLogger.Errorf("initialize database failed: %v", err)
			return fmt.Errorf("initialize database: %w", err)
		}
		if dbInstance == nil {
			err := errors.New("initialized database instance is nil")
			log.SugaredLogger.Error(err)
			return err
		}
			// 自动迁移:RegisterModels 提供的模型(函数形态,免 main 样板)
		if err := app.builder.runModelMigrations(gbxioc.GetContext().Ctx); err != nil {
			log.SugaredLogger.Errorf("auto migrate registered models failed: %v", err)
			return fmt.Errorf("auto migrate registered models: %w", err)
		}

	// Register container entries: Instance (typed) + GORM (legacy GormDb()).
		if regErr := gbxioc.RegisterInstance(dbInstance); regErr != nil {
			log.SugaredLogger.Errorf("register database instance failed: %v", regErr)
			return fmt.Errorf("register database instance: %w", regErr)
		}
		if regErr := gbxioc.RegisterInstance(dbInstance.DB()); regErr != nil {
			log.SugaredLogger.Errorf("register gorm instance failed: %v", regErr)
			return fmt.Errorf("register gorm instance: %w", regErr)
		}
		hookName := "database:" + string(dbInstance.Driver())
		if err := app.lifecycle.registerShutdown(hookName, func(context.Context) error {
			return dbInstance.Close()
		}); err != nil {
			log.SugaredLogger.Errorf("register database shutdown failed: %v", err)
			return fmt.Errorf("register database shutdown: %w", err)
		}
	}

	if app.builder.IsEnableCache {
		cacheInstance, cacheErr := cache.Init(gbxioc.GetContext().Ctx, app.builder.redisOptions)
		if cacheErr != nil {
			log.SugaredLogger.Errorf("initialize Redis cache failed: %v", cacheErr)
			return fmt.Errorf("initialize Redis cache: %w", cacheErr)
		}
		if cacheInstance == nil {
			err := errors.New("initialize Redis cache returned nil instance")
			log.SugaredLogger.Error(err)
			return err
		}
		cache.SetGlobal(cacheInstance)
		if err := gbxioc.RegisterInstance(cacheInstance); err != nil {
			log.SugaredLogger.Errorf("register Redis instance failed: %v", err)
			return fmt.Errorf("register Redis instance: %w", err)
		}
		if err := app.lifecycle.registerShutdown("Redis", func(context.Context) error {
			return cacheInstance.Close()
		}); err != nil {
			log.SugaredLogger.Errorf("register Redis shutdown failed: %v", err)
			return fmt.Errorf("register Redis shutdown: %w", err)
		}
	}

	if app.builder.IsEnableMongoDB {
		if app.builder.mongoBbConfig == nil {
			err := errors.New("enable MongoDB requires non-nil MongoDB config")
			log.SugaredLogger.Error(err)
			return err
		}
		client, mongoErr := mongodb.GetClient(app.builder.mongoBbConfig, gbxioc.GetContext().Ctx)
		if mongoErr != nil {
			log.SugaredLogger.Errorf("initialize MongoDB client failed: %v", mongoErr)
			return fmt.Errorf("initialize MongoDB client: %w", mongoErr)
		}
		if client == nil {
			err := errors.New("initialize MongoDB returned nil client")
			log.SugaredLogger.Error(err)
			return err
		}
		// Use configured timeout for connectivity check; fail fast and close client.
		pingTimeout := app.builder.mongoBbConfig.Timeout * time.Second
		if pingTimeout <= 0 {
			pingTimeout = 10 * time.Second
		}
		pingCtx, pingCancel := context.WithTimeout(gbxioc.GetContext().Ctx, pingTimeout)
		pingErr := client.Ping(pingCtx)
		pingCancel()
		if pingErr != nil {
			_ = client.Disconnect(context.Background())
			log.SugaredLogger.Errorf("ping MongoDB failed: %v", pingErr)
			return fmt.Errorf("ping MongoDB: %w", pingErr)
		}
		mongodb.SetGlobal(client)
		if err := gbxioc.RegisterInstance(client); err != nil {
			log.SugaredLogger.Errorf("register MongoDB instance failed: %v", err)
			return fmt.Errorf("register MongoDB instance: %w", err)
		}
		if err := app.lifecycle.registerShutdown("MongoDB", client.Disconnect); err != nil {
			log.SugaredLogger.Errorf("register MongoDB shutdown failed: %v", err)
			return fmt.Errorf("register MongoDB shutdown: %w", err)
		}
	}

	// Web、Setup 和 Seed 使用派生 Context。启动后任一步骤失败时调用 cancel，
	// 可以立即停止已经运行的 Web；正常退出时父 Context 会自动向下传播取消信号。
	runtimeCtx, cancelRuntime := context.WithCancel(gbxioc.GetContext().Ctx)
	keepRuntimeContext := false
	
	defer func() {
		if !keepRuntimeContext {
			cancelRuntime()
		}
	}()
	// Named database instances run in parallel with the default instance.
	for _, named := range app.builder.namedDatabases {
		namedInstance, namedErr := datasource.NewNamed(gbxioc.GetContext().Ctx, named.name, named.config, named.models...)
		if namedErr != nil {
			log.SugaredLogger.Errorf("initialize named database %q failed: %v", named.name, namedErr)
			return fmt.Errorf("initialize named database %q: %w", named.name, namedErr)
		}
		if regErr := gbxioc.RegisterNamed("database:"+named.name, func() *datasource.Instance {
			return namedInstance
		}); regErr != nil {
			log.SugaredLogger.Errorf("register named database %q failed: %v", named.name, regErr)
			return fmt.Errorf("register named database %q: %w", named.name, regErr)
		}
		namedName := named.name
		if err := app.lifecycle.registerShutdown("database:"+named.name, func(context.Context) error {
			return namedInstance.Close()
		}); err != nil {
			log.SugaredLogger.Errorf("register named database %q shutdown failed: %v", namedName, err)
			return fmt.Errorf("register named database %q shutdown: %w", namedName, err)
		}
	}



	// Start IOC container: construct registered singletons and run OnInit hooks.
	if err := gbxioc.Start(runtimeCtx); err != nil {
		log.SugaredLogger.Errorf("start IOC container failed: %v", err)
		return fmt.Errorf("start IOC container: %w", err)
	}

	if err := app.startWebLifecycle(runtimeCtx); err != nil {
		return err
	}

	if err := app.registerSeedsAndStartCron(runtimeCtx); err != nil {
		return err
	}
	// Start admin service (pprof/metrics/log-level) in parallel when enabled.
	if app.builder.admin != nil {
		adminReady := app.builder.admin.Ready()
		go func() {
			if err := app.builder.admin.Run(runtimeCtx); err != nil {
				log.SugaredLogger.Errorf("admin service stopped: %v", err)
			}
		}()
		select {
		case <-adminReady:
			log.SugaredLogger.Info("admin service is running")
		case <-time.After(2 * time.Second):
			log.SugaredLogger.Warn("admin service did not become ready within 2 seconds")
		case <-runtimeCtx.Done():
			return fmt.Errorf("admin service startup canceled: %w", runtimeCtx.Err())
		}
	}

	app.runtimeCancel = cancelRuntime
	if err := app.lifecycle.registerShutdown("runtime context", func(context.Context) error {
		app.cancelRuntimeContext()
		return nil
	}); err != nil {
		log.SugaredLogger.Errorf("register runtime context shutdown failed: %v", err)
		return fmt.Errorf("register runtime context shutdown: %w", err)
	}
	if app.builder.IsRunningCronJob {
		if err := app.lifecycle.registerShutdown("Cron", app.stopCron); err != nil {
			log.SugaredLogger.Errorf("register Cron shutdown failed: %v", err)
			return fmt.Errorf("register Cron shutdown: %w", err)
		}
	}
	for _, hook := range app.builder.shutdownHooks {
		if err := app.lifecycle.registerShutdown(hook.name, hook.stop); err != nil {
			log.SugaredLogger.Errorf("register application shutdown hook failed, name=%s, error=%v", hook.name, err)
			return fmt.Errorf("register application shutdown hook %q: %w", hook.name, err)
		}
	}
	// IOC container must shut down before business resources, register last.
	if err := app.lifecycle.registerShutdown("IOC container", func(ctx context.Context) error {
		return gbxioc.Shutdown(ctx)
	}); err != nil {
		log.SugaredLogger.Errorf("register IOC container shutdown failed: %v", err)
		return fmt.Errorf("register IOC container shutdown: %w", err)
	}
	keepRuntimeContext = true

	if app.builder.IsEnableWeb {
		log.SugaredLogger.Info("WebServer is running successfully right now...")
	} else {
		log.SugaredLogger.Info("application services initialized without WebServer")
	}
	return nil
}


// stopCron 停止 Cron 接收新任务，并等待正在执行的任务结束或关闭 Context 超时。
func (app *application) stopCron(ctx context.Context) error {
	cronInstance := CronJobSingle()
	if cronInstance == nil {
		return errors.New("Cron instance is nil during shutdown")
	}
	waitCtx := cronInstance.Stop()
	select {
	case <-waitCtx.Done():
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for Cron jobs to stop: %w", ctx.Err())
	}
}

// shutdownResources 执行应用已注册资源的逆序关闭并记录聚合错误。
func (app *application) shutdownResources() error {
	err := app.lifecycle.shutdown(context.Background(), app.builder.shutdownTimeout)
	if err != nil {
		log.SugaredLogger.Errorf("shutdown application resources failed: %v", err)
	}
	return err
}

// cancelRuntimeContext 释放 Application 持有的派生 Context。
// 正常退出和重复清理都可以安全调用，取消函数执行后会从 Application 中移除。
func (app *application) cancelRuntimeContext() {
	if app.runtimeCancel == nil {
		return
	}
	app.runtimeCancel()
	app.runtimeCancel = nil
}

// startWebLifecycle 按 BeforeSetup、Web Ready、AfterSetup 的顺序执行 Web 生命周期。
// 未启用 Web 时不会执行 BeforeSetup 和 AfterSetup，避免回调语义与 Worker 模式混淆。
func (app *application) startWebLifecycle(ctx context.Context) error {
	if !app.builder.IsEnableWeb {
		return nil
	}
	if err := app.executeSetupFunctions(ctx, "before web startup", app.builder.beforeSetups); err != nil {
		return err
	}
	if err := app.startWebService(ctx); err != nil {
		return err
	}
	if err := app.executeSetupFunctions(ctx, "after web startup", app.builder.afterSetups); err != nil {
		return err
	}
	return nil
}

// startWebService 配置静态资源并启动 Web 服务。
// Run 返回得早于 AfterSecond 通常意味着监听失败；服务进入运行状态后若异常退出，
// 会记录错误并向 shutdown 模块发送退出信号，避免进程在 Web 已不可用时继续假运行。
func (app *application) startWebService(ctx context.Context) error {
	if !app.builder.IsEnableWeb {
		return nil
	}
	if app.builder.irisApp == nil {
		err := errors.New("web service is enabled but iris application is nil")
		log.SugaredLogger.Error(err)
		return err
	}

	if app.builder.isLoadingStaticFs {
		if err := app.builder.irisApp.StaticSource(app.builder.StaticFs); err != nil {
			log.SugaredLogger.Errorf("configure web static source failed: %v", err)
			return fmt.Errorf("configure web static source: %w", err)
		}
	}

	log.SugaredLogger.Info("starting WebService...")
	runResult := make(chan error, 1)
	go func() {
		runResult <- app.builder.irisApp.Run(ctx)
	}()

	// 新版 WebIris 会在路由构建完成且 Listener 创建成功后关闭 Ready Channel。
	// 对实现了旧 WebBaseFunc 接口的自定义 Web 服务，继续使用兼容观察窗口。
	// readyReporter 是新版 Web 实现提供的可选能力，旧实现无需强制适配。
	type readyReporter interface {
		Ready() <-chan struct{}
	}
	var ready <-chan struct{}
	if reporter, ok := app.builder.irisApp.(readyReporter); ok {
		ready = reporter.Ready()
	}

	if ready != nil {
		select {
		case err := <-runResult:
			return app.handleWebStartupError(err)
		case <-ready:
			app.monitorWebService(runResult)
			return nil
		case <-ctx.Done():
			err := fmt.Errorf("web service startup canceled: %w", ctx.Err())
			log.SugaredLogger.Error(err)
			return err
		}
	}

	// 旧 WebBaseFunc 没有 Ready 信号，只能保留短暂观察窗口兼容既有实现。
	timer := time.NewTimer(AfterSecond)
	defer timer.Stop()
	select {
	case err := <-runResult:
		return app.handleWebStartupError(err)
	case <-timer.C:
		app.monitorWebService(runResult)
		return nil
	case <-ctx.Done():
		err := fmt.Errorf("web service startup canceled: %w", ctx.Err())
		log.SugaredLogger.Error(err)
		return err
	}
}

// handleWebStartupError 统一处理 Web 在发布 Ready 前退出的结果。
// 即使底层错误为空，也会转换为明确错误，防止调用方误认为服务启动成功。
func (app *application) handleWebStartupError(err error) error {
	if err == nil {
		err = errors.New("web service stopped before startup completed")
	}
	log.SugaredLogger.Errorf("start WebService failed: %v", err)
	return fmt.Errorf("start web service: %w", err)
}

// monitorWebService 在启动完成后持续接收 Web 主循环结果。
// 正常 Context 取消会返回 nil；非 nil 错误表示 Web 服务异常停止，需要结束应用生命周期。
func (app *application) monitorWebService(runResult <-chan error) {
	go func() {
		if err := <-runResult; err != nil {
			log.SugaredLogger.Errorf("WebService stopped unexpectedly: %v", err)
			shutdown.Exit(fmt.Sprintf("WebService stopped unexpectedly: %v", err))
		}
	}()
}

// GormDb 获取操作数据库-Gorm实例
func GormDb() *gorm.DB {
	return gbxioc.GetDb()
}

// GlobalCtx 获取context上下文
func GlobalCtx() *gbxioc.GlobalContext {
	return gbxioc.GetContext()
}

// RedisCache 获取Redis缓存实例
func RedisCache() cache.Rediser {
	return gbxioc.GetCache()
}

// CronJobSingle 获取定时任务执行器实例
func CronJobSingle() *cron.Cron {
	return gbxioc.GetCronJobInstance()
}

// MongoDb 获取MongoDB实例
func MongoDb() *mongodb.Client {
	return gbxioc.GetMongoDb()
}

// registerSeedsAndStartCron 执行定时任务注册回调，并在全部注册成功后启动 Cron。
// 回调失败时 Cron 不会启动，避免只注册部分任务造成运行行为不一致。
func (app *application) registerSeedsAndStartCron(ctx context.Context) error {
	if err := app.executeSetupFunctions(ctx, "register cron seeds", app.builder.seeds); err != nil {
		return err
	}

	if app.builder.IsRunningCronJob {
		CronJobSingle().Start()
	}
	return nil
}

// executeSetupFunctions 按注册顺序执行生命周期回调，并向每个回调传入相同 Context。
// Context 已取消或任一回调返回错误时立即中断，日志中的 index 用于定位失败回调。
func (app *application) executeSetupFunctions(ctx context.Context, stage string, setupFuncs []SetupFunc) error {
	if ctx == nil {
		err := fmt.Errorf("%s setup context is nil", stage)
		log.SugaredLogger.Error(err)
		return err
	}

	for index, setupFunc := range setupFuncs {
		if setupFunc == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			log.SugaredLogger.Errorf("%s canceled before callback, index=%d, error=%v", stage, index, err)
			return fmt.Errorf("%s canceled before callback %d: %w", stage, index, err)
		}
		if err := setupFunc(ctx); err != nil {
			log.SugaredLogger.Errorf("%s callback failed, index=%d, error=%v", stage, index, err)
			return fmt.Errorf("%s callback %d: %w", stage, index, err)
		}
	}
	return nil
}
