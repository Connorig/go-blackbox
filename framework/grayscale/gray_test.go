package grayscale

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestRouteHeaderAndStats 验证响应头标记与命中统计。
func TestRouteHeaderAndStats(t *testing.T) {
	strategy := New(0.5)
	app := iris.New()
	app.Get("/api", strategy.Route(
		func(ctx iris.Context) { _, _ = ctx.WriteString("new") },
		func(ctx iris.Context) { _, _ = ctx.WriteString("old") },
	))
	_ = app.Build()
	server := httptest.NewServer(app)
	defer server.Close()

	versions := map[string]int{}
	for i := 0; i < 100; i++ {
		response, err := http.Get(server.URL + "/api")
		if err != nil {
			t.Fatal(err)
		}
		versions[response.Header.Get(DefaultHeaderName)]++
		response.Body.Close()
	}
	if versions["new"] == 0 || versions["old"] == 0 {
		t.Fatalf("both versions must be served with ratio 0.5, got %v", versions)
	}

	stats := strategy.Stats()
	if stats.Total != 100 {
		t.Fatalf("total must be 100, got %d", stats.Total)
	}
	if int(stats.NewHits) != versions["new"] || int(stats.OldHits) != versions["old"] {
		t.Fatalf("stats mismatch: %+v vs headers %v", stats, versions)
	}
	if stats.Ratio <= 0 || stats.Ratio >= 1 {
		t.Fatalf("ratio must be in (0,1), got %.3f", stats.Ratio)
	}
}

// TestHeaderDisabled 验证关闭标记头。
func TestHeaderDisabled(t *testing.T) {
	strategy := New(0.5).WithHeaderName("")
	app := iris.New()
	app.Get("/api", strategy.Route(
		func(ctx iris.Context) { _, _ = ctx.WriteString("new") },
		func(ctx iris.Context) { _, _ = ctx.WriteString("old") },
	))
	_ = app.Build()
	server := httptest.NewServer(app)
	defer server.Close()

	response, err := http.Get(server.URL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if header := response.Header.Get(DefaultHeaderName); header != "" {
		t.Fatalf("header must be disabled, got %q", header)
	}
	if header := response.Header.Get("X-Gray"); header != "" {
		t.Fatalf("custom header must not appear, got %q", header)
	}
}

// TestStatsNilSafe 验证 nil 与无请求统计。
func TestStatsNilSafe(t *testing.T) {
	var strategy *Strategy
	if stats := strategy.Stats(); stats.Total != 0 {
		t.Fatalf("nil strategy stats must be zero, got %+v", stats)
	}
	strategy = New(1)
	if stats := strategy.Stats(); stats.Total != 0 || stats.Ratio != 0 {
		t.Fatalf("fresh strategy stats must be zero, got %+v", stats)
	}
	// 全量新版本
	app := iris.New()
	app.Get("/api", strategy.Route(
		func(ctx iris.Context) { _, _ = ctx.WriteString("new") },
		func(ctx iris.Context) { _, _ = ctx.WriteString("old") },
	))
	_ = app.Build()
	server := httptest.NewServer(app)
	defer server.Close()
	for i := 0; i < 3; i++ {
		response, err := http.Get(server.URL + "/api")
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	stats := strategy.Stats()
	if stats.Total != 3 || stats.NewHits != 3 || stats.Ratio != 1 {
		t.Fatalf("full-ratio stats wrong: %+v", stats)
	}
}
