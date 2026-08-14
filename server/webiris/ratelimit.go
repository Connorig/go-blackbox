package webiris

import (
	"sync"

	"github.com/kataras/iris/v12"
	"golang.org/x/time/rate"
)

// Limit 返回令牌桶限流中间件。
// ratePerSecond 是每秒补充的令牌数；burst 是突发容量（非正数时取 ratePerSecond）。
// keyFunc 返回限流维度（如客户端 IP、用户 ID）；为 nil 时全局限流。
// 超限请求返回 429 统一响应并停止后续处理器。
// 用法：app.Use(webiris.Limit(100, 200, nil)) 或按 IP：app.Use(webiris.Limit(10, 20, func(ctx iris.Context) string { return ctx.RemoteAddr() }))
func Limit(ratePerSecond float64, burst int, keyFunc func(ctx iris.Context) string) iris.Handler {
	if ratePerSecond <= 0 {
		ratePerSecond = 10
	}
	if burst <= 0 {
		burst = int(ratePerSecond)
	}

	// keyFunc 为 nil 时使用单一全局限流器
	var global *rate.Limiter
	if keyFunc == nil {
		global = rate.NewLimiter(rate.Limit(ratePerSecond), burst)
	}
	// 按维度限流时缓存每个 key 的限流器；key 基数应保持有限（如 IP 维度）
	var perKey sync.Map

	return func(ctx iris.Context) {
		var limiter *rate.Limiter
		if global != nil {
			limiter = global
		} else {
			key := keyFunc(ctx)
			if key == "" {
				key = ctx.RemoteAddr()
			}
			value, ok := perKey.Load(key)
			if !ok {
				value, _ = perKey.LoadOrStore(key, rate.NewLimiter(rate.Limit(ratePerSecond), burst))
			}
			limiter = value.(*rate.Limiter)
		}

		if !limiter.Allow() {
			Fail(ctx, iris.StatusTooManyRequests, iris.StatusTooManyRequests, "too many requests")
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}
