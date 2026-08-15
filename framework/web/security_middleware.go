package webiris

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/Connorig/go-blackbox/component/security"
	"github.com/kataras/iris/v12"
)

// SQLGuard 返回 SQL 注入防护中间件:检测 query 参数与请求体中的注入特征,
// 命中返回 400 A0400 并停止处理(建议挂在限流之后、业务路由之前)。
// 注意:此中间件是前置拦截,不能替代参数化查询。
//
//	app.Use(webiris.SQLGuard())
func SQLGuard() iris.Handler {
	return func(ctx iris.Context) {
		// ① 检测 URL 查询参数
		for _, value := range ctx.URLParams() {
			if security.IsSQLInjection(value) {
				Fail(ctx, http.StatusBadRequest, apperr.CodeRequestParamError, "invalid parameter content")
				ctx.StopExecution()
				return
			}
		}
		// ② 检测请求体字符串(读取后恢复,不破坏后续 handler)
		body, err := readBodyBytes(ctx)
		if err != nil {
			Fail(ctx, http.StatusBadRequest, apperr.CodeRequestParamError, "read body failed")
			ctx.StopExecution()
			return
		}
		if len(body) > 0 && security.IsSQLInjection(string(body)) {
			Fail(ctx, http.StatusBadRequest, apperr.CodeRequestParamError, "invalid request body")
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}

// BodyLimit 返回请求体大小限制中间件(DoS 防护:超大请求体拦截)。
// maxBytes 为上限(字节),超限返回 413 A0400。
// 用法:app.Use(webiris.BodyLimit(1 << 20)) // 1MB
func BodyLimit(maxBytes int64) iris.Handler {
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 默认 1MB
	}
	return func(ctx iris.Context) {
		if ctx.Request().Body != nil && ctx.Request().ContentLength > maxBytes {
			Fail(ctx, http.StatusRequestEntityTooLarge, apperr.CodeRequestParamError, "request body too large")
			ctx.StopExecution()
			return
		}
		// ContentLength 可能为 -1(分块传输),读取时用 LimitReader 拦截
		ctx.Request().Body = http.MaxBytesReader(ctx.ResponseWriter(), ctx.Request().Body, maxBytes)
		ctx.Next()
	}
}

// Timeout 返回请求超时中间件(DoS 防护:慢速请求拦截)。
// 超时后返回 504 B0100;业务应配合 context 感知(查询/HTTP 调用带 ctx)。
// 用法:app.Use(webiris.Timeout(10 * time.Second))
func Timeout(timeout time.Duration) iris.Handler {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return func(ctx iris.Context) {
		ctx2, cancel := context.WithTimeout(ctx.Request().Context(), timeout)
		defer cancel()
		ctx.ResetRequest(ctx.Request().WithContext(ctx2))
		done := make(chan struct{})
		go func() {
			ctx.Next()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx2.Done():
			Fail(ctx, http.StatusGatewayTimeout, apperr.CodeSystemTimeout, "request timeout")
			ctx.StopExecution()
		}
	}
}

// readBodyBytes 读取请求体并恢复,保证后续 handler 可正常读取。
func readBodyBytes(ctx iris.Context) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(ctx.Request().Body, 4<<20)) // 4MB 上限
	if err != nil {
		return nil, err
	}
	ctx.Request().Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
