package live

import (
	"encoding/json"
	"net/http"

	apperr "github.com/Connorig/go-blackbox/component/error"
	zaplog "github.com/Connorig/go-blackbox/framework/log"
	"github.com/kataras/iris/v12"
)

// Handlers 业务注入的回调裁决函数。
// 返回 error 的(on_publish/on_play/on_connect)决定放行或拒绝;
// 其余为通知型,返回值忽略(仅日志)。
type Handlers struct {
	// OnPublish 推流鉴权(最关键):返回 nil 放行,返回 error 拒绝(403,SRS 断流)。
	OnPublish func(ctx iris.Context, info *PublishInfo) error
	// OnPlay 播放鉴权:返回 nil 放行,返回 error 拒绝。
	OnPlay func(ctx iris.Context, info *PlayInfo) error
	// OnConnect 连接鉴权(可选):返回 nil 放行,返回 error 拒绝;未注入默认放行。
	OnConnect func(ctx iris.Context, info *ConnectInfo) error
	// OnUnpublish 下播通知(无裁决)。
	OnUnpublish func(ctx iris.Context, info *UnpublishInfo)
	// OnDvr 录制完成通知。
	OnDvr func(ctx iris.Context, info *DvrInfo)
	// OnHls 切片生成通知。
	OnHls func(ctx iris.Context, info *HlsInfo)
}

// mountCallback 挂载回调路由(party 为回调挂载前缀,如 /api/live)。
func mountCallback(party iris.Party, handlers *Handlers) {
	if handlers == nil {
		handlers = &Handlers{}
	}
	// on_publish:推流鉴权
	party.Post("/on_publish", func(ctx iris.Context) {
		info, ok := parseBody[PublishInfo](ctx)
		if !ok {
			respondDeny(ctx, "invalid callback body")
			return
		}
		zaplog.WithComponent("live").Infow("live on_publish", "app", info.App, "stream", info.Stream, "ip", info.IP)
		if handlers.OnPublish == nil {
			respondOK(ctx) // 未注入:默认放行(对接期友好)
			return
		}
		if err := handlers.OnPublish(ctx, info); err != nil {
			zaplog.WithComponent("live").Infow("live on_publish denied", "app", info.App, "stream", info.Stream, "err", err.Error())
			respondDeny(ctx, err.Error())
			return
		}
		respondOK(ctx)
	})

	// on_play:播放鉴权
	party.Post("/on_play", func(ctx iris.Context) {
		info, ok := parseBody[PlayInfo](ctx)
		if !ok {
			respondDeny(ctx, "invalid callback body")
			return
		}
		zaplog.WithComponent("live").Infow("live on_play", "app", info.App, "stream", info.Stream, "ip", info.IP)
		if handlers.OnPlay == nil {
			respondOK(ctx)
			return
		}
		if err := handlers.OnPlay(ctx, info); err != nil {
			zaplog.WithComponent("live").Infow("live on_play denied", "stream", info.Stream, "err", err.Error())
			respondDeny(ctx, err.Error())
			return
		}
		respondOK(ctx)
	})

	// on_connect:连接鉴权(可选)
	party.Post("/on_connect", func(ctx iris.Context) {
		info, ok := parseBody[ConnectInfo](ctx)
		if !ok {
			respondOK(ctx) // 连接回调解析失败默认放行(不影响推流)
			return
		}
		if handlers.OnConnect == nil {
			respondOK(ctx)
			return
		}
		if err := handlers.OnConnect(ctx, info); err != nil {
			respondDeny(ctx, err.Error())
			return
		}
		respondOK(ctx)
	})

	// on_unpublish:下播通知
	party.Post("/on_unpublish", func(ctx iris.Context) {
		if info, ok := parseBody[UnpublishInfo](ctx); ok {
			zaplog.WithComponent("live").Infow("live on_unpublish", "app", info.App, "stream", info.Stream)
			if handlers.OnUnpublish != nil {
				handlers.OnUnpublish(ctx, info)
			}
		}
		ctx.StatusCode(http.StatusOK)
		_, _ = ctx.WriteString("0")
	})

	// on_dvr:录制完成
	party.Post("/on_dvr", func(ctx iris.Context) {
		if info, ok := parseBody[DvrInfo](ctx); ok {
			zaplog.WithComponent("live").Infow("live on_dvr", "app", info.App, "stream", info.Stream, "file", info.File)
			if handlers.OnDvr != nil {
				handlers.OnDvr(ctx, info)
			}
		}
		ctx.StatusCode(http.StatusOK)
		_, _ = ctx.WriteString("0")
	})

	// on_hls:切片生成
	party.Post("/on_hls", func(ctx iris.Context) {
		if info, ok := parseBody[HlsInfo](ctx); ok {
			if handlers.OnHls != nil {
				handlers.OnHls(ctx, info)
			}
		}
		ctx.StatusCode(http.StatusOK)
		_, _ = ctx.WriteString("0")
	})
}

// parseBody 容错解析回调 body(非 JSON 返回 false,不 panic)。
func parseBody[T any](ctx iris.Context) (*T, bool) {
	body, err := ctx.GetBody()
	if err != nil || len(body) == 0 {
		return nil, false
	}
	var info T
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, false
	}
	return &info, true
}

// respondOK 放行响应(SRS 要求 {"code":0})。
func respondOK(ctx iris.Context) {
	ctx.StatusCode(http.StatusOK)
	_, _ = ctx.WriteString(`{"code":0}`)
}

// respondDeny 拒绝响应(SRS 收到 code!=0 即断流/拒流)。
func respondDeny(ctx iris.Context, message string) {
	ctx.StatusCode(http.StatusForbidden)
	payload, _ := json.Marshal(map[string]interface{}{
		"code": 1,
		"msg":  message,
	})
	_, _ = ctx.Write(payload)
}

// 引用 component/error 保持统一错误风格(后续扩展用)。
var _ = apperr.CodeSystemError
