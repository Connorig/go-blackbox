// examples/web-basic 展示 go-blackbox v1.17 全家桶能力的最小完整应用:
//   - 安全基线:限流/请求体上限/超时/SQL 注入拦截/安全头/统一错误
//   - JWT 认证 + 组织身份:登录签发 token(scope + orgID/deptID),数据权限隔离
//   - 开放平台:第三方通过 AppKey + HMAC 签名调用 /openapi/*,业务零加密负担
//   - 出站调用 + 熔断:签名客户端调用第三方,熔断器保护
//   - 监控 + 告警:资源监控页 /monitor,水位超阈值推送 webhook
//   - Admin:pprof + metrics + 运行时日志级别
//
// 运行:go run ./examples/web-basic
// 业务端口 :9528,Admin 端口 :6060
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	appbox "github.com/Connorig/go-blackbox"
	"github.com/Connorig/go-blackbox/component/auth/token"
	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/Connorig/go-blackbox/framework/alert"
	"github.com/Connorig/go-blackbox/framework/circuit"
	"github.com/Connorig/go-blackbox/framework/database"
	"github.com/Connorig/go-blackbox/framework/monitor"
	"github.com/Connorig/go-blackbox/framework/openapi"
	"github.com/Connorig/go-blackbox/framework/thirdparty"
	"github.com/Connorig/go-blackbox/framework/web"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
)

// User 是示例业务模型(数据权限字段:org_id/dept_id)。
type User struct {
	ID     int64  `gorm:"primarykey"`
	Name   string `gorm:"size:64"`
	OrgID  int64  `gorm:"column:org_id;index"`
	DeptID int64  `gorm:"column:dept_id;index"`
}

func main() {
	err := appbox.New().Start(func(ctx context.Context, builder *appbox.ApplicationBuild) error {
		// 1. 日志
		builder.InitLog(".", "info")

		// 2. SQLite 数据库(零外部依赖)
		builder.EnableDatabase(&datasource.Config{
			Driver:      datasource.DriverSQLite,
			DSN:         "./web-basic.db",
			AutoMigrate: false,
		})

		// 3. 版本化迁移(数据库初始化后、Web 启动前执行)
		builder.BeforeSetup(func(ctx context.Context) error {
			dbInstance, err := datasource.Get()
			if err != nil {
				return fmt.Errorf("get default database: %w", err)
			}
			migrator := datasource.NewMigrator(dbInstance,
				datasource.Migration{
					Name: "20260815_create_users",
					Up: func(db *gorm.DB) error {
						if err := db.AutoMigrate(&User{}); err != nil {
							return err
						}
						// 种子数据:两个组织,演示数据权限隔离
						return db.Create(&[]User{
							{Name: "org1-a", OrgID: 101, DeptID: 10101},
							{Name: "org1-b", OrgID: 101, DeptID: 10102},
							{Name: "org2-a", OrgID: 202, DeptID: 20201},
						}).Error
					},
					Down: func(db *gorm.DB) error {
						return db.Migrator().DropTable(&User{})
					},
				},
			)
			return migrator.Migrate(ctx)
		})

		// 4. JWT 密钥(生产环境从配置/密钥管理系统注入)
		if err := apptoken.SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
			return err
		}

		// 5. Web 服务
		builder.EnableWeb(appbox.TimeFormat, ":9528", "info", func(app *iris.Application) {
			// 5.1 安全基线:统一错误/链路/日志/安全头 + DoS 防护 + SQL 注入拦截
			app.Use(webiris.ErrorHandler, webiris.RequestID, webiris.AccessLog, webiris.SecurityHeaders)
			app.Use(webiris.Limit(100, 200, nil))
			app.Use(webiris.BodyLimit(1 << 20))
			app.Use(webiris.Timeout(10 * time.Second))
			app.Use(webiris.SQLGuard())

			// 5.2 健康探针
			webiris.RegisterHealth(app, func() error {
				return datasource.Health(ctx)
			})

			// 5.3 登录:签发携带 scope + 组织身份的 token
			app.Post("/api/v1/login", func(ctx iris.Context) {
				// 演示:固定账号 org1(组织 101/部门 10101);生产从用户表读取
				access, refresh, err := apptoken.GenTokenFull(42, "demo@example.com", "user:read", 101, 10101)
				if err != nil {
					webiris.Fail(ctx, 500, apperr.CodeSystemError, "generate token failed")
					return
				}
				webiris.OK(ctx, map[string]string{"access_token": access, "refresh_token": refresh})
			})

			// 5.4 认证中间件(健康/登录/监控页放行)
			app.Use(webiris.Auth(webiris.AuthConfig{
				Whitelist: []string{"/health", "/api/v1/login", "/monitor"},
				Scope:     "user:read",
			}))

			// 5.5 业务接口:当前用户 + 组织数据隔离列表
			app.Get("/api/v1/me", func(ctx iris.Context) {
				webiris.OK(ctx, map[string]interface{}{
					"id":    webiris.UserID(ctx),
					"email": webiris.UserEmail(ctx),
					"scope": webiris.DataScope(ctx),
				})
			})
			app.Get("/api/v1/users", func(ctx iris.Context) {
				scope := webiris.DataScope(ctx) // 组织 101/部门 10101
				dbInstance, err := datasource.Get()
				if err != nil {
					webiris.RespondError(ctx, err)
					return
				}
				var users []User
				query := dbInstance.DB().WithContext(ctx.Request().Context())
				if err := query.Scopes(scope.Condition()).Find(&users).Error; err != nil {
					webiris.RespondError(ctx, err)
					return
				}
				webiris.OK(ctx, users) // 只返回本组织数据(2 条,不含 org2)
			})

			// 5.6 开放平台:第三方签名调用(签名/防重放/限流由脚手架完成)
			openAPIRegistry := openapi.NewRegistryWith(&openapi.App{
				AppKey:    "partner-001",
				AppSecret: "change-me-secret",
				Algorithm: openapi.AlgHMAC,
				Enabled:   true,
			})
			api := openapi.New(app, openapi.Config{Registry: openAPIRegistry})
			api.GET("/v1/order/query", func(ctx iris.Context) {
				webiris.OK(ctx, map[string]interface{}{
					"order_id": ctx.URLParam("order_id"),
					"status":   "paid",
					"app":      openapi.AppKey(ctx),
				})
			})
			api.POST("/v1/order/update", func(ctx iris.Context) {
				var request struct {
					OrderID string `json:"order_id"`
					Status  string `json:"status"`
				}
				if err := ctx.ReadJSON(&request); err != nil {
					webiris.Fail(ctx, 400, apperr.CodeJSONParseFailed, "invalid json body")
					return
				}
				webiris.OK(ctx, map[string]interface{}{"order_id": request.OrderID, "status": request.Status, "updated": true})
			})

			// 5.7 出站调用示例:签名客户端 + 熔断保护
			// (演示端点:调用 /api/v1/partner 会请求本服务自己的 /monitor/api/stats 并返回)
			partnerClient := thirdparty.NewClient(thirdparty.Config{
				BaseURL:    "http://127.0.0.1:9528",
				Signer:     thirdparty.NewBearerSigner("demo-partner-token"),
				Timeout:    3 * time.Second,
				MaxRetries: 1,
				Breaker: circuit.New(circuit.Config{
					FailureThreshold: 0.5,
					MinRequests:      5,
					Window:           10 * time.Second,
					Cooldown:         10 * time.Second,
				}),
			})
			app.Get("/api/v1/partner", func(ctx iris.Context) {
				var stats struct {
					Hostname string `json:"hostname"`
				}
				if err := partnerClient.Get(ctx.Request().Context(), "/monitor/api/stats", nil, &stats); err != nil {
					webiris.RespondError(ctx, err)
					return
				}
				webiris.OK(ctx, map[string]string{"partner_host": stats.Hostname, "client": "thirdparty+breaker"})
			})

			// 5.8 资源监控页面 + 数据接口(接口内置限流;生产建议内网/Admin)
			monitor.Register(app, "/monitor", monitor.Config{})
		})

		// 6. Admin 管理服务
		builder.EnableAdmin()
		builder.EnableAdminRoutes(func(app *iris.Application) {
			app.Get("/ops/demo", func(ctx iris.Context) {
				webiris.OK(ctx, map[string]string{"status": "operational"})
			})
		})

		// 7. 监控告警:水位超阈值推送 webhook(示例用 OnNotify 打日志,生产配机器人地址)
		builder.AfterSetup(func(ctx context.Context) error {
			watcher := alert.NewWatcher(alert.Config{
				Interval:  15 * time.Second,
				Collector: monitor.NewCollector(),
				Notifiers: []alert.Notifier{
					// alert.NewWeComWebhook("https://qyapi.weixin.qq.com/..."),
				},
				Rules: []alert.Rule{
					alert.CPUUsage(90, 3),
					alert.MemoryUsage(85, 3),
					alert.DiskUsage(85, 3),
				},
				OnNotify: func(message alert.Message) {
					log.Printf("[ALERT] %s: %s", message.Level, message.Title)
				},
			})
			go watcher.Start(ctx)
			return nil
		})

		return nil
	})
	if err != nil {
		log.Fatalf("start application failed: %v", err)
	}
}
