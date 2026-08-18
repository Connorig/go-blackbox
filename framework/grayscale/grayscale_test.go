package grayscale

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestHitBoundaries 边界:0 全旧、1 全新。
func TestHitBoundaries(t *testing.T) {
	if New(0).Hit(nil) {
		t.Fatal("ratio 0 must never hit")
	}
	if !New(1).Hit(nil) {
		t.Fatal("ratio 1 must always hit")
	}
	var strategy *Strategy
	if strategy.Hit(nil) {
		t.Fatal("nil strategy must not hit")
	}
}

// TestUserStableHit 用户稳定分流:同一用户结果恒定。
func TestUserStableHit(t *testing.T) {
	strategy := New(0.3, func(ctx iris.Context) string {
		return ctx.GetHeader("X-User-ID")
	})
	app := iris.New()
	app.Get("/api", strategy.Route(
		func(ctx iris.Context) { _, _ = ctx.WriteString("new") },
		func(ctx iris.Context) { _, _ = ctx.WriteString("old") },
	))
	_ = app.Build()
	server := httptest.NewServer(app)
	defer server.Close()

	// 同一用户多次请求结果一致
	first := callUser(t, server.URL, "user-100")
	for i := 0; i < 10; i++ {
		if got := callUser(t, server.URL, "user-100"); got != first {
			t.Fatalf("user stable violated: %q then %q", first, got)
		}
	}
	// 不同用户可能分流到不同版本(统计性,不强制断言)
	_ = callUser(t, server.URL, "user-999")
}

// TestRatioRoughDistribution 比例分流统计:500 次请求,新版本占比接近 ratio(±15%)。
func TestRatioRoughDistribution(t *testing.T) {
	strategy := New(0.5)
	app := iris.New()
	app.Get("/api", strategy.Route(
		func(ctx iris.Context) { _, _ = ctx.WriteString("new") },
		func(ctx iris.Context) { _, _ = ctx.WriteString("old") },
	))
	_ = app.Build()
	server := httptest.NewServer(app)
	defer server.Close()

	newCount := 0
	total := 500
	for i := 0; i < total; i++ {
		response, err := http.Get(server.URL + "/api")
		if err != nil {
			t.Fatal(err)
		}
		buffer := make([]byte, 16)
		n, _ := response.Body.Read(buffer)
		response.Body.Close()
		if strings.Contains(string(buffer[:n]), "new") {
			newCount++
		}
	}
	ratio := float64(newCount) / float64(total)
	if ratio < 0.35 || ratio > 0.65 {
		t.Fatalf("ratio = %.2f, want ~0.5 (±0.15)", ratio)
	}
}

// callUser 以指定用户请求接口,返回响应体。
func callUser(t *testing.T, baseURL, userID string) string {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, baseURL+"/api", nil)
	request.Header.Set("X-User-ID", userID)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	buffer := make([]byte, 16)
	n, _ := response.Body.Read(buffer)
	return string(buffer[:n])
}
