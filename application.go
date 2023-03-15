package awesomeProject1

import (
	"awesomeProject1/server/datasource"
	"awesomeProject1/server/loadConf"
	"github.com/go-redis/redis/v8"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/kataras/iris/v12/middleware/recover"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"time"
)

type PartyComponent func(app *iris.Application)

type startFuns interface {
	enableWeb(port, logLevel string, components PartyComponent) *applicationStart
	enableDb(dbConfig datasource.PostgresConfig) *applicationStart
	enableCache(redConfig redis.UniversalOptions) *applicationStart
	loadConfig(func(loader loadConf.Loader) error) *applicationStart
}

type applicationStart struct {
	irisApp *iris.Application
	gormDb  *gorm.DB
	redisDb redis.UniversalClient
}

func (app *applicationStart) loadConfig(configStruct interface{}, loaderFun func(loader loadConf.Loader)) {
	loader := loadConf.NewLoader()
	if loaderFun == nil {

	}
	loaderFun(loader)
	err := loader.LoadToStruct(configStruct)
	if err != nil {
		logrus.Errorf("%s", err)
	}
}

func (app *applicationStart) enableWeb(port, logLevel string, components PartyComponent) *applicationStart {
	// 创建iris
	app.irisApp = iris.New()
	// 一个可以让程序从任意的 http-relative panics 中恢复过来，
	// 一个可以记录日志到终端。
	app.irisApp.Use(recover.New())
	app.irisApp.Use(logger.New())

	// 注册路由
	components(app.irisApp)
	// 日志级别
	app.irisApp.Logger().SetLevel(logLevel)

	// 开启监听端口
	app.irisApp.Listen(
		port,
		iris.WithoutInterruptHandler,
		iris.WithoutServerError(iris.ErrServerClosed),
		iris.WithOptimizations,
	)
	return app
}

func (app *applicationStart) enableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *applicationStart {
	// 初始化数据，注册模型
	datasource.GormInit(dbConfig, models)
	app.gormDb = datasource.GetDbInstance()
	return app
}

func (app *applicationStart) enableCache(redConfig redis.UniversalOptions) *applicationStart {
	universalOptions := &redis.UniversalOptions{
		Addrs:       redConfig.Addrs,
		Password:    redConfig.Password,
		PoolSize:    redConfig.PoolSize,
		IdleTimeout: 300 * time.Second,
	}
	app.redisDb = redis.NewUniversalClient(universalOptions)
	return app
}

func NewStart() (app *applicationStart) {
	//	初始化
	app = &applicationStart{}
	//// 创建iris
	//app.irisApp = iris.New()
	//// 一个可以让程序从任意的 http-relative panics 中恢复过来，
	//// 一个可以记录日志到终端。
	//app.irisApp.Use(recover.New())
	//app.irisApp.Use(logger.New())
	return
}
