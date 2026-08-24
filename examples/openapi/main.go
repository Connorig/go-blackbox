// examples/openapi 展示 go-blackbox v1.10 的第三方对接与开放 API 能力:
//   - 入站:第三方通过 AppKey + HMAC 签名调用我方开放接口(/openapi/*),
//     签名校验、时间戳窗口、nonce 防重放、每 App 限流全部由脚手架完成,
//     业务 handler 与普通路由完全一致,不接触任何加密细节。
//   - 出站:我方通过 thirdparty.Client 调用第三方接口,自动签名。
//
// 运行:go run ./examples/openapi
// 业务端口 :9530
//
// 验证命令(见 README.md):
//
//	PowerShell 运行 scripts/sign.ps1 生成签名请求头,再用 curl 调用。
package main

import (
	"context"
	"log"

	appbox "github.com/Connorig/go-blackbox"
	"github.com/Connorig/go-blackbox/framework/openapi"
	"github.com/Connorig/go-blackbox/framework/web"
	"github.com/kataras/iris/v12"
)

// openAPIRegistry 模拟第三方应用注册表。
// 生产环境可从数据库/配置中心加载,通过 registry.Set 热更新(密钥轮换立即生效)。
func openAPIRegistry() *openapi.Registry {
	return openapi.NewRegistryWith(
		&openapi.App{
			AppKey:    "company-001",      // 第三方公司标识
			AppSecret: "change-me-secret", // HMAC 对称密钥(生产走配置/密钥管理)
			Algorithm: openapi.AlgHMAC,
			Enabled:   true,
		},
	)
}

// QueryOrder 开放接口 handler——与普通路由 handler 完全一样,纯业务。
// 通过 openapi.AppKey(ctx) 可读取调用方应用标识(审计/权限用)。
func QueryOrder(ctx iris.Context) {
	orderID := ctx.URLParam("order_id")
	if orderID == "" {
		webiris.Fail(ctx, 400, "A0400", "order_id is required")
		return
	}
	webiris.OK(ctx, map[string]interface{}{
		"order_id": orderID,
		"status":   "paid",
		"app":      openapi.AppKey(ctx),
	})
}

// UpdateOrder 开放写接口:接收 JSON 请求体,签名已含请求体摘要。
func UpdateOrder(ctx iris.Context) {
	var request struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
	}
	if err := ctx.ReadJSON(&request); err != nil {
		webiris.Fail(ctx, 400, "A0427", "invalid json body")
		return
	}
	webiris.OK(ctx, map[string]interface{}{
		"order_id": request.OrderID,
		"status":   request.Status,
		"updated":  true,
	})
}

func main() {
	err := appbox.New().Start(func(ctx context.Context, builder *appbox.ApplicationBuild) error {
		builder.InitLog(".", "info")

		// Web 服务:注册开放接口——像注册普通路由一样,加密由脚手架接管
		builder.EnableWeb(appbox.TimeFormat, ":9530", "info", func(app *iris.Application) {
			app.Use(webiris.RequestID, webiris.AccessLog)

			api := openapi.New(app, openapi.Config{
				Registry: openAPIRegistry(),
				// 生产建议:
				//   NonceStore: openapi.NewRedisNonceStore(redis SETNX),多实例共享防重放
				//   OnAudit:    记录每次开放调用(zaplog 或异步日志队列)
			})
			api.GET("/v1/order/query", QueryOrder)    // 实际路径 /openapi/v1/order/query
			api.POST("/v1/order/update", UpdateOrder) // 实际路径 /openapi/v1/order/update

			// 内部健康探针(不经过开放网关)
			app.Get("/health", func(ctx iris.Context) {
				webiris.OK(ctx, map[string]string{"status": "up"})
			})
		})

		log.Println("openapi example listening on :9530 (openapi prefix /openapi)")
		return nil
	})
	if err != nil {
		log.Fatalf("start application failed: %v", err)
	}
}
