package apidoc

import (
	"net/http"

	"github.com/kataras/iris/v12"
)

// Register 注册文档服务:
//
//	GET <prefix>            API 浏览页面(内嵌,无外部资源)
//	GET <prefix>/api.json   OpenAPI 3.0 定义
func Register(app *iris.Application, prefix string, config Config) {
	if app == nil {
		return
	}
	if prefix == "" {
		prefix = "/docs"
	}
	party := app.Party(prefix)
	party.Get("/", func(ctx iris.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Header("Cache-Control", "no-store")
		_, _ = ctx.WriteString(renderPage(prefix))
	})
	party.Get("", func(ctx iris.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Header("Cache-Control", "no-store")
		_, _ = ctx.WriteString(renderPage(prefix))
	})
	party.Get("/api.json", func(ctx iris.Context) {
		spec := BuildOpenAPI(store, config.Title, config.Version, config.Description)
		ctx.Header("Content-Type", "application/json")
		data, err := spec.ToJSON()
		if err != nil {
			ctx.StatusCode(http.StatusInternalServerError)
			_, _ = ctx.WriteString(`{"error":"build openapi failed"}`)
			return
		}
		_, _ = ctx.Write(data)
	})
}

// Config 文档服务配置。
type Config struct {
	Title       string
	Version     string
	Description string
}

// CRUD 一次注册标准 CRUD 接口并生成文档:
//
//	GET    <path>        列表(QueryParam page/page_size)
//	GET    <path>/{id}   详情
//	POST   <path>        创建
//	PUT    <path>/{id}   更新
//	DELETE <path>/{id}   删除
//
// handlers 依次为:List / Get / Create / Update / Delete(均为 iris.Handler)。
func CRUD(app *iris.Application, path string, model interface{},
	listHandler, getHandler, createHandler, updateHandler, deleteHandler iris.Handler) {
	if app == nil {
		return
	}
	entity := inferEntity(path)
	// 列表
	GET(app, path, listHandler,
		Summary("查询"+entity+"列表"),
		QueryParam("page", "int", false, "页码(从 1 开始)"),
		QueryParam("page_size", "int", false, "每页数量(默认 20,上限 100)"),
		QueryParam("keyword", "string", false, "关键字(可选)"),
		Responds(model),
	)
	// 详情
	GET(app, path+"/{id:int64}", getHandler,
		Summary("获取"+entity+"详情"),
		PathParam("id", "int64", true, "ID"),
		Responds(model),
	)
	// 创建
	POST(app, path, createHandler,
		Summary("创建"+entity),
		Body(model, true, "创建参数"),
		Responds(model),
	)
	// 更新
	PUT(app, path+"/{id:int64}", updateHandler,
		Summary("更新"+entity),
		PathParam("id", "int64", true, "ID"),
		Body(model, true, "更新参数"),
		Respond(200, "更新成功", nil),
	)
	// 删除
	DELETE(app, path+"/{id:int64}", deleteHandler,
		Summary("删除"+entity),
		PathParam("id", "int64", true, "ID"),
		Respond(200, "删除成功", nil),
	)
}
