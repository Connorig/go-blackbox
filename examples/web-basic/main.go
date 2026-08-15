// examples/web-basic 展示 go-blackbox v1.4 的核心能力：
// Web + 中间件体系 + JWT 认证(scope) + 健康探针 + 统一响应 +
// Admin 管理服务(pprof/metrics/日志级别) + SQLite 数据库 + 版本化迁移 + 优雅关闭。
//
// 运行:go run ./examples/web-basic
// 业务端口 :9528,Admin 端口 :6060
package main

import (
	"context"
	"fmt"
	"log"

	appbox "github.com/Connorig/go-blackbox"
	"github.com/Connorig/go-blackbox/component/auth/token"
	"github.com/Connorig/go-blackbox/framework/database"
	"github.com/Connorig/go-blackbox/framework/web"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
)

// User 是示例业务模型。
type User struct {
	ID   int    `gorm:"primarykey"`
	Name string `gorm:"size:64"`
}

func main() {
	err := appbox.New().Start(func(ctx context.Context, builder *appbox.ApplicationBuild) error {
		// 1. 日志
		builder.InitLog(".", "info")

		// 2. SQLite 数据库(零外部依赖,演示多实例与迁移)
		builder.EnableDatabase(&datasource.Config{
			Driver:      datasource.DriverSQLite,
			DSN:         "./web-basic.db",
			AutoMigrate: false,
		})

		// 3. 版本化迁移
		dbInstance, err := datasource.Get()
		if err != nil {
			return fmt.Errorf("get default database: %w", err)
		}
		migrator := datasource.NewMigrator(dbInstance,
			datasource.Migration{
				Name: "20260815_create_users",
				Up: func(db *gorm.DB) error {
					return db.AutoMigrate(&User{})
				},
			},
		)
		if err := migrator.Migrate(ctx); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}

		// 4. JWT 密钥(生产环境从配置/密钥管理系统注入)
		if err := apptoken.SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
			return err
		}

		// 5. Web 服务:中间件体系 + 认证 + 健康探针 + 统一响应
		builder.EnableWeb(appbox.TimeFormat, ":9528", "info", func(app *iris.Application) {
			app.Use(webiris.ErrorHandler, webiris.RequestID, webiris.AccessLog, webiris.SecurityHeaders)
			app.Use(webiris.CORS("https://trusted.example.com"))
			webiris.RegisterHealth(app, func() error {
				return datasource.Health(ctx)
			})

			// 公开接口:登录换取 token(携带 scope)
			app.Post("/api/v1/login", func(ctx iris.Context) {
				access, refresh, err := apptoken.GenTokenWithScope(42, "demo@example.com", "user:read")
				if err != nil {
					webiris.Fail(ctx, 500, 500, "generate token failed")
					return
				}
				webiris.OK(ctx, map[string]string{"access_token": access, "refresh_token": refresh})
			})

			// 认证接口(要求 scope:user:read;/health 与 /login 放行)
			app.Use(webiris.Auth(webiris.AuthConfig{
				Whitelist: []string{"/health", "/api/v1/login"},
				Scope:     "user:read",
			}))
			app.Get("/api/v1/me", func(ctx iris.Context) {
				webiris.OK(ctx, map[string]interface{}{
					"id":    webiris.UserID(ctx),
					"email": webiris.UserEmail(ctx),
				})
			})
			app.Get("/api/v1/users", func(ctx iris.Context) {
				var users []User
				if err := datasource.WithTx(ctx, func(tx *gorm.DB) error {
					return tx.Find(&users).Error
				}); err != nil {
					webiris.RespondError(ctx, err)
					return
				}
				webiris.OK(ctx, users)
			})
		})

		// 6. Admin 管理服务:pprof + metrics + 运行时日志级别 + 业务管理路由
		builder.EnableAdmin()
		builder.EnableAdminRoutes(func(app *iris.Application) {
			app.Get("/ops/demo", func(ctx iris.Context) {
				webiris.OK(ctx, map[string]string{"status": "operational"})
			})
		})

		return nil
	})
	if err != nil {
		log.Fatalf("start application failed: %v", err)
	}
}
