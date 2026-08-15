package monitor

import (
	"encoding/json"
	"net/http"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/kataras/iris/v12"
	"golang.org/x/time/rate"
)

// Config 监控组件配置。
type Config struct {
	// Auth 监控 API 的身份校验中间件(推荐 webiris.Auth,白名单放行页面)。
	// 为 nil 时不校验(仅限内网/Admin 端口场景)。
	Auth iris.Handler
	// RatePerSecond 监控 API 每 IP 限流速率;非正数时默认 5 QPS(防接口轰炸)。
	RatePerSecond float64
	// Burst 突发容量;非正数时取 RatePerSecond。
	Burst int
}

// Register 注册监控路由:
//
//	GET  <prefix>          监控页面(HTML,匿名可看,建议内网)
//	GET  <prefix>/api/stats 监控数据 JSON(挂 Auth + 限流)
//
// prefix 建议 /monitor。返回注册后的 party。
//
//	monitor.Register(app, "/monitor", monitor.Config{
//	    Auth: webiris.Auth(webiris.AuthConfig{Whitelist: []string{"/monitor"}}),
//	})
func Register(app *iris.Application, prefix string, config Config) iris.Party {
	if app == nil {
		panic("monitor: app is nil")
	}
	if prefix == "" {
		prefix = "/monitor"
	}
	collector := NewCollector()
	party := app.Party(prefix)

	// 监控页面(匿名;部署建议:内网或 Admin 端口,或由网关控制访问)
	party.Get("/", func(ctx iris.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Header("Cache-Control", "no-store")
		_, _ = ctx.WriteString(monitorPageHTML)
	})
	// 兼容无斜杠访问
	party.Get("", func(ctx iris.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Header("Cache-Control", "no-store")
		_, _ = ctx.WriteString(monitorPageHTML)
	})

	// 数据 API:限流(必选)+ 身份校验(可选)
	rps := config.RatePerSecond
	if rps <= 0 {
		rps = 5
	}
	burst := config.Burst
	if burst <= 0 {
		burst = int(rps)
	}
	limiters := rate.NewLimiter(rate.Limit(rps), burst)
	handlers := []iris.Handler{func(ctx iris.Context) {
		if !limiters.Allow() {
			Fail(ctx, http.StatusTooManyRequests, apperr.CodeSystemRateLimited, "monitor api rate limited")
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}}
	if config.Auth != nil {
		handlers = append(handlers, config.Auth)
	}
	party.Get("/api/stats", append(handlers, func(ctx iris.Context) {
		stats, err := collector.Stats()
		if err != nil {
			// 部分平台数据缺失时仍返回已采集部分
			ctx.StatusCode(http.StatusOK)
		}
		_ = json.NewEncoder(ctx.ResponseWriter()).Encode(stats)
	})...)

	return party
}

// Fail 输出统一失败响应(与 webiris 一致的错误格式)。
func Fail(ctx iris.Context, status int, code apperr.Code, message string) {
	ctx.StatusCode(status)
	_ = ctx.JSON(map[string]interface{}{"code": code, "message": message})
}
