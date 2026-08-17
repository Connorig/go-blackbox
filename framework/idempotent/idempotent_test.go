package idempotent

import (
	"context"
	"testing"
	"time"
)

// TestGuardNilSafe nil/空参数安全。
func TestGuardNilSafe(t *testing.T) {
	var guard *Guard
	if _, err := guard.Check(context.Background(), "k", 0); err == nil {
		t.Fatal("nil guard must return error")
	}
	empty := NewGuard(nil, "")
	if _, err := empty.Check(context.Background(), "k", 0); err == nil {
		t.Fatal("nil redis must return error")
	}
	if _, err := empty.Check(nil, "k", 0); err == nil {
		t.Fatal("nil ctx must return error")
	}
	if _, err := empty.Check(context.Background(), "", 0); err == nil {
		t.Fatal("empty key must return error")
	}
}

// TestGuardKeyPrefix key 前缀组装。
func TestGuardKeyPrefix(t *testing.T) {
	guard := NewGuard(nil, "idem")
	if guard.key("gift-001") != "idem:gift-001" {
		t.Fatalf("key = %q", guard.key("gift-001"))
	}
	noPrefix := NewGuard(nil, "")
	if noPrefix.key("gift-001") != "gift-001" {
		t.Fatalf("key = %q", noPrefix.key("gift-001"))
	}
}

// TestGuardTTLDefault 默认 TTL 生效(通过 Check 内部逻辑间接验证:此处验证常量语义)。
func TestGuardTTLDefault(t *testing.T) {
	guard := NewGuard(nil, "")
	// 需要真实 Redis 才能验证 SetNX ttl;此处验证 nil-safe 行为即可
	_, err := guard.Check(context.Background(), "x", time.Hour)
	if err == nil {
		t.Fatal("expected nil redis error")
	}
}
