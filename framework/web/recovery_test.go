package webiris

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestPanicRecovery 验证业务 handler panic 时返回统一 500,进程不崩溃。
func TestPanicRecovery(t *testing.T) {
	app := iris.New()
	app.Use(PanicRecovery())
	app.Get("/boom", func(ctx iris.Context) {
		panic("business panic")
	})
	app.Get("/ok", func(ctx iris.Context) {
		OK(ctx, map[string]interface{}{"healthy": true})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	response, err := http.Get(server.URL + "/boom")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("panic must map to 500, got %d", response.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode body failed: %v", err)
	}
	if body["code"] != "B0001" {
		t.Fatalf("panic must map to CodeSystemError, got %v", body["code"])
	}

	// 正常请求不受影响
	response2, err := http.Get(server.URL + "/ok")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response2.Body.Close()
	if response2.StatusCode != http.StatusOK {
		t.Fatalf("normal request must still work, got %d", response2.StatusCode)
	}
}

// TestPanicRecoveryMidChain 验证恢复后后续中间件不执行(已短路)。
func TestPanicRecoveryMidChain(t *testing.T) {
	app := iris.New()
	var afterCalled bool
	app.Use(PanicRecovery())
	app.Use(func(ctx iris.Context) {
		ctx.Next()
		afterCalled = true // 在 panic 路由中不应执行到
	})
	app.Get("/boom", func(ctx iris.Context) {
		panic("again")
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	response, err := http.Get(server.URL + "/boom")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	response.Body.Close()
	_ = strings.TrimSpace
	if response.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", response.StatusCode)
	}
	if afterCalled {
		t.Fatal("middleware after panic must not run")
	}
}
