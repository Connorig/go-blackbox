package webiris

import (
	"errors"
	"net/http"
	"testing"

	"github.com/Connorig/go-blackbox/apputils/apperr"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

// TestErrorHandlerRecoversPanic 验证 panic 被转换为 500 统一响应。
func TestErrorHandlerRecoversPanic(t *testing.T) {
	app := iris.New()
	app.Use(ErrorHandler)
	app.Get("/panic", func(ctx iris.Context) {
		panic("boom")
	})

	e := httptest.New(t, app)
	e.GET("/panic").Expect().Status(500).
		JSON().Object().ValueEqual("code", 500).ValueEqual("message", "internal server error")
}

// TestRespondErrorWithAppError 验证业务错误按自身状态码输出。
func TestRespondErrorWithAppError(t *testing.T) {
	app := iris.New()
	app.Get("/not-found", func(ctx iris.Context) {
		RespondError(ctx, apperr.New(http.StatusNotFound, 40401, "order not found"))
	})
	app.Get("/ok", func(ctx iris.Context) {
		RespondError(ctx, nil)
	})

	e := httptest.New(t, app)
	e.GET("/not-found").Expect().Status(404).
		JSON().Object().ValueEqual("code", 40401).ValueEqual("message", "order not found")
	e.GET("/ok").Expect().Status(200).JSON().Object().ValueEqual("code", 0)
}

// TestRespondErrorWithUnknownError 验证未知错误转换为 500。
func TestRespondErrorWithUnknownError(t *testing.T) {
	app := iris.New()
	app.Get("/boom", func(ctx iris.Context) {
		RespondError(ctx, errors.New("unexpected failure"))
	})

	e := httptest.New(t, app)
	e.GET("/boom").Expect().Status(500).
		JSON().Object().ValueEqual("code", 500)
}

// TestRegisterPprof 验证 pprof 诊断端点可用。
func TestRegisterPprof(t *testing.T) {
	app := iris.New()
	RegisterPprof(app)

	e := httptest.New(t, app)
	e.GET("/debug/pprof/").Expect().Status(200)
	e.GET("/debug/pprof/goroutine").Expect().Status(200)
	e.GET("/debug/pprof/heap").Expect().Status(200)
}
