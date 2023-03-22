package goblackbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/server/cache"
	"gorm.io/gorm"
)

var ctx context.Context

type Application interface {
	Start(builder func(ctx context.Context, builder *ApplicationBuild) error) error
}

type application struct {
	builder *ApplicationBuild
}

func New() (app *application) {
	builder := &ApplicationBuild{}
	ctx = context.Background()
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
	// 属性构建初始化
	err = builder(ctx, app.builder)
	if err != nil {
		err = fmt.Errorf("application builder fail checkout what have happened")
	}
	return
}

func (app *application) getCache() cache.Rediser {
	return app.builder.redisDb
}

func (app *application) getDb() *gorm.DB {
	return app.builder.gormDb
}
