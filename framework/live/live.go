package live

import (
	"time"

	"github.com/Connorig/go-blackbox"
	"github.com/kataras/iris/v12"
)

// Config live 模块配置。
type Config struct {
	// APIBase SRS HTTP API 地址(如 http://127.0.0.1:1985)。
	APIBase string
	// CallbackMount 回调挂载前缀(需与 SRS http_hooks 配置一致,如 /api/live)。
	CallbackMount string
	// Handlers 业务注入的回调裁决函数。
	Handlers *Handlers
	// APITimeout SRS API 超时(默认 5s)。
	APITimeout int
}

// Provide 装配 live 模块(builder 回调中调用,业务一行启用):
//
//	builder.EnableWeb(appbox.TimeFormat, ":8080", "info", func(app *iris.Application) {
//	    live.Provide(app, live.Config{
//	        APIBase:       "http://127.0.0.1:1985",
//	        CallbackMount: "/api/live",
//	        Handlers: &live.Handlers{
//	            OnPublish: func(ctx iris.Context, info *live.PublishInfo) error {
//	                // 业务鉴权:密钥校验/封禁名单;返回 error 即拒流
//	                if info.Param != "?key=secret" {
//	                    return errors.New("invalid stream key")
//	                }
//	                return nil
//	            },
//	        },
//	    })
//	})
//
// 注册 *live.Client 全局便捷入口(SetGlobal);挂载 6 类回调路由。
func Provide(app *iris.Application, config Config) {
	if app == nil {
		return
	}
	if config.CallbackMount == "" {
		config.CallbackMount = "/api/live"
	}
	timeout := timeDurationSeconds(config.APITimeout)
	if config.APIBase != "" {
		client := NewClient(config.APIBase, timeout)
		SetGlobal(client)
	}
	party := app.Party(config.CallbackMount)
	mountCallback(party, config.Handlers)
}

// timeDurationSeconds 秒转 duration(默认 5s)。
func timeDurationSeconds(seconds int) time.Duration {
	if seconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// 引用 appbox 保持装配约定一致(Provide 由业务在 EnableWeb 回调调用)。
var _ = appbox.Version
