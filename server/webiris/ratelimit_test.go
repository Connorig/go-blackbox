package webiris

import (
	"testing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

// TestLimitGlobalRejectsOverBurst 验证全局限流在突发容量耗尽后返回 429。
func TestLimitGlobalRejectsOverBurst(t *testing.T) {
	app := iris.New()
	app.Use(Limit(1, 2, nil))
	app.Get("/limited", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})

	e := httptest.New(t, app)
	e.GET("/limited").Expect().Status(200).Body().IsEqual("ok")
	e.GET("/limited").Expect().Status(200)
	e.GET("/limited").Expect().Status(429).
		JSON().Object().ValueEqual("code", 429).ValueEqual("message", "too many requests")
}

// TestLimitPerKeyIsIndependent 验证按维度限流时不同 key 互不影响。
func TestLimitPerKeyIsIndependent(t *testing.T) {
	app := iris.New()
	app.Use(Limit(1, 1, func(ctx iris.Context) string {
		return ctx.GetHeader("X-User-Key")
	}))
	app.Get("/limited", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})

	e := httptest.New(t, app)
	// user-a 突发容量耗尽
	e.GET("/limited").WithHeader("X-User-Key", "user-a").Expect().Status(200)
	e.GET("/limited").WithHeader("X-User-Key", "user-a").Expect().Status(429)
	// user-b 不受影响
	e.GET("/limited").WithHeader("X-User-Key", "user-b").Expect().Status(200)
}
