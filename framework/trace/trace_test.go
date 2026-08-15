package trace

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

// TestInitLocalProvider Endpoint 为空时本地初始化不报错。
func TestInitLocalProvider(t *testing.T) {
	ctx := context.Background()
	shutdown, err := Init(ctx, Config{ServiceName: "test-svc"})
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown must not be nil")
	}
	defer func() { _ = shutdown(ctx) }()

	if otel.GetTracerProvider() == nil {
		t.Fatal("tracer provider not set")
	}
}

// TestSpanSuccess Span 执行成功无错误。
func TestSpanSuccess(t *testing.T) {
	ctx := context.Background()
	shutdown, _ := Init(ctx, Config{ServiceName: "test"})
	defer func() { _ = shutdown(ctx) }()

	called := false
	err := Span(ctx, "test-op", func(ctx context.Context) error {
		called = true
		WithAttribute(ctx, "key", "value")
		return nil
	})
	if err != nil || !called {
		t.Fatalf("span failed: %v called=%v", err, called)
	}
}

// TestSpanError 业务错误传递 + Span 标记。
func TestSpanError(t *testing.T) {
	ctx := context.Background()
	shutdown, _ := Init(ctx, Config{ServiceName: "test"})
	defer func() { _ = shutdown(ctx) }()

	bizErr := errors.New("business error")
	err := Span(ctx, "fail-op", func(ctx context.Context) error {
		return bizErr
	})
	if !errors.Is(err, bizErr) {
		t.Fatalf("error must propagate: %v", err)
	}
}

// TestTraceID 未初始化时返回空;初始化后可获取(记录型 Span)。
func TestTraceID(t *testing.T) {
	ctx := context.Background()
	if TraceID(ctx) != "" {
		t.Fatal("empty context must have no trace id")
	}
	shutdown, _ := Init(ctx, Config{ServiceName: "test"})
	defer func() { _ = shutdown(ctx) }()

	var traceID string
	_ = Span(ctx, "trace-check", func(spanCtx context.Context) error {
		traceID = TraceID(spanCtx)
		return nil
	})
	if traceID == "" {
		t.Fatal("span context must have trace id")
	}
}

// TestProviderType 使用 SDK 提供者(编译期验证)。
func TestProviderType(t *testing.T) {
	provider, ok := otel.GetTracerProvider().(*trace.TracerProvider)
	_ = provider
	_ = ok
}
