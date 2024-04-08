package appbox

import (
	"context"
	"embed"
	"fmt"
	"github.com/Domingor/go-blackbox/apputils/apptoken"
	"github.com/Domingor/go-blackbox/seed"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/cronjobs"
	"github.com/Domingor/go-blackbox/server/datasource"
	"github.com/Domingor/go-blackbox/server/loader"
	"github.com/Domingor/go-blackbox/server/mongodb"
	"github.com/Domingor/go-blackbox/server/webiris"
	log "github.com/Domingor/go-blackbox/server/zaplog"
	"github.com/Domingor/go-blackbox/simpleioc"
	"net/http"
	"time"
)

const (
	// TimeFormat 日期格式
	TimeFormat = "2006-01-02 15:04:05"
)

// ApplicationBuilder app builder接口提供系统初始化服务基础功能
type ApplicationBuilder interface {
	EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *ApplicationBuild // 启动web服务

	EnableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *ApplicationBuild // 启动数据库
	EnableCache(redConfig cache.RedisOptions) *ApplicationBuild                            // 启动缓存
	LoadConfig(configStruct interface{}, loaderFun func(loader.Loader)) error              // 加载配置文件、环境变量等
	InitLog(outDirPath, level string) *ApplicationBuild                                    // 初始化日志打印
	EnableMongoDB(dbConfig *mongodb.MongoDBConfig) *ApplicationBuild                       // 启动缓存数据库
	InitCronJob() *ApplicationBuild                                                        // 初始化定时任务
	SetupToken(AMinute, RHour time.Duration, TokenIssuer string) *ApplicationBuild         // 配置web-token属性
	EnableStaticSource(file embed.FS) *ApplicationBuild
	// 加载静态资源

	// TODO ...more
}

type ApplicationBuild struct {

	// 创建Iris实例对象
	irisApp webiris.WebBaseFunc

	// 启动种子list集合
	seeds []seed.SeedFunc
	// 数据库配置
	dbConfig *datasource.PostgresConfig
	// 注册表模块-tables
	dbModels []interface{}
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

// EnableDb 启动数据库操作对象
func (app *ApplicationBuild) EnableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *ApplicationBuild {
	//开启 db
	app.IsEnableDB = true

	app.dbConfig = dbConfig

	app.dbModels = models
	return app
}

// EnableCache 启动缓存
func (app *ApplicationBuild) EnableCache(redConfig cache.RedisOptions) *ApplicationBuild {
	app.IsEnableCache = true

	app.redisOptions = redConfig
	return app
}

// LoadConfig 加载配置文件、环境变量值
func (app *ApplicationBuild) LoadConfig(configStruct interface{}, loaderFun func(loader.Loader)) error {
	loader := loader.NewLoader()
	if loaderFun == nil {

		return fmt.Errorf("loaderFun is nil")
	}

	// 加载解析配置文件属性
	loaderFun(loader)

	// 读取到的属性值赋值给配置对象
	err := loader.LoadToStruct(configStruct)
	return err
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

	// 初始化日志，通过 zapLog.日志对象进行调用

	if err := log.Init(); err != nil {
		fmt.Printf("Log Init() err %v\n", err)
	}

	return app
}

// EnableMongoDB 配置MongoDB客户端
func (app *ApplicationBuild) EnableMongoDB(dbConfig *mongodb.MongoDBConfig) *ApplicationBuild {
	if dbConfig != nil {

		app.IsEnableDB = true
		app.mongoBbConfig = dbConfig
	}
	return app
}

// InitCronJob 初始化定时任务对象，存放入IOC
func (app *ApplicationBuild) InitCronJob() *ApplicationBuild {
	// 设置启动定时任务
	app.IsRunningCronJob = true

	// 定时任务客户端放入容器
	simpleioc.Set(cronjobs.CronInstance())

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

// SetSeeds 设置启动项目时，要执行的一些钩子函数
func (app *ApplicationBuild) SetSeeds(seedFuncs ...seed.SeedFunc) *ApplicationBuild {
	app.seeds = append(app.seeds, seedFuncs...)
	return app
}
