package ratelimit

import (
	"context"
	"testing"
)

// TestLimiterNilSafe nil/空参数安全。
func TestLimiterNilSafe(t *testing.T) {
	var limiter *Limiter
	if _, err := limiter.Allow(context.Background(), "k", 1, 1); err == nil {
		t.Fatal("nil limiter must return error")
	}
	empty := NewLimiter(nil, "")
	if _, err := empty.Allow(context.Background(), "k", 1, 1); err == nil {
		t.Fatal("nil redis must return error")
	}
	if _, err := empty.Allow(nil, "k", 1, 1); err == nil {
		t.Fatal("nil ctx must return error")
	}
	if _, err := empty.Allow(context.Background(), "", 1, 1); err == nil {
		t.Fatal("empty key must return error")
	}
	if _, err := empty.Allow(context.Background(), "k", 0, 1); err == nil {
		t.Fatal("invalid rate must return error")
	}
}

// TestLimiterKeyPrefix key 前缀。
func TestLimiterKeyPrefix(t *testing.T) {
	limiter := NewLimiter(nil, "rl")
	// 通过 Reset 错误路径间接验证 prefix 逻辑无法直测;此处只验证构造
	if limiter.prefix != "rl" {
		t.Fatalf("prefix = %q", limiter.prefix)
	}
}
