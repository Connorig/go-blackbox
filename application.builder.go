package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/etc"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/datasource"
	"github.com/Domingor/go-blackbox/server/loadconf"
	"github.com/Domingor/go-blackbox/server/webiris"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ApplicationBuilder interface {
	EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *ApplicationBuild
	EnableDb(dbConfig *datasource.PostgresConfig, models []interface{}) *ApplicationBuild
	EnableCache(ctx context.Context, redConfig cache.RedisOptions) *ApplicationBuild
	LoadConfig(configStruct interface{}, loaderFun func(loadconf.Loader)) error
}

type ApplicationBuild struct {
	irisApp webiris.WebBaseFunc
	gormDb  *gorm.DB
	redisDb cache.Rediser
}

func (app *ApplicationBuild) EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *ApplicationBuild {
	app.irisApp = webiris.Init(
		timeFormat,
		port,
		logLevel,
		components)

	app.irisApp.Run()
	return app
}
func (app *ApplicationBuild) EnableDb(dbConfig *datasource.PostgresConfig, models []interface{}) *ApplicationBuild {
	//	// 初始化数据，注册模型
	datasource.GormInit(dbConfig, models...)
	//app.gormDb = datasource.GetDbInstance()
	// 放入容器
	etc.Set(datasource.GetDbInstance())

	return app
}
func (app *ApplicationBuild) EnableCache(ctx context.Context, redConfig cache.RedisOptions) *ApplicationBuild {
	// 初始化redis，放入容器
	etc.Set(cache.Init(ctx, redConfig))

	return app
}
func (app *ApplicationBuild) LoadConfig(configStruct interface{}, loaderFun func(loadconf.Loader)) error {
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
