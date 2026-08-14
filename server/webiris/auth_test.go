package webiris

import (
	"testing"

	"github.com/Connorig/go-blackbox/apputils/apptoken"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

// testSecret 是认证中间件测试使用的密钥（≥32 字节）。
const testSecret = "0123456789abcdef0123456789abcdef"

// TestAuthRejectsMissingHeader 验证缺失 Authorization 头返回 401。
func TestAuthRejectsMissingHeader(t *testing.T) {
	if err := apptoken.SetSecretKey(testSecret); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	app := iris.New()
	app.Use(Auth())
	app.Get("/protected", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})

	e := httptest.New(t, app)
	e.GET("/protected").Expect().Status(401).
		JSON().Object().ValueEqual("code", 401)
}

// TestAuthRejectsInvalidToken 验证无效 token 返回 401。
func TestAuthRejectsInvalidToken(t *testing.T) {
	if err := apptoken.SetSecretKey(testSecret); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	app := iris.New()
	app.Use(Auth())
	app.Get("/protected", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})

	e := httptest.New(t, app)
	e.GET("/protected").
		WithHeader("Authorization", "Bearer invalid-token").
		Expect().Status(401)
}

// TestAuthAcceptsValidTokenAndExposesIdentity 验证有效 token 通过并暴露用户身份。
func TestAuthAcceptsValidTokenAndExposesIdentity(t *testing.T) {
	if err := apptoken.SetSecretKey(testSecret); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	accessToken, _, err := apptoken.GenToken(42, "user@example.com")
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	app := iris.New()
	app.Use(Auth())
	app.Get("/me", func(ctx iris.Context) {
		ctx.Writef("id=%d email=%s", UserID(ctx), UserEmail(ctx))
	})

	e := httptest.New(t, app)
	e.GET("/me").
		WithHeader("Authorization", "Bearer "+accessToken).
		Expect().Status(200).
		Body().IsEqual("id=42 email=user@example.com")
}

// TestAuthWhitelistBypassesAuthentication 验证白名单路径无需认证。
func TestAuthWhitelistBypassesAuthentication(t *testing.T) {
	if err := apptoken.SetSecretKey(testSecret); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	app := iris.New()
	app.Use(Auth(AuthConfig{Whitelist: []string{"/health", "/login"}}))
	app.Get("/health/live", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})
	app.Get("/protected", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})

	e := httptest.New(t, app)
	e.GET("/health/live").Expect().Status(200).Body().IsEqual("ok")
	e.GET("/protected").Expect().Status(401)
}
