package openapi

import (
	"bytes"
	"io"
	"strconv"
	"sync"
	"time"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/kataras/iris/v12"
	"golang.org/x/time/rate"
)

// ContextKey 网关写入上下文的键。
const (
	contextKeyAppKey = "openapi_app_key"
	// DefaultRatePerSecond 应用默认限流速率。
	DefaultRatePerSecond = 100.0
)

// Config 开放网关配置。
type Config struct {
	// Registry 第三方应用注册表(必填)。
	Registry *Registry
	// NonceStore 防重放存储;nil 时使用内存版(单实例)。
	NonceStore NonceStore
	// TimestampWindow 时间戳允许偏差;非正数时默认 5 分钟。
	TimestampWindow time.Duration
	// NonceTTL nonce 保留时长;非正数时默认 10 分钟。
	NonceTTL time.Duration
	// OnAudit 审计钩子(每次开放接口调用都会回调,含失败请求)。
	// 业务可接异步日志队列或数据库;为 nil 时跳过。
	OnAudit func(ctx iris.Context, appKey string, ok bool, code apperr.Code)
}

// OpenAPI 开放接口注册器。
// 业务像注册普通路由一样注册 handler,签名校验由网关自动完成:
//
//	api := openapi.New(app, openapi.Config{Registry: registry})
//	api.GET("/v1/order/query", QueryOrder)  // 实际路径 /openapi/v1/order/query
//	api.POST("/v1/order/update", UpdateOrder)
type OpenAPI struct {
	party   iris.Party
	limiter sync.Map // appKey → *rate.Limiter
	cfg     Config
}

// New 创建开放接口注册器(自动挂载 /openapi 前缀 + 网关中间件)。
// app 为 iris 应用;Config.Registry 必填。
func New(app *iris.Application, cfg Config) *OpenAPI {
	registry := cfg.Registry
	if registry == nil {
		registry = NewRegistry()
	}
	if cfg.NonceStore == nil {
		cfg.NonceStore = NewMemNonceStore()
	}
	if cfg.TimestampWindow <= 0 {
		cfg.TimestampWindow = 5 * time.Minute
	}
	if cfg.NonceTTL <= 0 {
		cfg.NonceTTL = 10 * time.Minute
	}
	api := &OpenAPI{party: app.Party("/openapi"), cfg: cfg}
	api.party.Use(api.gateway)
	return api
}

// GET 注册开放 GET 接口(路径相对 /openapi)。
func (o *OpenAPI) GET(path string, handlers ...iris.Handler) {
	o.party.Get(path, handlers...)
}

// POST 注册开放 POST 接口。
func (o *OpenAPI) POST(path string, handlers ...iris.Handler) {
	o.party.Post(path, handlers...)
}

// PUT 注册开放 PUT 接口。
func (o *OpenAPI) PUT(path string, handlers ...iris.Handler) {
	o.party.Put(path, handlers...)
}

// DELETE 注册开放 DELETE 接口。
func (o *OpenAPI) DELETE(path string, handlers ...iris.Handler) {
	o.party.Delete(path, handlers...)
}

// Party 返回底层路由分组(高级用法,可继续嵌套)。
func (o *OpenAPI) Party(relative string, handlers ...iris.Handler) iris.Party {
	return o.party.Party(relative, handlers...)
}

// AppKey 从上下文读取调用方应用标识;非开放接口请求返回空串。
func AppKey(ctx iris.Context) string {
	return ctx.Values().GetString(contextKeyAppKey)
}

// gateway 开放网关中间件:签名校验 → 防重放 → 限流 → 放行。
func (o *OpenAPI) gateway(ctx iris.Context) {
	cfg := o.cfg
	appKey := ctx.GetHeader(HeaderAppKey)
	timestamp := ctx.GetHeader(HeaderTimestamp)
	nonce := ctx.GetHeader(HeaderNonce)
	signature := ctx.GetHeader(HeaderSignature)

	// ① 头完整性
	if appKey == "" || timestamp == "" || nonce == "" || signature == "" {
		o.reject(ctx, appKey, iris.StatusUnauthorized, apperr.CodeAccessUnauthorized, "missing signature headers")
		return
	}

	// ② 应用存在且启用
	app := cfg.Registry.Get(appKey)
	if app == nil || !app.Enabled {
		o.reject(ctx, appKey, iris.StatusUnauthorized, apperr.CodeAccessUnauthorized, "app not found or disabled")
		return
	}

	// ③ 时间戳窗口
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)) > cfg.TimestampWindow || time.Since(time.Unix(ts, 0)) < -cfg.TimestampWindow {
		o.reject(ctx, appKey, iris.StatusUnauthorized, apperr.CodeUserLoginExpired, "timestamp out of window")
		return
	}

	// ④ 读取并恢复请求体(不破坏后续 handler 读取)
	body, readErr := readBody(ctx)
	if readErr != nil {
		o.reject(ctx, appKey, iris.StatusBadRequest, apperr.CodeRequestParamError, "read body failed")
		return
	}

	// ⑤ 验签
	bodySHA256 := BodySHA256(body)
	if err := VerifySignature(app, ctx.Method(), ctx.Path(), timestamp, nonce, bodySHA256, signature); err != nil {
		o.reject(ctx, appKey, iris.StatusUnauthorized, apperr.CodeUserSignatureError, "signature verification failed")
		return
	}

	// ⑥ nonce 防重放
	used, err := cfg.NonceStore.TrySet(ctx.Request().Context(), "openapi:nonce:"+appKey+":"+nonce, cfg.NonceTTL)
	if err != nil {
		o.reject(ctx, appKey, iris.StatusInternalServerError, apperr.CodeSystemError, "nonce store unavailable")
		return
	}
	if !used {
		o.reject(ctx, appKey, iris.StatusBadRequest, apperr.CodeDuplicateRequest, "duplicate request (nonce replayed)")
		return
	}

	// ⑦ 每 App 限流
	limiter := o.limiterFor(app)
	if !limiter.Allow() {
		o.reject(ctx, appKey, iris.StatusTooManyRequests, apperr.CodeSystemRateLimited, "app rate limited")
		return
	}

	// ⑧ 放行
	ctx.Values().Set(contextKeyAppKey, appKey)
	if cfg.OnAudit != nil {
		cfg.OnAudit(ctx, appKey, true, apperr.CodeOK)
	}
	ctx.Next()
}

// reject 统一失败响应 + 审计回调。
func (o *OpenAPI) reject(ctx iris.Context, appKey string, status int, code apperr.Code, message string) {
	if o.cfg.OnAudit != nil {
		o.cfg.OnAudit(ctx, appKey, false, code)
	}
	ctx.StatusCode(status)
	_ = ctx.JSON(map[string]interface{}{
		"code":    code,
		"message": message,
	})
	ctx.StopExecution()
}

// limiterFor 获取应用限流器(按 AppKey 缓存)。
func (o *OpenAPI) limiterFor(app *App) *rate.Limiter {
	value, ok := o.limiter.Load(app.AppKey)
	if ok {
		return value.(*rate.Limiter)
	}
	rps := app.RatePerSecond
	if rps <= 0 {
		rps = DefaultRatePerSecond
	}
	burst := app.Burst
	if burst <= 0 {
		burst = int(rps)
	}
	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	actual, _ := o.limiter.LoadOrStore(app.AppKey, limiter)
	return actual.(*rate.Limiter)
}

// readBody 读取请求体并恢复,保证后续 handler 仍可正常读取。
func readBody(ctx iris.Context) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(ctx.Request().Body, 4<<20)) // 4MB 上限
	if err != nil {
		return nil, err
	}
	ctx.Request().Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
