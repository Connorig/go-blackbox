package appbox

import (
	"context"
	"errors"

	"github.com/Connorig/go-blackbox/framework/database"
)

// RegisterModels 注册业务模型提供函数:
// 业务项目在 model 包定义 All() 返回全部模型,main 中一行注册;
// EnableDatabase 开启 AutoMigrate 时,gbx 在数据库就绪后自动建表,
// 无需在 main 中列出全部模型或手写迁移样板。
//
//	// internal/model/all.go(业务项目)
//	func All() []interface{} {
//	    return []interface{}{&User{}, &Order{}, &Product{}}
//	}
//
//	// main.go
//	builder.EnableDatabase(&datasource.Config{Driver: ..., DSN: ..., AutoMigrate: true})
//	builder.RegisterModels(model.All)
func (app *ApplicationBuild) RegisterModels(models func() []interface{}) *ApplicationBuild {
	if app == nil {
		return app
	}
	app.modelProvider = models
	return app
}

// runModelMigrations 执行注册模型的自动迁移(数据库就绪后调用)。
// 仅当 AutoMigrate 开启且有模型提供函数时执行。
func (app *ApplicationBuild) runModelMigrations(ctx context.Context) error {
	if app == nil || app.modelProvider == nil {
		return nil
	}
	if app.databaseConfig == nil || !app.databaseConfig.AutoMigrate {
		return nil
	}
	models := app.modelProvider()
	if len(models) == 0 {
		return nil
	}
	instance, err := datasource.Get()
	if err != nil {
		return err
	}
	if instance == nil || instance.DB() == nil {
		return errors.New("database instance is nil: call EnableDatabase before RegisterModels")
	}
	return instance.DB().WithContext(ctx).AutoMigrate(models...)
}
