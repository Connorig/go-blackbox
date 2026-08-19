package configcenter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// nacosServer 模拟 Nacos 风格配置服务:content 可通过原子替换实现变更。
func nacosServer(t *testing.T) (*httptest.Server, *sync.Map) {
	t.Helper()
	var store sync.Map
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/nacos/v1/cs/configs" {
			http.NotFound(writer, request)
			return
		}
		content, ok := store.Load("content")
		if !ok {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte(content.(string)))
	}))
	t.Cleanup(server.Close)
	return server, &store
}

func testCachedClient(t *testing.T) (*httptest.Server, *sync.Map, *CachedClient) {
	t.Helper()
	server, store := nacosServer(t)
	store.Store("content", "initial-config")
	client := NewClient(server.URL, "test-data", "DEFAULT_GROUP")
	return server, store, NewCachedClient(client)
}

// TestCachedGetAndRefresh 验证首次拉取缓存与强制刷新。
func TestCachedGetAndRefresh(t *testing.T) {
	_, store, cached := testCachedClient(t)

	content, err := cached.Get(context.Background())
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if content != "initial-config" {
		t.Fatalf("unexpected content: %q", content)
	}
	if !cached.Loaded() {
		t.Fatal("must be loaded after get")
	}

	// 服务端变更后 Refresh 生效
	store.Store("content", "updated-config")
	if err := cached.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if got := cached.Content(); got != "updated-config" {
		t.Fatalf("unexpected refreshed content: %q", got)
	}
	if cached.UpdatedAt().IsZero() {
		t.Fatal("updated at must be set")
	}
}

// TestCachedKeepsOldValueOnFailure 验证配置中心不可用时保留旧值。
func TestCachedKeepsOldValueOnFailure(t *testing.T) {
	server, store, cached := testCachedClient(t)

	if _, err := cached.Get(context.Background()); err != nil {
		t.Fatalf("get failed: %v", err)
	}

	// 服务端下线
	server.Close()
	_ = cached.Refresh(context.Background()) // 失败
	if got := cached.Content(); got != "initial-config" {
		t.Fatalf("old value must be kept on failure, got %q", got)
	}
	if _, err := cached.Get(context.Background()); err != nil {
		t.Fatalf("get must serve cache on failure: %v", err)
	}
	_ = store
}

// TestCachedSubscribeBroadcast 验证订阅立即收到当前值 + 变更广播。
func TestCachedSubscribeBroadcast(t *testing.T) {
	_, store, cached := testCachedClient(t)

	updates := cached.Subscribe()
	// 首次订阅立即收到当前值(已有缓存时)
	if _, err := cached.Get(context.Background()); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	select {
	case content := <-updates:
		if content != "initial-config" {
			t.Fatalf("unexpected initial subscription value: %q", content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial subscription value")
	}

	// 变更广播
	store.Store("content", "v2-config")
	if err := cached.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	select {
	case content := <-updates:
		if content != "v2-config" {
			t.Fatalf("unexpected update value: %q", content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for update broadcast")
	}

	cached.Close()
	select {
	case _, ok := <-updates:
		if ok {
			t.Fatal("channel must be closed after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

// TestCachedStartPeriodic 验证后台轮询刷新。
func TestCachedStartPeriodic(t *testing.T) {
	_, store, cached := testCachedClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = cached.Start(ctx, 50*time.Millisecond)
		close(done)
	}()
	t.Cleanup(func() { cancel(); <-done })

	// 等待首次刷新
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cached.Loaded() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !cached.Loaded() {
		t.Fatal("must load within periodic start")
	}

	// 变更后轮询生效
	store.Store("content", "periodic-config")
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cached.Content() == "periodic-config" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("periodic refresh did not pick up change, content=%q", cached.Content())
}

// TestCachedNilSafe 验证 nil 安全。
func TestCachedNilSafe(t *testing.T) {
	var cached *CachedClient
	if cached.Content() != "" {
		t.Fatal("nil content must be empty")
	}
	if cached.Loaded() {
		t.Fatal("nil must not be loaded")
	}
	if _, err := cached.Get(context.Background()); err == nil {
		t.Fatal("nil get must return error")
	}
	cached.Close() // 不 panic
	if fmt.Sprintf("%v", cached) == "" {
		t.Fatal("unreachable")
	}
}
