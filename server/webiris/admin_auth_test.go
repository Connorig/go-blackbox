package webiris

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Connorig/go-blackbox/apputils/apptoken"
	"github.com/kataras/iris/v12"
	irishttptest "github.com/kataras/iris/v12/httptest"
)

// get 发送 GET 请求并返回响应。
func get(t *testing.T, server *httptest.Server, path string) *http.Response {
	t.Helper()
	response, err := http.Get(server.URL + path)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

// postJSON 发送 JSON POST 请求并返回响应。
func postJSON(t *testing.T, server *httptest.Server, path string, body interface{}) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}
	response, err := http.Post(server.URL+path, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

// readBody 读取响应体。
func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	return string(content)
}

// TestAdminBuiltinEndpoints 验证管理服务内置 API：pprof、metrics、日志级别切换。
func TestAdminBuiltinEndpoints(t *testing.T) {
	admin := NewAdmin()
	if err := admin.app.Build(); err != nil {
		t.Fatalf("build admin app failed: %v", err)
	}
	server := httptest.NewServer(admin.app)
	t.Cleanup(server.Close)

	if response := get(t, server, "/debug/pprof/"); response.StatusCode != 200 {
		t.Fatalf("pprof index must return 200, got %d", response.StatusCode)
	}
	if response := get(t, server, "/metrics"); response.StatusCode != 200 {
		t.Fatalf("metrics must return 200, got %d", response.StatusCode)
	}
	if response := postJSON(t, server, "/cl", map[string]string{"level": "debug"}); response.StatusCode != 200 {
		t.Fatalf("log level switch must return 200, got %d", response.StatusCode)
	}
	if response := postJSON(t, server, "/cl", map[string]string{"level": "invalid"}); response.StatusCode != 400 {
		t.Fatalf("invalid log level must return 400, got %d", response.StatusCode)
	}
}

// TestAdminBusinessRoutes 验证业务管理路由注册且不影响内置 API。
func TestAdminBusinessRoutes(t *testing.T) {
	admin := NewAdmin()
	admin.RegisterRoutes(func(app *iris.Application) {
		app.Get("/demo/ping", func(ctx iris.Context) {
			OK(ctx, map[string]bool{"ok": true})
		})
	})
	if err := admin.app.Build(); err != nil {
		t.Fatalf("build admin app failed: %v", err)
	}
	server := httptest.NewServer(admin.app)
	t.Cleanup(server.Close)

	response := get(t, server, "/demo/ping")
	if response.StatusCode != 200 {
		t.Fatalf("business admin route must return 200, got %d", response.StatusCode)
	}
	if body := readBody(t, response); !bytes.Contains([]byte(body), []byte(`"ok":true`)) {
		t.Fatalf("unexpected business route body: %s", body)
	}
	if response := get(t, server, "/metrics"); response.StatusCode != 200 {
		t.Fatalf("builtin metrics must still work, got %d", response.StatusCode)
	}
}

// TestAdminDisabledFeatures 验证可关闭的内置能力。
func TestAdminDisabledFeatures(t *testing.T) {
	admin := NewAdminWithConfig(AdminConfig{
		EnablePprof:    false,
		EnableMetrics:  false,
		EnableLogLevel: false,
	})
	if err := admin.app.Build(); err != nil {
		t.Fatalf("build admin app failed: %v", err)
	}
	server := httptest.NewServer(admin.app)
	t.Cleanup(server.Close)

	if response := get(t, server, "/debug/pprof/"); response.StatusCode != 404 {
		t.Fatalf("disabled pprof must return 404, got %d", response.StatusCode)
	}
	if response := get(t, server, "/metrics"); response.StatusCode != 404 {
		t.Fatalf("disabled metrics must return 404, got %d", response.StatusCode)
	}
	if response := postJSON(t, server, "/cl", map[string]string{"level": "debug"}); response.StatusCode != 404 {
		t.Fatalf("disabled log level api must return 404, got %d", response.StatusCode)
	}
}

// TestAuthWithScope 验证 scope 权限校验：无 scope token 403，匹配 scope 通过。
func TestAuthWithScope(t *testing.T) {
	if err := apptoken.SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set secret failed: %v", err)
	}
	noScopeToken, _, err := apptoken.GenToken(1, "user@example.com")
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}
	adminToken, _, err := apptoken.GenTokenWithScope(1, "user@example.com", "admin,order:write")
	if err != nil {
		t.Fatalf("generate scoped token failed: %v", err)
	}

	app := iris.New()
	app.Use(Auth(AuthConfig{Scope: "admin"}))
	app.Get("/admin", func(ctx iris.Context) {
		ctx.WriteString("ok")
	})

	e := irishttptest.New(t, app)
	e.GET("/admin").WithHeader("Authorization", "Bearer "+adminToken).
		Expect().Status(200).Body().IsEqual("ok")
	e.GET("/admin").WithHeader("Authorization", "Bearer "+noScopeToken).
		Expect().Status(403).JSON().Object().ValueEqual("message", "insufficient scope")
}

// TestAuthWithoutScopeConfig 验证未配置 scope 时不影响认证行为。
func TestAuthWithoutScopeConfig(t *testing.T) {
	if err := apptoken.SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set secret failed: %v", err)
	}
	accessToken, _, err := apptoken.GenToken(1, "user@example.com")
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}
	app := iris.New()
	app.Use(Auth())
	app.Get("/any", func(ctx iris.Context) {
		ctx.Writef("id=%d", UserID(ctx))
	})
	e := irishttptest.New(t, app)
	e.GET("/any").WithHeader("Authorization", "Bearer "+accessToken).
		Expect().Status(200).Body().IsEqual("id=1")
}
