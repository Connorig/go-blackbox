package webiris

import (
	"testing"

	"github.com/Connorig/go-blackbox/framework/apidoc"
	"github.com/kataras/iris/v12"
)

// TestRegisterRoutes 路由切片批量注册(含 apidoc 文档)。
func TestRegisterRoutes(t *testing.T) {
	resetDocStore()
	app := iris.New()
	routes := []Route{
		{Method: "GET", Path: "/api/v1/orders", Handler: func(ctx iris.Context) { _ = ctx.JSON("ok") }},
		{Method: "POST", Path: "/api/v1/orders", Handler: func(ctx iris.Context) { _ = ctx.JSON("ok") },
			Doc: []apidoc.Option{apidoc.Summary("创建订单")}},
	}
	RegisterRoutes(app, routes)
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	// 文档收集器应收到 1 条(带 Doc 的)
	if len(apidoc.Store().Operations()) != 1 {
		t.Fatalf("expected 1 documented operation, got %d", len(apidoc.Store().Operations()))
	}
	operation := apidoc.Store().Operations()[0]
	if operation.Method != "POST" || operation.Summary != "创建订单" {
		t.Fatalf("operation wrong: %+v", operation)
	}
}

// TestRegisterRoutesMultiGroup 多分组组合。
func TestRegisterRoutesMultiGroup(t *testing.T) {
	resetDocStore()
	app := iris.New()
	orderRoutes := []Route{{Method: "GET", Path: "/api/v1/orders", Handler: func(ctx iris.Context) {}}}
	userRoutes := []Route{{Method: "GET", Path: "/api/v1/users", Handler: func(ctx iris.Context) {}}}
	RegisterRoutes(app, orderRoutes, userRoutes)
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(apidoc.Store().Operations()) != 0 {
		t.Fatalf("no doc options means no operations: %d", len(apidoc.Store().Operations()))
	}
}

// TestRegisterRoutesSkipInvalid 非法路由跳过不 panic。
func TestRegisterRoutesSkipInvalid(t *testing.T) {
	app := iris.New()
	RegisterRoutes(app, []Route{{Method: "", Path: "", Handler: nil}})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
}

// resetDocStore 重置全局文档收集器(与 apidoc 测试隔离)。
func resetDocStore() {
	// 通过重新赋值全局 store 实现(apidoc 包内部,此处用导出方法重建)
	apidoc.ResetStoreForTest()
}
