package webiris

import (
	"github.com/Connorig/go-blackbox/framework/apidoc"
	"github.com/kataras/iris/v12"
)

// Route 路由定义(业务项目分组注册用)。
// 业务项目在内部 router 包定义路由函数返回 Route 切片,
// main 中一行挂载,避免全部路由堆在 main。
//
//	// internal/router/order.go(业务项目,按功能分组)
//	func OrderRoutes() []webiris.Route {
//	    return []webiris.Route{
//	        {Method: "GET", Path: "/api/v1/orders", Handler: handler.ListOrder},
//	        {Method: "POST", Path: "/api/v1/orders", Handler: handler.CreateOrder,
//	         Doc: []apidoc.Option{apidoc.Body(&model.Order{}, true, "创建"), apidoc.Responds(&model.Order{})}},
//	    }
//	}
//
//	// internal/router/router.go(组合分组:一级/二级路由按类型功能组织)
//	func All() []webiris.Route {
//	    var routes []webiris.Route
//	    routes = append(routes, OrderRoutes()...)
//	    routes = append(routes, UserRoutes()...)
//	    return routes
//	}
//
//	// main.go(一行挂载)
//	webiris.RegisterRoutes(app, router.All())
type Route struct {
	Method  string          // HTTP 方法:GET/POST/PUT/DELETE
	Path    string          // 路由路径(支持 iris 参数语法 /{id:int64})
	Handler iris.Handler    // 处理函数
	Doc     []apidoc.Option // 可选:接口文档描述(apidoc.Summary 等);nil 时仅注册路由
}

// RegisterRoutes 批量注册路由(可选附带 apidoc 文档)。
// 传多个路由切片可组合分组(如 router.OrderRoutes(), router.UserRoutes())。
func RegisterRoutes(app *iris.Application, routeGroups ...[]Route) {
	if app == nil {
		return
	}
	for _, routes := range routeGroups {
		for _, route := range routes {
			if route.Handler == nil || route.Method == "" || route.Path == "" {
				continue
			}
			if len(route.Doc) > 0 {
				apidoc.Handle(app, route.Method, route.Path, route.Handler, route.Doc...)
				continue
			}
			app.Handle(route.Method, route.Path, route.Handler)
		}
	}
}
