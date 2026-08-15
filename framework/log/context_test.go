package zaplog

import (
	"context"
	"testing"
)

// TestWithRequestIDAndFromContext 验证 request_id 注入与日志字段联动。
func TestWithRequestIDAndFromContext(t *testing.T) {
	ctx := WithRequestID(context.Background(), "trace-42")
	if got := RequestIDFromContext(ctx); got != "trace-42" {
		t.Fatalf("unexpected request id: %q", got)
	}

	logger := FromContext(ctx)
	if logger == Logger {
		t.Fatal("FromContext must return a derived logger when request id is present")
	}
	// derived logger 应包含 request_id 字段（zap 内部不可直接读字段，
	// 通过 With 派生验证不 panic 且可写日志）
	logger.Info("request-scoped log")
}

// TestFromContextWithoutRequestID 验证无 request_id 时返回全局 Logger。
func TestFromContextWithoutRequestID(t *testing.T) {
	if got := FromContext(context.Background()); got != Logger {
		t.Fatal("FromContext without request id must return the global logger")
	}
	if got := FromContext(nil); got != Logger {
		t.Fatal("FromContext with nil context must return the global logger")
	}
}

// TestRequestIDFromContextNilSafe 验证 nil context 读取安全。
func TestRequestIDFromContextNilSafe(t *testing.T) {
	if got := RequestIDFromContext(nil); got != "" {
		t.Fatalf("nil context must return empty request id, got %q", got)
	}
}
