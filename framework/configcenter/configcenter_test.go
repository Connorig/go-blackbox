package configcenter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestFetch 拉取配置(httptest 模拟 Nacos)。
func TestFetch(t *testing.T) {
	var serverConfig atomic.Value
	serverConfig.Store("version=1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("dataId") != "app.toml" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(serverConfig.Load().(string)))
	}))
	defer server.Close()

	client := NewClient(server.URL, "app.toml", "DEFAULT_GROUP")
	content, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if content != "version=1" {
		t.Fatalf("content = %q", content)
	}
}

// TestFetchNotFound 配置不存在错误。
func TestFetchNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := NewClient(server.URL, "missing.toml", "")
	if _, err := client.Fetch(context.Background()); err == nil {
		t.Fatal("not found must fail")
	}
}

// TestWatchChange 监听配置变更回调。
func TestWatchChange(t *testing.T) {
	var serverConfig atomic.Value
	serverConfig.Store("v1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(serverConfig.Load().(string)))
	}))
	defer server.Close()

	client := NewClient(server.URL, "app.toml", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var received []string
	done := make(chan struct{})
	go func() {
		_ = client.Watch(ctx, 100*time.Millisecond, func(content string) {
			mu.Lock()
			received = append(received, content)
			mu.Unlock()
			if len(received) >= 2 {
				close(done)
			}
		})
	}()

	time.Sleep(50 * time.Millisecond) // 等首次拉取
	serverConfig.Store("v2")          // 变更配置

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("watch change not received")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(received) < 2 || received[0] != "v1" || received[1] != "v2" {
		t.Fatalf("received = %v", received)
	}
}

// TestNilSafe nil 安全。
func TestNilSafe(t *testing.T) {
	var client *Client
	if _, err := client.Fetch(context.Background()); err == nil {
		t.Fatal("nil fetch must fail")
	}
	if err := client.Watch(context.Background(), time.Second, nil); err == nil {
		t.Fatal("nil watch must fail")
	}
}
