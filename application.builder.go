package appbox

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"time"

	"github.com/Connorig/go-blackbox/component/auth/token"
	"github.com/Connorig/go-blackbox/framework/seed"
	"github.com/Connorig/go-blackbox/framework/config"
	"github.com/Connorig/go-blackbox/framework/cache"
	"github.com/Connorig/go-blackbox/framework/cron"
	"github.com/Connorig/go-blackbox/framework/database"
	"github.com/Connorig/go-blackbox/framework/mongo"
	"github.com/Connorig/go-blackbox/framework/web"
	"github.com/kataras/iris/v12"
	log "github.com/Connorig/go-blackbox/framework/log"
	"github.com/Connorig/go-blackbox/framework/gbxioc"
)

const (
	// TimeFormat 日期格式
	TimeFormat = webiris.DefaultTimeFormat
)

// ApplicationBuilder 定义依赖项目可选择的组件和生命周期注册能力。
type ApplicationBuilder interface {
	// EnableWeb 使用兼容参数配置 Iris Web 服务。
	EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *ApplicationBuild
	// EnableWebWithConfig 使用结构化配置启用 Iris Web 服务。
	EnableWebWithConfig(config webiris.Config, components webiris.PartyComponent) *ApplicationBuild
	// EnableDb 配置 PostgreSQL、GORM 和需要迁移的 Model。
	EnableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *ApplicationBuild
	// EnableDatabase 使用统一配置启用 PostgreSQL、MySQL 或已注册的其他关系数据库。
	EnableDatabase(config *datasource.Config, models ...interface{}) *ApplicationBuild
	// EnableNamedDatabase 启用与默认实例并行的具名数据库实例（多数据源场景）。
	EnableNamedDatabase(name string, config *datasource.Config, models ...interface{}) *ApplicationBuild
	// EnableAdmin enables the admin service (pprof/metrics/log-level), optional listen address.
	EnableAdmin(listen ...string) *ApplicationBuild
	// EnableAdminRoutes registers business admin routes.
	EnableAdminRoutes(register func(app *iris.Application)) *ApplicationBuild
	// EnableCache 配置 Redis 缓存客户端。
	EnableCache(redConfig cache.RedisOptions) *ApplicationBuild
	// LoadConfig 通过调用方提供的 Loader 配置读取目标结构体。
	LoadConfig(configStruct interface{}, loaderFun func(apploader.Loader)) error
	// InitLog 初始化脚手架全局日志。
	InitLog(outDirPath, level string) *ApplicationBuild
	// EnableMongoDB 配置 MongoDB 客户端。
	EnableMongoDB(dbConfig *mongodb.MongoDBConfig) *ApplicationBuild
	// InitCronJob 显式启用 Cron 调度器，保留给未使用 SetSeeds 的兼容场景。
	InitCronJob() *ApplicationBuild
	// SetupToken 配置 JWT 有效期和签发者。
	SetupToken(AMinute, RHour time.Duration, TokenIssuer string) *ApplicationBuild
	// EnableStaticSource 配置 Web 根路径使用的嵌入式静态资源。
	EnableStaticSource(file embed.FS) *ApplicationBuild
	// BeforeSetup 注册 Web 启动前回调。
	BeforeSetup(setupFuncs ...SetupFunc) *ApplicationBuild
	// AfterSetup 注册 Web Ready 后回调。
	AfterSetup(setupFuncs ...SetupFunc) *ApplicationBuild
	// SetSeeds 注册 Cron 任务创建回调并自动启用调度器。
	SetSeeds(seedFuncs ...SetupFunc) *ApplicationBuild
	// OnShutdown 注册应用退出时需要逆序执行的资源关闭函数。
	OnShutdown(name string, shutdownFunc ShutdownFunc) *ApplicationBuild
	// WithShutdownTimeout 设置全部资源共享的应用关闭期限。
	WithShutdownTimeout(timeout time.Duration) *ApplicationBuild
}

// SetupFunc 是应用生命周期回调函数。
// 脚手架负责传入运行 Context、按注册顺序调用，并在错误发生时终止后续启动流程。
type SetupFunc = seed.SeedFunc

// ApplicationBuild 保存依赖项目通过 Builder 选择的组件配置和启用状态。
// 字段由各 Enable 方法写入，并由 Application.Start 在单线程启动阶段读取。
type ApplicationBuild struct {

	// 创建Iris实例对象
	irisApp webiris.WebBaseFunc
	// beforeSetups 保存 Web 启动前执行的回调。
	beforeSetups []SetupFunc
	// afterSetups 保存 Web Ready 后执行的回调。
	afterSetups []SetupFunc
	// seeds 保存用于注册 Cron 定时任务的回调。
	seeds []SetupFunc
	// shutdownHooks 保存业务侧注册的资源关闭函数，启动成功后转移到应用生命周期管理器。
	shutdownHooks []shutdownHook
	// shutdownTimeout 是全部资源执行逆序关闭时共享的最大时长。
	shutdownTimeout time.Duration
	// logInitErr 保存兼容链式 InitLog 无法直接返回的初始化错误。
	logInitErr error
	// 数据库配置
	dbConfig *datasource.PostgresConfig
	// databaseConfig 保存统一关系数据库配置；非 nil 时优先于兼容 PostgreSQL 配置。
	databaseConfig *datasource.Config
	// namedDatabases 保存具名数据库实例配置（多数据源）。
	namedDatabases []namedDatabaseConfig
	// admin is the standalone admin service; nil means disabled.
	admin *webiris.Admin
	// 注册表模块-tables(EnableDatabase 直接传入的模型)
	dbModels []interface{}
	// modelProvider 注册模型提供函数(RegisterModels,自动迁移用)
	modelProvider func() []interface{}
	// 上下文对象
	ctx context.Context
	// redis配置对象
	redisOptions cache.RedisOptions
	// MongoDB
	mongoBbConfig *mongodb.MongoDBConfig
	//=========================================》 启动标识
	// 是否启动定时服务，在enableCronjob后为true，会自动start()，即开始调用定时Cron表达式函数
	IsRunningCronJob bool
	// 是否加载静态Vue文件
	isLoadingStaticFs bool
	// 静态服务文件系统
	StaticFs http.FileSystem
	// 是否开启web
	IsEnableWeb bool
	// 是否开启数据库
	IsEnableDB bool
	// 是否开启redis
	IsEnableCache bool
	// 是否开始RabbitMq
	IsEnableRabbitMq bool
	// 是否开始定时任务
	IsEnableCronTask bool
	// 是否开启mongoDB
	IsEnableMongoDB bool
	// 是否开启静态服务文件
	IsEnableStaticFileServe bool
	// 是否开启日志zapLogs
	IsEnableZapLogs bool
	//=========================================》 启动标识
}

// EnableWeb 启动Web服务
func (app *ApplicationBuild) EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *ApplicationBuild {
	// 开启web服务
	app.IsEnableWeb = true

	if timeFormat == "" {
		timeFormat = TimeFormat
	}

	// 初始化iris对象
	app.irisApp = webiris.Init(
		timeFormat, // 日期格式化
		port,       // 监听服务端口
		logLevel,   // 日志级别
		components) // router路由组件
	return app
}

// EnableWebWithConfig 使用结构化配置启动 Web 服务。
// 该方法在保留 EnableWeb 兼容性的同时，允许依赖方设置优雅关闭超时等新增参数。
// 配置校验错误会在 Application.Start 阶段通过 WebIris.Run 返回并记录日志。
func (app *ApplicationBuild) EnableWebWithConfig(config webiris.Config, components webiris.PartyComponent) *ApplicationBuild {
	app.IsEnableWeb = true
	app.irisApp = webiris.InitWithConfig(config, components)
	return app
}

// EnableDb 启动数据库操作对象
func (app *ApplicationBuild) EnableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *ApplicationBuild {
	//开启 db
	app.IsEnableDB = true

	app.dbConfig = dbConfig

	app.dbModels = models
	return app
}

// EnableDatabase 使用统一关系数据库配置启用 GORM 数据源。
// PostgreSQL 已内置，MySQL、Oracle 等数据库需要在启动前通过 datasource.RegisterDialector 注册选定驱动。
func (app *ApplicationBuild) EnableDatabase(config *datasource.Config, models ...interface{}) *ApplicationBuild {
	app.IsEnableDB = true
	app.databaseConfig = config
	app.dbModels = models
	return app
}

// namedDatabaseConfig 描述一个具名数据库实例。
type namedDatabaseConfig struct {
	name   string
	config *datasource.Config
	models []interface{}
}

// EnableNamedDatabase 启用与默认实例并行的具名数据库实例。
// 实例会在启动期注册到 IOC 容器（名称 database:<name>），并独立接入关闭栈；
// 依赖方通过 datasource.GetNamed(name) 获取。
func (app *ApplicationBuild) EnableNamedDatabase(name string, config *datasource.Config, models ...interface{}) *ApplicationBuild {
	app.namedDatabases = append(app.namedDatabases, namedDatabaseConfig{name: name, config: config, models: models})
	return app
}

// EnableCache 启动缓存
// EnableAdmin enables the standalone admin service (default :6060) with
// pprof diagnostics, Prometheus metrics and runtime log-level switch (POST /cl).
func (app *ApplicationBuild) EnableAdmin(listen ...string) *ApplicationBuild {
	config := webiris.AdminConfig{
		EnablePprof:    true,
		EnableMetrics:  true,
		EnableLogLevel: true,
	}
	if len(listen) > 0 && listen[0] != "" {
		config.Listen = listen[0]
	}
	app.admin = webiris.NewAdminWithConfig(config)
	return app
}

// EnableAdminRoutes registers business admin routes (framework APIs take precedence).
func (app *ApplicationBuild) EnableAdminRoutes(register func(app *iris.Application)) *ApplicationBuild {
	if app.admin == nil {
		app.admin = webiris.NewAdmin()
	}
	app.admin.RegisterRoutes(register)
	return app
}

func (app *ApplicationBuild) EnableCache(redConfig cache.RedisOptions) *ApplicationBuild {
	app.IsEnableCache = true

	app.redisOptions = redConfig
	return app
}

// LoadConfig 使用独立 Loader 加载配置文件和环境变量。
// 配置函数或读取过程失败时返回保留错误链的错误，调用方必须终止当前启动流程。
func (app *ApplicationBuild) LoadConfig(configStruct interface{}, loaderFun func(apploader.Loader)) error {
	if loaderFun == nil {
		return fmt.Errorf("configure application loader: loader function is nil")
	}

	loader := apploader.NewLoader()
	loaderFun(loader)
	if err := loader.LoadToStruct(configStruct); err != nil {
		return fmt.Errorf("load application configuration: %w", err)
	}
	return nil
}

// InitLog 初始化自定义日志
func (app *ApplicationBuild) InitLog(outDirPath, level string) *ApplicationBuild {
	app.IsEnableZapLogs = true

	if len(outDirPath) > 0 {
		log.CONFIG.Director = outDirPath

	}

	if len(level) > 0 {
		log.CONFIG.Level = level
	}

	app.logInitErr = log.Init()

	return app
}

// EnableMongoDB 配置MongoDB客户端
func (app *ApplicationBuild) EnableMongoDB(dbConfig *mongodb.MongoDBConfig) *ApplicationBuild {
	if dbConfig != nil {
		// 只应启用 MongoDB 组件开关，不能误触发关系数据库分支（历史 bug）。
		app.IsEnableMongoDB = true
		app.mongoBbConfig = dbConfig
	}
	return app
}

// InitCronJob 初始化定时任务对象，存放入IOC
func (app *ApplicationBuild) InitCronJob() *ApplicationBuild {
	// 设置启动定时任务
	app.IsRunningCronJob = true

	// 定时任务客户端放入容器
	if err := gbxioc.RegisterInstance(cronjobs.CronInstance()); err != nil {
			// 閲嶅娉ㄥ唽淇濇寔鍏煎锛氱洿鎺ュ拷鐣r
		}

	return app
}

// SetupToken 设置系统token有效期
func (app *ApplicationBuild) SetupToken(AMinute, RHour time.Duration, TokenIssuer string) *ApplicationBuild {

	apptoken.Init(AMinute, RHour, TokenIssuer)
	return app
}

// EnableStaticSource  加载web服务静态资源文件
func (app *ApplicationBuild) EnableStaticSource(file embed.FS) *ApplicationBuild {
	// 开启静态服务器
	app.isLoadingStaticFs = true

	// 封装 Https文件系统
	app.StaticFs = http.FS(file)
	return app
}

// BeforeSetup 注册 Web 启动前回调。
// 回调仅在启用 Web 时执行，适合完成路由依赖检查、内存数据预热等启动前准备。
func (app *ApplicationBuild) BeforeSetup(setupFuncs ...SetupFunc) *ApplicationBuild {
	app.beforeSetups = appendSetupFunctions(app.beforeSetups, setupFuncs...)
	return app
}

// AfterSetup 注册 Web Ready 后回调。
// 回调仅在启用 Web 且发布 Ready 后执行，任一回调失败都会停止已启动的 Web 服务。
func (app *ApplicationBuild) AfterSetup(setupFuncs ...SetupFunc) *ApplicationBuild {
	app.afterSetups = appendSetupFunctions(app.afterSetups, setupFuncs...)
	return app
}

// SetSeeds 注册 Cron 定时任务创建回调。
// 至少注册一个有效回调时会自动启用 Cron，调用方无需再显式调用 InitCronJob。
func (app *ApplicationBuild) SetSeeds(seedFuncs ...SetupFunc) *ApplicationBuild {
	previousCount := len(app.seeds)
	app.seeds = appendSetupFunctions(app.seeds, seedFuncs...)
	if len(app.seeds) > previousCount {
		app.InitCronJob()
	}
	return app
}

// OnShutdown 注册应用退出时执行的资源关闭函数。
// 注册顺序应与资源初始化顺序一致，Application 会在启动失败或退出时按逆序执行。
// 空名称或 nil 函数会被忽略，避免把不可定位或不可执行的关闭任务带入运行阶段。
func (app *ApplicationBuild) OnShutdown(name string, shutdownFunc ShutdownFunc) *ApplicationBuild {
	if name == "" || shutdownFunc == nil {
		return app
	}
	app.shutdownHooks = append(app.shutdownHooks, shutdownHook{name: name, stop: shutdownFunc})
	return app
}

// WithShutdownTimeout 设置应用关闭的总超时时间。
// 非正数恢复为默认值，避免错误配置造成无限等待或关闭流程立即超时。
func (app *ApplicationBuild) WithShutdownTimeout(timeout time.Duration) *ApplicationBuild {
	if timeout <= 0 {
		app.shutdownTimeout = defaultShutdownTimeout
		return app
	}
	app.shutdownTimeout = timeout
	return app
}

// appendSetupFunctions 过滤 nil 回调并保持注册顺序。
// 忽略 nil 可以避免启动阶段出现无法定位的函数调用 panic。
func appendSetupFunctions(target []SetupFunc, setupFuncs ...SetupFunc) []SetupFunc {
	for _, setupFunc := range setupFuncs {
		if setupFunc != nil {
			target = append(target, setupFunc)
		}
	}
	return target
}
