package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/appioc"
	"github.com/Domingor/go-blackbox/seed"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/mongodb"
	"github.com/Domingor/go-blackbox/server/shutdown"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"sync"
	"time"
)

// 初始化执行器
var doOnce sync.Once

type Application interface {
	Start(builder func(ctx context.Context, builder *ApplicationBuild) error) error
}

// app启动应用
type application struct {
	builder *ApplicationBuild
}

// New 创建app-starter启动器
func New() (app *application) {
	doOnce.Do(func() {
		builder := &ApplicationBuild{}
		// 创建app启动器
		app = &application{
			builder,
		}
	})
	return
}

// Start 全局启动配置器，初始化个个服务配置信息
func (app *application) Start(builder func(ctx context.Context, builder *ApplicationBuild) error) (err error) {

	if builder == nil {
		err = fmt.Errorf("application builder is nil")
		return
	}
	// 全局context
	ctx := appioc.GetContext().Ctx

	// 属性\服务构建初始化
	err = builder(ctx, app.builder)

	if err != nil {
		err = fmt.Errorf("application builder fail checkout what've happened. %s", err.Error())
		return
	}

	// 启动iris之后再执行seed
	seedErr := seed.Seed(app.builder.seeds...)

	if seedErr != nil {
		zaplog.ZAPLOGSUGAR.Errorf("application builder seed fail checkout what've happened. %s", seedErr.Error())
	}

	// 执行定时任务
	if app.builder.IsRunningCronJob {
		CronJobSingle().Start()
	}

	// 打印输出web服务已启动
	zaplog.ZAPLOGSUGAR.Info("web server is running...", time.Now().Format(TimeFormat))
	//fmt.Println("web server is running...")

	if err == nil {
		shutdown.WaitExit(&shutdown.Configuration{
			BeforeExit: func(s string) {
				fmt.Println(s)
				zaplog.ZAPLOGSUGAR.Info(s)
				//if len(onTerminate) > 0 {
				//	for _, terminateFunc := range onTerminate {
				//		if terminateFunc != nil {
				//			terminateFunc(s)
				//		}
				//	}
				//}
			},
		})
	}
	return
}

// GormDb 获取操作数据库-Gorm实例
func GormDb() *gorm.DB {
	return appioc.GetDb()
}

// GlobalCtx 获取context上下文
func GlobalCtx() *appioc.GlobalContext {
	return appioc.GetContext()
}

// RedisCache 获取Redis缓存实例
func RedisCache() cache.Rediser {
	return appioc.GetCache()
}

// CronJobSingle 获取定时任务执行器实例
func CronJobSingle() *cron.Cron {
	return appioc.GetCronJobInstance()
}

// MongoDb 获取MongoDB实例
func MongoDb() *mongodb.Client {
	return appioc.GetMongoDb()
}
