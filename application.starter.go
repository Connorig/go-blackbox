package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/etc"
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
	ctx := etc.GetContext().Ctx

	// 属性构建初始化
	err = builder(ctx, app.builder)

	if err != nil {
		err = fmt.Errorf("application builder fail checkout what have happened")
	}
	return
}
