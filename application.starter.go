package appbox

import (
	"context"
	"errors"
	"github.com/Domingor/go-blackbox/seed"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/datasource"
	"github.com/Domingor/go-blackbox/server/mongodb"
	"github.com/Domingor/go-blackbox/server/shutdown"
	log "github.com/Domingor/go-blackbox/server/zaplog"
	"github.com/Domingor/go-blackbox/simpleioc"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"sync"
	"time"
)

// 初始化执行器
var (
	doOnce  sync.Once
	app     *application
	afterDo = make(chan struct{})
)

// AfterSecond 默认时长后开始执行 后置函数
const AfterSecond = time.Second * 2

// Application app启动器接口
type Application interface {
	// Start 用于读取配置文件、启动所有服务
	Start(builder func(ctx context.Context, builder *ApplicationBuild) error) error
}

// app启动器-实现Application接口
type application struct {
	builder *ApplicationBuild
}

// New 创建app-starter启动器
func New() Application {
	// 单例模式-
	doOnce.Do(func() {
		// 创建app启动器
		app = &application{
			&ApplicationBuild{},
		}
	})
	return app
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

	// 传入全局Context，开始执行配置信息，标记要启动的服务
	if err = builderFun(simpleioc.GetContext().Ctx, app.builder); err != nil {
		return err
	}

	// 启动日志
	if !app.builder.IsEnableZapLogs {
		// 未配置日志，则使用默认配置
		app.builder.InitLog(".", "debug")
	}

	// TODO others services that needed to be handled.
	// TODO write down here.
	// 1. 数据库
	if app.builder.IsEnableDB {
		// 初始化数据，注册模型
		if err = datasource.GormInit(app.builder.dbConfig, app.builder.dbModels); err != nil {
			log.SugaredLogger.Debugf("init db service error %s", err)
			return err
		}
		// 放入ioc
		instance, _ := datasource.GetDbInstance()
		//放入ioc容器
		simpleioc.Set(instance)
	}

	//2. cache
	if app.builder.IsEnableCache {
		// 初始化redis，放入容器
		simpleioc.Set(cache.Init(simpleioc.GetContext().Ctx, app.builder.redisOptions))
	}
	//3. MongoDb
	if app.builder.IsEnableMongoDB {
		if client, err := mongodb.GetClient(app.builder.mongoBbConfig, simpleioc.GetContext().Ctx); err != nil {
			log.SugaredLogger.Debugf("init mongoDb fail err %s", err)
		} else {
			// mongoDb客户端放入容器
			simpleioc.Set(client)
		}
	}

	// n. WebService
	if app.builder.IsEnableWeb { // 是否开启 WebServer
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

			// 预留n秒给iris进行服务监听启动，n秒过后开始执行后置函数（定时cron任务函数等）
			time.AfterFunc(AfterSecond, func() {
				afterDo <- struct{}{}
			})

			// 启动web，此时会阻塞。后面的代码不会被轮到执行
			if err = app.builder.irisApp.Run(simpleioc.GetContext().Ctx); err != nil {
				log.SugaredLogger.Errorf("Runing WebService error %s", err)
			}
		}()
		// 诺干秒后调用后置函数（定时cron任务函数等）
		time.AfterFunc(AfterSecond, func() {
			// 发送信道到 信道
			afterDo <- struct{}{}
		})
	}

	// 监听 web服务启动后3秒执行后置函数
	for {
		select {
		case <-afterDo:
			err = afterDoSomething()
			// 执行结束，退出for循环
			break
		}
		break
	}

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

func afterDoSomething() (err error) {
	log.SugaredLogger.Info("executing seeds")
	// 启动iris之后再执行seed
	if err = seed.Seed(app.builder.seeds...); err != nil {
		log.SugaredLogger.Debug("seed.Seed running failed,", err)
		return
	}

	// 执行定时任务
	if app.builder.IsRunningCronJob {
		CronJobSingle().Start()
	}
	return err
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
