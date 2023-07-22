package appbox

import (
	"context"
	"errors"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/mongodb"
	"github.com/Domingor/go-blackbox/server/shutdown"
	log "github.com/Domingor/go-blackbox/server/zaplog"
	"github.com/Domingor/go-blackbox/simpleioc"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"sync"
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
	// single instance
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
func (app *application) Start(builderFun func(ctx context.Context, builder *ApplicationBuild) error) (err error) {

	// 开始执行构建服务程序
	if err = app.buildingService(builderFun); err == nil {
		// 全部服务启动成功后，阻塞主线程，开始监听web端口服务:
		// 这里会监听一个无缓存chanel，阻塞式监听消息。防止main现场结束，一旦main现场结束，web服务的协程也会结束，即服务终止。
		shutdown.WaitExit(&shutdown.Configuration{
			BeforeExit: func(s string) {
				// 收到消息-开始执行钩子函数
				log.SugaredLogger.Info(s)
			},
		})
	}

	return
}

// 根据build配置是否开启服务标识进行一一初始化
func (app *application) buildingService(builderFun func(ctx context.Context, builder *ApplicationBuild) error) (err error) {
	// 构建器必须有效！
	if builderFun == nil {
		err = errors.New("builderFun is not a expected function for building")
		return
	}
	// 传入全局Context，开始执行配置信息
	if err = builderFun(simpleioc.GetContext().Ctx, app.builder); err != nil {
		return
	}

	// 是否开启 WebServer
	if app.builder.IsEnableWeb {
		// 开启协程监听TCP-Web端口服务
		go func() {
			log.SugaredLogger.Info("starting WebService...")
			// 判断是否加载静态文件
			if app.builder.isLoadingStaticFs {
				if err = app.builder.irisApp.StaticSource(app.builder.StaticFs); err != nil {
					log.SugaredLogger.Errorf("app.irisApp.StaticSource fail!")
					return
				}
			}
			// 启动web，此时会阻塞。后面的代码不会被轮到执行
			if err = app.builder.irisApp.Run(simpleioc.GetContext().Ctx); err != nil {
				log.SugaredLogger.Errorf("Runing WebService error %s", err)
			}
		}()
	}

	// TODO others services that needed to be handled.
	// TODO write down here.

	// 打印输出web服务已启动
	log.SugaredLogger.Info("WebServer is running successfully right now...")
	return
}

// GormDb 获取操作数据库-Gorm实例
func GormDb() *gorm.DB {
	return simpleioc.GetDb()
}

// GlobalCtx 获取context上下文
func GlobalCtx() *simpleioc.GlobalContext {
	return simpleioc.GetContext()
}

// RedisCache 获取Redis缓存实例
func RedisCache() cache.Rediser {
	return simpleioc.GetCache()
}

// CronJobSingle 获取定时任务执行器实例
func CronJobSingle() *cron.Cron {
	return simpleioc.GetCronJobInstance()
}

// MongoDb 获取MongoDB实例
func MongoDb() *mongodb.Client {
	return simpleioc.GetMongoDb()
}

/*


	//if builder == nil {
	//	log.SugaredLogger.Debug("application builder is nil")
	//	err = fmt.Errorf("application builder is nil")
	//	return
	//}
	//// 全局context
	//ctx := simpleioc.GetContext().Ctx
	//
	//// 服务构建初始化
	//err = builder(ctx, app.builder)
	//
	//if err != nil {
	//	log.SugaredLogger.Debug("application builder fail, please check what have happened here!")
	//	err = fmt.Errorf("application builder fail, please check what have happened here! %s", err.Error())
	//	return
	//}
	//
	//// 启动iris之后再执行seed
	//seedErr := seed.Seed(app.builder.seeds...)
	//
	//if seedErr != nil {
	//	log.SugaredLogger.Debug("seed.Seed fail,", seedErr.Error())
	//}
	//
	//// 执行定时任务
	//if app.builder.IsRunningCronJob {
	//	CronJobSingle().Start()
	//}
	//
	//// 打印输出web服务已启动
	//log.SugaredLogger.Info("web server is running now")
	//
	//if err == nil {
	//	// 全部服务启动成功后，阻塞主线程，开始监听web端口服务
	//	shutdown.WaitExit(&shutdown.Configuration{
	//		BeforeExit: func(s string) {
	//			// 收到消息-开始执行钩子函数
	//
	//			log.SugaredLogger.Info(s)
	//			//if len(onTerminate) > 0 {
	//			//	for _, terminateFunc := range onTerminate {
	//			//		if terminateFunc != nil {
	//			//			terminateFunc(s)
	//			//		}
	//			//	}
	//			//}
	//		},
	//	})
	//}*/
