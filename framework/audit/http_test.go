package oplog

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
)

// TestQueryHandler 验证审计查询 HTTP 接口(需 Redis)。
func TestQueryHandler(t *testing.T) {
	addr := os.Getenv("GO_BLACKBOX_REDIS_ADDR")
	if addr == "" {
		t.Skip("Redis integration test requires GO_BLACKBOX_REDIS_ADDR environment variable")
	}
	client := redisClient(t)
	const key = "go-blackbox-test-oplog-http"
	_ = client.Del(context.Background(), key)

	sink := NewRedisListSink(client, key, 0)
	if err := sink.Write(context.Background(), []Entry{
		{Time: time.Now(), UserID: 1, Method: "GET", Path: "/api/v1/orders", Status: 200},
		{Time: time.Now(), UserID: 2, Method: "POST", Path: "/api/v1/orders", Status: 201},
	}); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	app := iris.New()
	app.Get("/ops/audit", QueryHandler(client, key))
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/ops/audit?offset=0&count=10")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	content, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		t.Fatalf("read body failed: %v", readErr)
	}
	body := string(content)
	if !strings.Contains(body, `"total":2`) || !strings.Contains(body, `"method"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}
