package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/appioc"
	"github.com/Domingor/go-blackbox/apputils/apptoken"
	"github.com/Domingor/go-blackbox/seed"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/cronjobs"
	"github.com/Domingor/go-blackbox/server/datasource"
	"github.com/Domingor/go-blackbox/server/loadconf"
	"github.com/Domingor/go-blackbox/server/mongodb"
	"github.com/Domingor/go-blackbox/server/webiris"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"time"
)

const TimeFormat = "2006-01-02 15:04:05"

// ApplicationBuilder app builder接口提供系统初始化服务基础功能
type ApplicationBuilder interface {
	EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *ApplicationBuild
	EnableDb(dbConfig *datasource.PostgresConfig, models []interface{}) *ApplicationBuild
	EnableCache(ctx context.Context, redConfig cache.RedisOptions) *ApplicationBuild
	LoadConfig(configStruct interface{}, loaderFun func(loadconf.Loader)) error
	InitLog(outDirPath, level string) *ApplicationBuild
	EnableMongoDB(dbConfig *mongodb.MongoDBConfig) *ApplicationBuild
	InitCronJob() *ApplicationBuild
	SetupToken(AMinute, RHour time.Duration, TokenIssuer string) *ApplicationBuild
}

type ApplicationBuild struct {
	// 创建Iris实例对象
	irisApp webiris.WebBaseFunc

	//gormDb  *gorm.DB
	//redisDb cache.Rediser

	// 启动种子list集合
	seeds []seed.SeedFunc

	// 是否启动定时服务，在enableCronjob后为true，会自动start()
	IsRunningCronJob bool
}

// EnableWeb 启动Web服务
func (app *ApplicationBuild) EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *ApplicationBuild {
	app.irisApp = webiris.Init(
		timeFormat,
		port,
		logLevel,
		components)

	go func() {
		// 启动web，此时会阻塞。后面的代码不会被轮到执行
		app.irisApp.Run()
	}()
	return app
}

// EnableDb 启动数据库操作对象
func (app *ApplicationBuild) EnableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *ApplicationBuild {
	//	// 初始化数据，注册模型
	datasource.GormInit(dbConfig, models)

	// 放入容器
	appioc.Set(datasource.GetDbInstance())

	return app
}

// EnableCache 启动缓存
func (app *ApplicationBuild) EnableCache(ctx context.Context, redConfig cache.RedisOptions) *ApplicationBuild {
	// 初始化redis，放入容器
	appioc.Set(cache.Init(ctx, redConfig))

	return app
}

// LoadConfig 加载配置文件、环境变量值
func (app *ApplicationBuild) LoadConfig(configStruct interface{}, loaderFun func(loadconf.Loader)) error {
	loader := loadconf.NewLoader()
	if loaderFun == nil {
		return fmt.Errorf("loaderFun is nil")
	}
	// 加载解析配置文件属性
	loaderFun(loader)

	err := loader.LoadToStruct(configStruct)
	return err
}

// InitLog 初始化自定义日志
func (app *ApplicationBuild) InitLog(outDirPath, level string) *ApplicationBuild {
	if len(outDirPath) > 0 {
		zaplog.CONFIG.Director = outDirPath
	}
	if len(level) > 0 {
		zaplog.CONFIG.Level = level
	}
	// 初始化日志
	zaplog.Init()
	return app
}

// EnableMongoDB 启动MongoDB客户端
func (app *ApplicationBuild) EnableMongoDB(dbConfig *mongodb.MongoDBConfig) *ApplicationBuild {
	client, err := mongodb.GetClient(dbConfig, appioc.GetContext().Ctx)
	if err != nil {
		zaplog.ZAPLOGSUGAR.Debugf("init mongoDb fail err %s", err)
	}
	// mongoDb客户端放入容器
	appioc.Set(client)
	return app
}

// SetSeeds 设置启动项目时，要执行的一些钩子函数
func (app *ApplicationBuild) SetSeeds(seedFuncs ...seed.SeedFunc) *ApplicationBuild {
	app.seeds = append(app.seeds, seedFuncs...)
	return app
}

// InitCronJob 初始化定时任务对象，存放入IOC
func (app *ApplicationBuild) InitCronJob() *ApplicationBuild {
	instance := cronjobs.CronInstance()
	// 设置启动定时任务
	app.IsRunningCronJob = true

	// 定时任务客户端放入容器
	appioc.Set(instance)
	return app
}

// SetupToken 设置系统token有效期
func (app *ApplicationBuild) SetupToken(AMinute, RHour time.Duration, TokenIssuer string) *ApplicationBuild {
	apptoken.Init(AMinute, RHour, TokenIssuer)
	return app
}
