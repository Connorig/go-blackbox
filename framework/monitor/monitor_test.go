package monitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/kataras/iris/v12"
)

// TestStatsShape 采集器返回结构完整(字段类型正确,不依赖具体平台值)。
func TestStatsShape(t *testing.T) {
	collector := NewCollector()
	stats, err := collector.Stats()
	if err != nil {
		t.Logf("partial collection error (platform dependent): %v", err)
	}
	if stats == nil {
		t.Fatal("stats must not be nil")
	}
	if stats.Hostname == "" || stats.Platform == "" || stats.GoVersion == "" {
		t.Fatalf("identity fields missing: %+v", stats)
	}
	if stats.Version == "" {
		t.Fatal("version missing")
	}
	if stats.Goroutines <= 0 {
		t.Fatalf("goroutines must be positive: %d", stats.Goroutines)
	}
	if stats.ProcessUptime < 0 {
		t.Fatalf("process uptime must not be negative: %d", stats.ProcessUptime)
	}
	// 使用率范围校验(有数据时)
	if stats.Memory.Total > 0 {
		if stats.Memory.UsagePercent < 0 || stats.Memory.UsagePercent > 100 {
			t.Fatalf("memory percent out of range: %f", stats.Memory.UsagePercent)
		}
		if stats.Memory.Used > stats.Memory.Total {
			t.Fatalf("memory used > total: %d > %d", stats.Memory.Used, stats.Memory.Total)
		}
	}
	if stats.Disk.Total > 0 && stats.Disk.UsagePercent > 100 {
		t.Fatalf("disk percent out of range: %f", stats.Disk.UsagePercent)
	}
	if stats.CPU.UsagePercent < 0 || stats.CPU.UsagePercent > 100 {
		t.Fatalf("cpu percent out of range: %f", stats.CPU.UsagePercent)
	}
}

// TestCPUStatsSamples 两次采样后 CPU 使用率在合法范围。
func TestCPUStatsSamples(t *testing.T) {
	collector := NewCollector()
	for i := 0; i < 3; i++ {
		stats, err := collector.Stats()
		if err != nil {
			t.Logf("collect error: %v", err)
		}
		if stats.CPU.UsagePercent < 0 || stats.CPU.UsagePercent > 100 {
			t.Fatalf("cpu percent out of range: %f", stats.CPU.UsagePercent)
		}
	}
}

// TestRegisterRoutes 页面与 API 路由注册。
func TestRegisterRoutes(t *testing.T) {
	app := iris.New()
	Register(app, "/monitor", Config{})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// 页面
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monitor", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("page status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "服务器资源监控") {
		t.Fatal("page content missing")
	}
	// 无斜杠兼容:iris 对尾斜杠返回 301 重定向到规范路径(浏览器自动跟随)
	recorder2 := httptest.NewRecorder()
	app.ServeHTTP(recorder2, httptest.NewRequest(http.MethodGet, "/monitor/", nil))
	if recorder2.Code != http.StatusOK && recorder2.Code != http.StatusMovedPermanently {
		t.Fatalf("page(/) status = %d", recorder2.Code)
	}

	// API
	recorder3 := httptest.NewRecorder()
	app.ServeHTTP(recorder3, httptest.NewRequest(http.MethodGet, "/monitor/api/stats", nil))
	if recorder3.Code != http.StatusOK {
		t.Fatalf("api status = %d", recorder3.Code)
	}
	var stats Stats
	if err := json.Unmarshal(recorder3.Body.Bytes(), &stats); err != nil {
		t.Fatalf("api response not valid json: %v", err)
	}
	if stats.Hostname == "" {
		t.Fatal("api response missing hostname")
	}
}

// TestRegisterWithAuth 配置 Auth 后 API 未授权返回 401,授权后 200。
func TestRegisterWithAuth(t *testing.T) {
	app := iris.New()
	authCalled := false
	Register(app, "/monitor", Config{
		Auth: func(ctx iris.Context) {
			authCalled = true
			if ctx.GetHeader("X-Token") != "secret" {
				ctx.StatusCode(http.StatusUnauthorized)
				_ = ctx.JSON(map[string]interface{}{"code": apperr.CodeAccessUnauthorized, "message": "unauthorized"})
				ctx.StopExecution()
				return
			}
			ctx.Next()
		},
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// 未授权
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monitor/api/stats", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", recorder.Code)
	}
	// 授权
	request := httptest.NewRequest(http.MethodGet, "/monitor/api/stats", nil)
	request.Header.Set("X-Token", "secret")
	recorder2 := httptest.NewRecorder()
	app.ServeHTTP(recorder2, request)
	if recorder2.Code != http.StatusOK {
		t.Fatalf("authorized status = %d", recorder2.Code)
	}
	if !authCalled {
		t.Fatal("auth middleware not invoked")
	}
}

// TestRegisterRateLimit API 超限返回 429(防接口轰炸)。
func TestRegisterRateLimit(t *testing.T) {
	app := iris.New()
	Register(app, "/monitor", Config{RatePerSecond: 1, Burst: 1})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	var lastCode int
	for i := 0; i < 5; i++ {
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monitor/api/stats", nil))
		lastCode = recorder.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", lastCode)
	}
}

// TestPageNoStoreHeader 页面禁止缓存。
func TestPageNoStoreHeader(t *testing.T) {
	app := iris.New()
	Register(app, "/monitor", Config{})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monitor", nil))
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control = %q", recorder.Header().Get("Cache-Control"))
	}
}
