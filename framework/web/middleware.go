package webiris

import (
	"crypto/rand"
	"encoding/hex"
	"runtime/debug"
	"time"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/Connorig/go-blackbox/component/i18n"
	zaplog "github.com/Connorig/go-blackbox/framework/log"
	"github.com/kataras/iris/v12"
)

// RequestIDHeader 是 Request ID 透传/写入的响应头名称。
const RequestIDHeader = "X-Request-ID"

// RequestID 中间件为每个请求生成或透传 Request ID，并写入响应头与上下文。
// 透传来源：请求头 X-Request-ID；业务代码可通过 ctx.Values().GetString("request_id") 读取。
// 用法：app.Use(RequestID)
func RequestID(ctx iris.Context) {
	requestID := ctx.GetHeader(RequestIDHeader)
	if requestID == "" {
		requestID = newRequestID()
	}
	ctx.Header(RequestIDHeader, requestID)
	ctx.Values().Set("request_id", requestID)
	// 注入标准 context，供 zaplog.FromContext 读取并附加 request_id 字段。
	req := ctx.Request().WithContext(zaplog.WithRequestID(ctx.Request().Context(), requestID))
	ctx.ResetRequest(req)
	ctx.Next()
}

// newRequestID 生成 16 字节随机 hex 标识；随机源失败时回退固定值。
func newRequestID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buffer)
}

// AccessLog 中间件记录方法、路径、状态码、耗时、Request ID 与客户端地址。
// 用法：app.Use(AccessLog)
func AccessLog(ctx iris.Context) {
	start := time.Now()
	ctx.Next()
	zaplog.WithComponent("web").Infow("http request",
		"method", ctx.Method(),
		"path", ctx.Path(),
		"status", ctx.GetStatusCode(),
		"duration", time.Since(start).String(),
		"request_id", ctx.Values().GetString("request_id"),
		"client_ip", ctx.RemoteAddr(),
	)
}

// CORS 中间件添加跨域响应头。
// 不传 allowedOrigins 时允许所有来源；传入时仅放行匹配来源；OPTIONS 预检直接返回 204。
// 用法：app.Use(CORS()) 或 app.Use(CORS("https://trusted.example.com"))
func CORS(allowedOrigins ...string) iris.Handler {
	allowAll := len(allowedOrigins) == 0
	return func(ctx iris.Context) {
		origin := ctx.GetHeader("Origin")
		if allowAll {
			ctx.Header("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					ctx.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		ctx.Header("Access-Control-Max-Age", "86400")
		if ctx.Method() == iris.MethodOptions {
			ctx.StatusCode(iris.StatusNoContent)
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}

// SecurityHeaders 中间件添加基础安全响应头。
// 用法：app.Use(SecurityHeaders)
func SecurityHeaders(ctx iris.Context) {
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("X-Frame-Options", "DENY")
	ctx.Header("Referrer-Policy", "no-referrer")
	ctx.Next()
}

// RegisterHealth 注册存活与就绪探针端点。
// /health/live 始终返回 ok；/health/ready 在 ready 回调返回错误时响应 503。
func RegisterHealth(app *iris.Application, ready func() error) {
	if app == nil {
		return
	}
	app.Get("/health/live", func(ctx iris.Context) {
		if _, err := ctx.WriteString("ok"); err != nil {
			app.Logger().Errorf("write live probe response failed: %v", err)
		}
	})
	app.Get("/health/ready", func(ctx iris.Context) {
		if ready != nil {
			if err := ready(); err != nil {
				ctx.StatusCode(iris.StatusServiceUnavailable)
				if _, writeErr := ctx.WriteString(err.Error()); writeErr != nil {
					app.Logger().Errorf("write ready probe response failed: %v", writeErr)
				}
				return
			}
		}
		if _, err := ctx.WriteString("ready"); err != nil {
			app.Logger().Errorf("write ready probe response failed: %v", err)
		}
	})
}

// PanicRecovery 兜底中间件:业务 handler panic 时返回统一 500 响应并记录
// 错误与堆栈,避免连接悬挂。建议放在中间件链最外层(app.Use 第一个注册)。
// 响应已开始写入(iris IsRecovered)时不再覆盖,仅记录日志。
func PanicRecovery() iris.Handler {
	return func(ctx iris.Context) {
		defer func() {
			if r := recover(); r != nil {
				zaplog.SugaredLogger.Errorw("http handler panic",
					"request_id", ctx.Values().GetString("request_id"),
					"path", ctx.Path(),
					"panic", r,
					"stack", string(debug.Stack()),
				)
				if _, recovered := ctx.IsRecovered(); recovered {
					return
				}
				Fail(ctx, 500, apperr.CodeSystemError, "internal server error")
			}
		}()
		ctx.Next()
	}
}

// Language 中间件:按 Accept-Language 检测请求语言并写入上下文
// (ctx.Values "lang";读取用 webiris.Lang(ctx))。
// bundle 为 i18n 资源包,负责语言识别与回退(nil 时恒为默认语言)。
// 用法:
//
//	app.Use(webiris.Language(i18nBundle))
//	message := bundle.T(webiris.Lang(ctx), "order.created", params)
func Language(bundle *i18n.Bundle) iris.Handler {
	return func(ctx iris.Context) {
		lang := i18n.DefaultLang
		if bundle != nil {
			lang = bundle.DetectLanguage(ctx.GetHeader("Accept-Language"))
		}
		ctx.Values().Set("lang", lang)
		ctx.Next()
	}
}

// Lang 返回当前请求语言(webiris.Language 中间件设置;未设置返回默认 zh-CN)。
func Lang(ctx iris.Context) string {
	if ctx == nil {
		return i18n.DefaultLang
	}
	if lang, ok := ctx.Values().Get("lang").(string); ok && lang != "" {
		return lang
	}
	return i18n.DefaultLang
}

