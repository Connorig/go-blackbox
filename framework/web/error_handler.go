package webiris

import (
	"net/http"
	"net/http/pprof"

	"github.com/Connorig/go-blackbox/component/error"
	"github.com/Connorig/go-blackbox/framework/log"
	"github.com/kataras/iris/v12"
)

// ErrorHandler 是全局错误处理中间件:
// 捕获处理器 panic 并输出 500 统一响应,避免框架默认 HTML 错误页。
// 应注册在所有路由之前:app.Use(webiris.ErrorHandler)。
func ErrorHandler(ctx iris.Context) {
	defer func() {
		if recovered := recover(); recovered != nil {
			zaplog.WithComponent("web").Errorw("panic recovered in http handler",
				"method", ctx.Method(), "path", ctx.Path(),
				"panic", recovered, "request_id", ctx.Values().GetString("request_id"))
			if !ctx.IsStopped() {
				Fail(ctx, http.StatusInternalServerError, apperr.CodeSystemError, "internal server error")
			}
		}
	}()
	ctx.Next()
}

// RespondError 把错误转为统一响应:
// apperr.Error 按自身状态码与业务码输出;其他错误输出 500(服务端日志记录原始错误)。
func RespondError(ctx iris.Context, err error) {
	if err == nil {
		OK(ctx, nil)
		return
	}
	appError := apperr.From(err)
	if appError.Cause != nil {
		zaplog.WithComponent("web").Errorw("request failed",
			"method", ctx.Method(), "path", ctx.Path(),
			"code", appError.Code, "error", appError.Cause,
			"request_id", ctx.Values().GetString("request_id"))
	}
	Fail(ctx, appError.HTTPStatus, appError.Code, appError.Message)
}

// RegisterPprof 注册 /debug/pprof 诊断端点(heap、goroutine、profile、trace 等)。
// 生产环境建议通过配置开关决定是否注册。
func RegisterPprof(app *iris.Application) {
	if app == nil {
		return
	}
	party := app.Party("/debug/pprof")
	party.Any("/", func(ctx iris.Context) {
		pprof.Index(ctx.ResponseWriter(), ctx.Request())
	})
	party.Any("/cmdline", func(ctx iris.Context) {
		pprof.Cmdline(ctx.ResponseWriter(), ctx.Request())
	})
	party.Any("/profile", func(ctx iris.Context) {
		pprof.Profile(ctx.ResponseWriter(), ctx.Request())
	})
	party.Any("/symbol", func(ctx iris.Context) {
		pprof.Symbol(ctx.ResponseWriter(), ctx.Request())
	})
	party.Any("/trace", func(ctx iris.Context) {
		pprof.Trace(ctx.ResponseWriter(), ctx.Request())
	})
	// 其余 profile 类型(heap/goroutine/allocs 等)由 Index 处理
	party.Any("/{action:path}", func(ctx iris.Context) {
		pprof.Index(ctx.ResponseWriter(), ctx.Request())
	})
}
