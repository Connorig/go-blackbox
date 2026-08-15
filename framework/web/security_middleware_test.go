package webiris

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
)

// TestSQLGuardRejectsInjection 注入载荷被拦截。
func TestSQLGuardRejectsInjection(t *testing.T) {
	app := iris.New()
	app.Use(SQLGuard())
	app.Get("/api/v1/search", func(ctx iris.Context) {
		OK(ctx, map[string]string{"result": "ok"})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	payloads := []string{
		"1' OR '1'='1",
		"1 UNION SELECT 2",
		"x; DROP TABLE users",
	}
	for _, payload := range payloads {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+url.QueryEscape(payload), nil)
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("payload %q must be rejected, got %d", payload, recorder.Code)
		}
	}
}

// TestSQLGuardAllowsNormal 正常参数放行。
func TestSQLGuardAllowsNormal(t *testing.T) {
	app := iris.New()
	app.Use(SQLGuard())
	app.Get("/api/v1/search", func(ctx iris.Context) {
		OK(ctx, map[string]string{"result": ctx.URLParam("q")})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=hello%20world&page=1", nil)
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("normal request must pass, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// TestSQLGuardBodyInjection 请求体注入被拦截且后续 handler 仍可读 body。
func TestSQLGuardBodyInjection(t *testing.T) {
	app := iris.New()
	app.Use(SQLGuard())
	app.Post("/api/v1/order", func(ctx iris.Context) {
		OK(ctx, map[string]string{"result": "created"})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// 注入 body
	bad := httptest.NewRequest(http.MethodPost, "/api/v1/order", strings.NewReader(`{"remark":"' OR '1'='1"}`))
	bad.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, bad)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("injection body must be rejected, got %d", recorder.Code)
	}

	// 正常 body 且 handler 能读到(验证 body 恢复)
	good := httptest.NewRequest(http.MethodPost, "/api/v1/order", strings.NewReader(`{"remark":"hello"}`))
	good.Header.Set("Content-Type", "application/json")
	recorder2 := httptest.NewRecorder()
	app.ServeHTTP(recorder2, good)
	if recorder2.Code != http.StatusOK {
		t.Fatalf("normal body must pass, got %d: %s", recorder2.Code, recorder2.Body.String())
	}
}

// TestBodyLimit 超大请求体返回 413。
func TestBodyLimit(t *testing.T) {
	app := iris.New()
	app.Use(BodyLimit(1024)) // 1KB
	app.Post("/api/v1/upload", func(ctx iris.Context) {
		OK(ctx, map[string]string{"result": "ok"})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	big := strings.Repeat("a", 4096)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/upload", strings.NewReader(big))
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("big body must be rejected with 413, got %d", recorder.Code)
	}
}

// TestTimeoutSlowHandler 慢 handler 返回 504。
func TestTimeoutSlowHandler(t *testing.T) {
	app := iris.New()
	app.Use(Timeout(200 * time.Millisecond))
	app.Get("/api/v1/slow", func(ctx iris.Context) {
		time.Sleep(2 * time.Second)
		OK(ctx, map[string]string{"result": "late"})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/slow", nil)
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("slow handler must be cut with 504, got %d", recorder.Code)
	}
}
