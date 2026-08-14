package zaplog

import (
	"context"

	"go.uber.org/zap"
)

// requestIDContextKey 是 request_id 在 context 中的存储键。
type requestIDContextKey struct{}

// WithRequestID 把 request_id 注入 context，供 FromContext 读取。
// 典型用法：Web 中间件在生成/透传 Request ID 后调用。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// FromContext 返回附加 request_id 字段的结构化 Logger。
// context 中不存在 request_id 时返回全局 Logger，保证日志调用方无需判空。
func FromContext(ctx context.Context) *zap.Logger {
	requestID := RequestIDFromContext(ctx)
	if requestID == "" {
		return Logger
	}
	return Logger.With(zap.String("request_id", requestID))
}

// RequestIDFromContext 从 context 读取 request_id；不存在时返回空字符串。
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, ok := ctx.Value(requestIDContextKey{}).(string)
	if !ok {
		return ""
	}
	return value
}
