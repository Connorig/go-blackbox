package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/etc"
	"github.com/Domingor/go-blackbox/seed"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/datasource"
	"github.com/Domingor/go-blackbox/server/loadconf"
	"github.com/Domingor/go-blackbox/server/webiris"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"gorm.io/gorm"
)

type ApplicationBuilder interface {
	EnableWeb(timeFormat, port, logLevel string, components webiris.PartyComponent) *ApplicationBuild
	EnableDb(dbConfig *datasource.PostgresConfig, models []interface{}) *ApplicationBuild
	EnableCache(ctx context.Context, redConfig cache.RedisOptions) *ApplicationBuild
	LoadConfig(configStruct interface{}, loaderFun func(loadconf.Loader)) error
	InitLog(outDirPath string) *ApplicationBuild
}

type ApplicationBuild struct {
	irisApp webiris.WebBaseFunc
	gormDb  *gorm.DB
	redisDb cache.Rediser

	seeds []seed.SeedFunc
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
	// 加载解析配置文件属性
	loaderFun(loader)

	err := loader.LoadToStruct(configStruct)
	return err
}

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

func (app *ApplicationBuild) SetSeeds(seedFuncs ...seed.SeedFunc) *ApplicationBuild {
	app.seeds = append(app.seeds, seedFuncs...)
	return app
}
