package webiris

import (
	"context"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/recover"
	"time"
)

/**
* @Author: Connor
* @Date:   23.3.22 15:53
* @Description:
 */

type PartyComponent func(app *iris.Application)

type WebBaseFunc interface {
	Run(ctx context.Context) error
}

type WebIris struct {
	app        *iris.Application
	port       string // 监听端口地址
	timeFormat string // 时间格式化
}

func Init(timeFormat, port, logLevel string, components PartyComponent) *WebIris {
	// 创建iris实例
	application := iris.New()

	// 一个可以让程序从任意的 http-relative panics 中恢复过来，
	application.Use(recover.New())

	// 日志级别
	application.Logger().SetLevel(logLevel)

	if components != nil {
		// 注册路路由
		components(application)
	}

	// 返回WebIris实例
	return &WebIris{
		app:        application,
		port:       port,
		timeFormat: timeFormat,
	}
}

func (w *WebIris) shutdownFuture(ctx context.Context) {
	if ctx == nil {
		return
	}
	var c context.Context
	var cancel context.CancelFunc
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			c = context.TODO()
			if err := w.app.Shutdown(c); nil != err {
			}
			return
		default:
			time.Sleep(time.Millisecond * 500)
		}
	}
}
func (w *WebIris) Run(ctx context.Context) (err error) {

	err = w.app.Listen(w.port,
		iris.WithoutInterruptHandler,
		iris.WithoutServerError(iris.ErrServerClosed),
		iris.WithOptimizations,
		iris.WithTimeFormat(w.timeFormat))
	//fmt.Println(err)
	zaplog.ZAPLOGSUGAR.Error(err)
	//go w.shutdownFuture(ctx)
	return
}
