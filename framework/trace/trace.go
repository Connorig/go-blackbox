// Package trace 提供 OpenTelemetry 链路追踪集成(对标 Spring Cloud Sleuth):
// OTLP 导出 + 采样配置 + 便捷 Span 封装。接入后 HTTP 请求、日志自动携带 trace_id。
package trace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Config 链路追踪配置。
type Config struct {
	// ServiceName 服务名(上报到 Jaeger/OTel Collector 的 service.name)。
	ServiceName string
	// Endpoint OTLP HTTP 采集端点(如 http://127.0.0.1:4318);空时不启用导出。
	Endpoint string
	// Environment 环境(dev/test/prod,写入 resource 属性)。
	Environment string
	// SampleRatio 采样率 0-1(默认 1:全量;生产建议 0.1-0.3)。
	SampleRatio float64
	// Timeout 导出超时(默认 5s)。
	Timeout time.Duration
}

// normalize 补齐默认值。
func (c Config) normalize() Config {
	if c.ServiceName == "" {
		c.ServiceName = "go-blackbox-app"
	}
	if c.SampleRatio <= 0 || c.SampleRatio > 1 {
		c.SampleRatio = 1
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	return c
}

// Tracer 返回全局 Tracer(业务创建 Span 用)。
func Tracer() trace.Tracer {
	return otel.Tracer("go-blackbox")
}

// Init 初始化 OpenTelemetry:设置全局 TracerProvider + 传播器。
// 返回关闭函数(应用退出时调用,刷出未导出的 Span)。
// Endpoint 为空时仅初始化本地 Provider(不导出,Span 仍可编程使用)。
func Init(ctx context.Context, config Config) (func(context.Context) error, error) {
	cfg := config.normalize()

	options := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			attribute.String("deployment.environment", cfg.Environment),
		)),
	}

	if cfg.Endpoint != "" {
		exporter, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithInsecure(),
			otlptracehttp.WithTimeout(cfg.Timeout),
		)
		if err != nil {
			return nil, fmt.Errorf("trace: create otlp exporter: %w", err)
		}
		options = append(options, sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(2*time.Second)))
	}

	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider.Shutdown, nil
}

// Span 便捷封装:创建子 Span 并执行函数。
// fn 返回 error 时 Span 标记 error 并记录异常;返回 span 结束时间(供耗时统计)。
//
//	err := trace.Span(ctx, "create-order", func(ctx context.Context) error {
//	    // 业务逻辑(内部再嵌套 Span 自动形成父子关系)
//	    return createOrder(ctx)
//	})
func Span(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	spanCtx, span := Tracer().Start(ctx, name)
	defer span.End()
	if fn == nil {
		return nil
	}
	if err := fn(spanCtx); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("error", true))
		return err
	}
	return nil
}

// WithAttribute 向当前 Span 追加属性(函数内多次调用)。
func WithAttribute(ctx context.Context, key string, value interface{}) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(attr(key, value))
}

// TraceID 从 Context 读取当前 Span 的 TraceID(日志关联用)。
func TraceID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

// SpanID 当前 Span ID。
func SpanID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.SpanID().String()
}

// attr 构造属性(支持常见类型)。
func attr(key string, value interface{}) attribute.KeyValue {
	switch typed := value.(type) {
	case string:
		return attribute.String(key, typed)
	case int:
		return attribute.Int(key, typed)
	case int64:
		return attribute.Int64(key, typed)
	case float64:
		return attribute.Float64(key, typed)
	case bool:
		return attribute.Bool(key, typed)
	default:
		return attribute.String(key, fmt.Sprintf("%v", typed))
	}
}

// ErrNotInitialized 未调用 Init 时使用全局默认 Provider 的提示错误。
var ErrNotInitialized = errors.New("trace: call trace.Init first")
