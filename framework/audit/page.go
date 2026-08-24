package oplog

import (
	"net/http"
	"strings"

	apperr "github.com/Connorig/go-blackbox/component/error"
	web "github.com/Connorig/go-blackbox/framework/web"
	"github.com/kataras/iris/v12"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// Config 审计查询页配置。
type Config struct {
	// Auth 数据接口的身份校验中间件(推荐 webiris.Auth,白名单放行页面)。
	// 为 nil 时不校验(仅限内网/Admin 端口场景)。
	Auth iris.Handler
	// RatePerSecond 数据接口每 IP 限流速率;非正数时默认 5 QPS。
	RatePerSecond float64
	// Burst 突发容量;非正数时取 RatePerSecond。
	Burst int
}

// Register 注册审计查询页与数据接口:
//
//	GET  <prefix>           审计查询页(HTML,自包含,10s 自动刷新)
//	GET  <prefix>/api/audit 审计数据 JSON(挂 Auth + 限流)
//
// prefix 建议 /ops/audit。返回注册后的 party。
//
//	oplog.Register(app, "/ops/audit", client, "ops:audit", oplog.Config{
//	    Auth: webiris.Auth(webiris.AuthConfig{Whitelist: []string{"/ops/audit"}}),
//	})
func Register(app *iris.Application, prefix string, client *redis.Client, key string, config Config) iris.Party {
	if app == nil {
		panic("oplog: app is nil")
	}
	if prefix == "" {
		prefix = "/ops/audit"
	}
	pageHTML := renderAuditPage(prefix)
	party := app.Party(prefix)

	// 页面(匿名;部署建议:内网或 Admin 端口,或由网关控制访问)
	party.Get("/", func(ctx iris.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Header("Cache-Control", "no-store")
		_, _ = ctx.WriteString(pageHTML)
	})
	// 兼容无斜杠访问
	party.Get("", func(ctx iris.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Header("Cache-Control", "no-store")
		_, _ = ctx.WriteString(pageHTML)
	})

	// 数据接口:限流(必选)+ 身份校验(可选)
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
			web.Fail(ctx, http.StatusTooManyRequests, apperr.CodeSystemRateLimited, "audit api rate limited")
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}}
	if config.Auth != nil {
		handlers = append(handlers, config.Auth)
	}
	handlers = append(handlers, QueryHandler(client, key))
	party.Get("/api/audit", handlers...)
	return party
}

// renderAuditPage 注入 prefix 到页面脚本。
func renderAuditPage(prefix string) string {
	return strings.ReplaceAll(auditPageHTML, "{{PREFIX}}", prefix)
}
