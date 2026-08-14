package mongodb

import (
	"strings"
	"testing"
)

// TestGetApplyURIWithoutCredentials 验证无凭据时生成简洁连接串。
func TestGetApplyURIWithoutCredentials(t *testing.T) {
	config := &MongoDBConfig{Addr: "127.0.0.1:27017", DB: "admin"}
	uri := config.GetApplyURI()
	if uri != "mongodb://127.0.0.1:27017?connect=direct" {
		t.Fatalf("unexpected URI without credentials: %q", uri)
	}
}

// TestGetApplyURIWithCredentials 验证带凭据时生成含编码凭据的连接串。
func TestGetApplyURIWithCredentials(t *testing.T) {
	config := &MongoDBConfig{
		Addr:     "10.0.0.1:27017",
		DB:       "admin",
		User:     "admin",
		Password: "p@ss:word",
	}
	uri := config.GetApplyURI()
	if !strings.HasPrefix(uri, "mongodb://admin:p%40ss%3Aword@10.0.0.1:27017?") {
		t.Fatalf("unexpected URI with credentials: %q", uri)
	}
	if strings.Contains(uri, "p@ss:word") {
		t.Fatalf("credentials must be URL-encoded in URI: %q", uri)
	}
}

// TestGetApplyURINilConfig 验证 nil 配置返回空串而不是 panic。
func TestGetApplyURINilConfig(t *testing.T) {
	var config *MongoDBConfig
	if uri := config.GetApplyURI(); uri != "" {
		t.Fatalf("nil config must return empty URI, got %q", uri)
	}
}
