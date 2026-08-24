package grayscale

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestStatsHandler 验证统计接口返回完整 JSON。
func TestStatsHandler(t *testing.T) {
	strategy := New(1) // 全量新版本
	app := iris.New()
	app.Get("/api", strategy.Route(
		func(ctx iris.Context) { _, _ = ctx.WriteString("new") },
		func(ctx iris.Context) { _, _ = ctx.WriteString("old") },
	))
	app.Get("/gray/stats", strategy.StatsHandler())
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	// 产生 3 次命中
	for i := 0; i < 3; i++ {
		response, err := http.Get(server.URL + "/api")
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}

	response, err := http.Get(server.URL + "/gray/stats")
	if err != nil {
		t.Fatalf("stats request failed: %v", err)
	}
	defer response.Body.Close()
	var body map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data missing: %v", body)
	}
	if data["total"] != float64(3) || data["new_hits"] != float64(3) {
		t.Fatalf("unexpected stats: %v", data)
	}
	if data["ratio"] != float64(1) || data["config_ratio"] != float64(1) {
		t.Fatalf("unexpected ratios: %v", data)
	}
}

// TestStatsHandlerNil 验证 nil strategy 返回错误而非 panic。
func TestStatsHandlerNil(t *testing.T) {
	var strategy *Strategy
	app := iris.New()
	app.Get("/gray/stats", strategy.StatsHandler())
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	response, err := http.Get(server.URL + "/gray/stats")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 500 {
		t.Fatalf("nil strategy must return 500, got %d", response.StatusCode)
	}
}
