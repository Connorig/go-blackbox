package goblackbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/datasource"
	"github.com/Domingor/go-blackbox/server/loadconf"
	"github.com/Domingor/go-blackbox/server/webiris"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ApplicationBuilder interface {
	EnableWeb(port, logLevel string, components webiris.PartyComponent) *applicationBuilder
	EnableDb(dbConfig *datasource.PostgresConfig, models ...interface{}) *applicationBuilder
	EnableCache(ctx context.Context, redConfig cache.RedisOptions) *applicationBuilder
	LoadConfig(configStruct interface{}, loaderFun func(loadconf.Loader)) error
}

type applicationBuilder struct {
	irisApp webiris.WebBaseFunc
	gormDb  *gorm.DB
	redisDb cache.Rediser
}

func (app *applicationBuilder) LoadConfig(configStruct interface{}, loaderFun func(loadconf.Loader)) error {
	loader := loadconf.NewLoader()
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

func (app *applicationBuilder) EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *applicationBuilder {
	app.irisApp = webiris.Init(
		timeFormat,
		port,
		logLevel,
		components)

	app.irisApp.Run()
	return app
}

func (app *applicationBuilder) EnableDb(dbConfig *datasource.PostgresConfig, models []interface{}) *applicationBuilder {
	// 初始化数据，注册模型
	datasource.GormInit(dbConfig, models...)
	app.gormDb = datasource.GetDbInstance()
	return app
}

func (app *applicationBuilder) EnableCache(ctx context.Context, redConfig cache.RedisOptions) *applicationBuilder {
	// 初始化redis
	app.redisDb = cache.Init(ctx, redConfig)
	return app
}
