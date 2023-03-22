package webiris

import (
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/recover"
)

/**
* @Author: Connor
* @Date:   23.3.22 15:53
* @Description:
 */
const TimeFormat = "2006-01-02 15:04:05"

type PartyComponent func(app *iris.Application)

type WebBaseFunc interface {
	Run()
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

	// 注册路路由
	components(application)

	// 返回WebIris实例
	return &WebIris{
		app:        application,
		port:       port,
		timeFormat: timeFormat,
	}
}

func (w *WebIris) Run() {
	w.app.Listen(w.port,
		iris.WithoutInterruptHandler,
		iris.WithoutServerError(iris.ErrServerClosed),
		iris.WithOptimizations,
		iris.WithTimeFormat(w.timeFormat))
}
