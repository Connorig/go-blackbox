package es

import (
	"context"
	"os"
	"testing"
)

// esAddr 返回测试 ES 地址;未配置时跳过。
func esAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("GO_BLACKBOX_ES_ADDR")
	if addr == "" {
		t.Skip("es not configured: set GO_BLACKBOX_ES_ADDR to run")
	}
	return addr
}

// testClient 连接测试 ES。
func testClient(t *testing.T) *Client {
	t.Helper()
	client, err := NewClient(Config{Addresses: []string{esAddr(t)}})
	if err != nil {
		t.Fatalf("new es client failed: %v", err)
	}
	return client
}

// TestNewClientInvalid 无效地址报错。
func TestNewClientInvalid(t *testing.T) {
	if _, err := NewClient(Config{Addresses: []string{"http://127.0.0.1:1"}}); err == nil {
		t.Fatal("unreachable address must fail")
	}
}

// TestIndexGetDelete 写入/获取/删除闭环(需真实 ES)。
func TestIndexGetDelete(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	index := "gbx-test-users"
	_, _ = client.CreateIndex(ctx, index, []byte(`{}`))
	t.Cleanup(func() { _ = client.Delete(ctx, index, "doc-1") })

	doc := map[string]interface{}{"name": "connor", "age": 30}
	if err := client.IndexWithRefresh(ctx, index, "doc-1", doc); err != nil {
		t.Fatalf("index failed: %v", err)
	}
	raw, err := client.Get(ctx, index, "doc-1")
	if err != nil || raw == nil {
		t.Fatalf("get failed: %v %s", err, raw)
	}
	// 不存在返回 nil
	missing, err := client.Get(ctx, index, "doc-missing")
	if err != nil || missing != nil {
		t.Fatalf("missing get: %v %s", err, missing)
	}
	if err := client.Delete(ctx, index, "doc-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

// TestSearch 查询(需真实 ES)。
func TestSearch(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	index := "gbx-test-search"
	_, _ = client.CreateIndex(ctx, index, []byte(`{}`))
	_ = client.IndexWithRefresh(ctx, index, "s-1", map[string]interface{}{"title": "hello world"})
	_ = client.IndexWithRefresh(ctx, index, "s-2", map[string]interface{}{"title": "hello gbx"})
	t.Cleanup(func() {
		_, _ = client.es.Indices.Delete([]string{index})
	})

	query := []byte(`{"query":{"match":{"title":"gbx"}}}`)
	result, err := client.Search(ctx, index, query)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if result.Hits.Total.Value < 1 {
		t.Fatalf("expected hits, total = %d", result.Hits.Total.Value)
	}
}

// TestExistsIndex 索引存在性(需真实 ES)。
func TestExistsIndex(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	index := "gbx-test-exists"
	_, _ = client.es.Indices.Delete([]string{index})
	exists, err := client.ExistsIndex(ctx, index)
	if err != nil || exists {
		t.Fatalf("exists = %v, %v", exists, err)
	}
	created, err := client.CreateIndex(ctx, index, []byte(`{}`))
	if err != nil || !created {
		t.Fatalf("create = %v, %v", created, err)
	}
	// 重复创建返回 false 不报错
	created, err = client.CreateIndex(ctx, index, []byte(`{}`))
	if err != nil || created {
		t.Fatalf("duplicate create = %v, %v", created, err)
	}
	exists, err = client.ExistsIndex(ctx, index)
	if err != nil || !exists {
		t.Fatalf("exists after create = %v, %v", exists, err)
	}
	_, _ = client.es.Indices.Delete([]string{index})
}
