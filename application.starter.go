package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/etc"
	"github.com/Domingor/go-blackbox/seed"
	"github.com/Domingor/go-blackbox/server/cache"
	"gorm.io/gorm"
)

type Application interface {
	Start(builder func(ctx context.Context, builder *ApplicationBuild) error) error
}

type application struct {
	builder *ApplicationBuild
}

func New() (app *application) {
	builder := &ApplicationBuild{}

	app = &application{
		builder,
	}
	return
}

func (app *application) Start(builder func(ctx context.Context, builder *ApplicationBuild) error) (err error) {

	if builder == nil {
		err = fmt.Errorf("application builder is nil")
		return
	}
	// 全局context
	ctx := etc.GetContext().Ctx

	// 属性构建初始化
	err = builder(ctx, app.builder)

	if err != nil {
		err = fmt.Errorf("application builder fail checkout what've happened. %s", err.Error())
	}

	// 启动iris之后再执行seed
	err = seed.Seed(app.builder.seeds...)
	if err != nil {
		err = fmt.Errorf("application builder seed fail checkout what've happened. %s", err.Error())
	}
	return
}

func GormDb() *gorm.DB {
	return etc.GetDb()
}

func GlobalCtx() *etc.GlobalContext {
	return etc.GetContext()
}

func RedisCache() cache.Rediser {
	return etc.GetCache()
}
