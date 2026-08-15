package webiris

import (
	"errors"
	"testing"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

// TestRequestIDAndSecurityHeaders 验证 Request ID 生成与安全响应头。
func TestRequestIDAndSecurityHeaders(t *testing.T) {
	app := iris.New()
	app.Use(RequestID, SecurityHeaders)
	app.Get("/test", func(ctx iris.Context) {
		if _, err := ctx.WriteString("ok"); err != nil {
			t.Errorf("write response failed: %v", err)
		}
	})

	e := httptest.New(t, app)
	response := e.GET("/test").Expect().Status(200)
	response.Header(RequestIDHeader).NotEmpty()
	response.Header("X-Content-Type-Options").Equal("nosniff")
	response.Header("X-Frame-Options").Equal("DENY")
	response.Header("Referrer-Policy").Equal("no-referrer")
}

// TestRequestIDPassesThrough 验证已有 Request ID 会被透传。
func TestRequestIDPassesThrough(t *testing.T) {
	app := iris.New()
	app.Use(RequestID)
	app.Get("/test", func(ctx iris.Context) {
		ctx.WriteString(ctx.Values().GetString("request_id"))
	})

	e := httptest.New(t, app)
	e.GET("/test").
		WithHeader(RequestIDHeader, "trace-123").
		Expect().Status(200).
		Header(RequestIDHeader).Equal("trace-123")
}

// TestCORSWildcard 验证默认允许所有来源并输出 CORS 方法/头声明。
func TestCORSWildcard(t *testing.T) {
	app := iris.New()
	app.Use(CORS())
	app.Get("/test", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})

	e := httptest.New(t, app)
	response := e.GET("/test").
		WithHeader("Origin", "https://example.com").
		Expect().Status(200)
	response.Header("Access-Control-Allow-Origin").Equal("*")
	response.Header("Access-Control-Allow-Methods").Equal("GET, POST, PUT, PATCH, DELETE, OPTIONS")
	response.Header("Access-Control-Allow-Headers").Equal("Content-Type, Authorization, X-Request-ID")
}

// TestCORSAllowListedOrigin 验证白名单之外来源不返回允许头。
func TestCORSAllowListedOrigin(t *testing.T) {
	app := iris.New()
	app.Use(CORS("https://trusted.example.com"))
	app.Get("/test", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})

	e := httptest.New(t, app)
	e.GET("/test").
		WithHeader("Origin", "https://trusted.example.com").
		Expect().Status(200).
		Header("Access-Control-Allow-Origin").Equal("https://trusted.example.com")
	e.GET("/test").
		WithHeader("Origin", "https://evil.example.com").
		Expect().Status(200).
		Header("Access-Control-Allow-Origin").Empty()
}

// TestRegisterHealth 验证存活与就绪探针端点。
func TestRegisterHealth(t *testing.T) {
	app := iris.New()
	RegisterHealth(app, func() error { return nil })
	e := httptest.New(t, app)
	e.GET("/health/live").Expect().Status(200).Body().IsEqual("ok")
	e.GET("/health/ready").Expect().Status(200).Body().IsEqual("ready")
}

// TestRegisterHealthReadyFailure 验证就绪回调失败时返回 503。
func TestRegisterHealthReadyFailure(t *testing.T) {
	app := iris.New()
	RegisterHealth(app, func() error { return errors.New("database unavailable") })
	e := httptest.New(t, app)
	e.GET("/health/ready").Expect().Status(503)
}

// TestUnifiedResponse 验证统一响应结构。
func TestUnifiedResponse(t *testing.T) {
	app := iris.New()
	app.Get("/ok", func(ctx iris.Context) {
		OK(ctx, map[string]string{"key": "value"})
	})
	app.Get("/fail", func(ctx iris.Context) {
		Fail(ctx, 400, apperr.CodeRequestParamError, "bad request")
	})

	e := httptest.New(t, app)
	e.GET("/ok").Expect().Status(200).
		JSON().Object().
		ValueEqual("code", "00000").
		ValueEqual("message", "ok").
		Value("data").Object().ValueEqual("key", "value")

	e.GET("/fail").Expect().Status(400).
		JSON().Object().
		ValueEqual("code", "A0400").
		ValueEqual("message", "bad request")
}
