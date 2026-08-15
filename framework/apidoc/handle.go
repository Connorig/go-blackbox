package apidoc

import (
	"github.com/kataras/iris/v12"
)

// Handle 注册任意方法路由并收集文档(webiris.Route.Doc 使用)。
// 与 GET/POST/PUT/DELETE 等价,方法名由调用方指定。
func Handle(app *iris.Application, method, path string, handler iris.Handler, options ...Option) {
	register(app, method, path, handler, options...)
}
