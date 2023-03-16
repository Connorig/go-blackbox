package awesomeProject1

import (
	"awesomeProject1/server/cache"
	"awesomeProject1/server/datasource"
	"awesomeProject1/server/loadConf"
	"context"
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/kataras/iris/v12/middleware/recover"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type PartyComponent func(app *iris.Application)

type ApplicationBuilder interface {
	EnableWeb(port, logLevel string, components PartyComponent) *applicationBuilder
	EnableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *applicationBuilder
	EnableCache(ctx context.Context, redConfig cache.RedisOptions) *applicationBuilder
	LoadConfig(configStruct interface{}, loaderFun func(loadConf.Loader)) error
}

type applicationBuilder struct {
	irisApp *iris.Application
	gormDb  *gorm.DB
	redisDb cache.Rediser
}

func (app *applicationBuilder) LoadConfig(configStruct interface{}, loaderFun func(loadConf.Loader)) error {
	loader := loadConf.NewLoader()
	if loaderFun == nil {
		return fmt.Errorf("loaderFun is nil")
	}
	loaderFun(loader)
	err := loader.LoadToStruct(configStruct)
	if err != nil {
		logrus.Errorf("%s", err)
	}
	return nil
}

func (app *applicationBuilder) EnableWeb(port, logLevel string, components PartyComponent) *applicationBuilder {
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

func (app *applicationBuilder) EnableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *applicationBuilder {
	// 初始化数据，注册模型
	datasource.GormInit(dbConfig, models)
	app.gormDb = datasource.GetDbInstance()
	return app
}

func (app *applicationBuilder) EnableCache(ctx context.Context, redConfig cache.RedisOptions) *applicationBuilder {
	// 初始化redis
	app.redisDb = cache.CacheInit(ctx, redConfig)
	return app
}
