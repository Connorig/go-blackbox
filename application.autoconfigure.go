package appbox

import (
	"time"

	"github.com/Connorig/go-blackbox/framework/cache"
	apploader "github.com/Connorig/go-blackbox/framework/config"
	"github.com/Connorig/go-blackbox/framework/database"
	mongodb "github.com/Connorig/go-blackbox/framework/mongo"
	"github.com/Connorig/go-blackbox/framework/monitor"
	"github.com/Connorig/go-blackbox/framework/openapi"
	"github.com/Connorig/go-blackbox/framework/web"
	"github.com/kataras/iris/v12"
)

// AutoConfigure 按配置自动装配模块(对标 Spring Boot Auto-Configuration):
// 业务在 config.toml 的 [modules] 段通过 enabled 开关模块,启动时按配置启用。
// 配置启用的模块才会初始化;未启用不占资源(热插拔)。
//
// 覆盖模块:Log / Auth(JWT 参数)/ Web(含安全基线)/ Database / Cache / Mongo / Cron / Admin / Monitor / OpenAPI。
// 手动调用 Enable* 与 AutoConfigure 互补:显式代码调用优先。
//
// 用法:
//
//	type AppConfig struct {
//	    apploader.Modules `mapstructure:",squash"`
//	    Business          BusinessConfig
//	}
//	var cfg AppConfig
//	builder.LoadConfig(&cfg, func(l apploader.Loader) {
//	    l.SetConfigFileSearcher("config", ".")
//	    l.EnableEnvSearcher("GBX")
//	})
//	builder.AutoConfigure(&cfg.Modules, func(app *iris.Application) {
//	    // 业务路由(Web 启用时自动挂载,安全基线已就位)
//	    app.Get("/api/v1/me", handler.Me)
//	})
func (app *ApplicationBuild) AutoConfigure(modules *apploader.Modules, webRoutes ...func(*iris.Application)) *ApplicationBuild {
	if app == nil || modules == nil {
		return app
	}

	// ① 日志
	if modules.Log.Enabled {
		app.InitLog(modules.Log.OutDir, modules.Log.Level)
	}

	// ② JWT 认证参数(密钥仍由业务 apptoken.SetSecretKey 注入)
	if modules.Auth.Enabled {
		accessTTL := modules.Auth.AccessTTL
		if accessTTL <= 0 {
			accessTTL = 30 * time.Minute
		}
		refreshTTL := modules.Auth.RefreshTTL
		if refreshTTL <= 0 {
			refreshTTL = 7 * 24 * time.Hour
		}
		app.SetupToken(accessTTL, refreshTTL, modules.Auth.Issuer)
	}

	// ③ 关系数据库
	if modules.Database.Enabled {
		app.EnableDatabase(&datasource.Config{
			Driver:       datasource.Driver(modules.Database.Driver),
			DSN:          modules.Database.DSN,
			Host:         modules.Database.Host,
			Port:         modules.Database.Port,
			UserName:     modules.Database.User,
			Password:     modules.Database.Password,
			Database:     modules.Database.Database,
			SSLMode:      modules.Database.SSLMode,
			AutoMigrate:  modules.Database.AutoMigrate,
			MaxIdleConns: modules.Database.MaxIdle,
			MaxOpenConns: modules.Database.MaxOpen,
		})
	}

	// ④ Redis 缓存
	if modules.Cache.Enabled {
		app.EnableCache(cache.RedisOptions{
			Addr:     modules.Cache.Addr,
			Password: modules.Cache.Password,
			DB:       modules.Cache.DB,
			PoolSize: modules.Cache.PoolSize,
		})
	}

	// ⑤ MongoDB
	if modules.Mongo.Enabled {
		timeout := modules.Mongo.Timeout
		if timeout <= 0 {
			timeout = 20
		}
		app.EnableMongoDB(&mongodb.MongoDBConfig{
			Timeout:  time.Duration(timeout) * time.Second,
			DB:       modules.Mongo.DB,
			Addr:     modules.Mongo.Addr,
			User:     modules.Mongo.User,
			Password: modules.Mongo.Password,
		})
	}

	// ⑥ 定时任务
	if modules.Cron.Enabled {
		app.InitCronJob()
	}

	// ⑦ Admin 管理服务
	if modules.Admin.Enabled {
		listen := modules.Admin.Listen
		if listen == "" {
			listen = ":6060"
		}
		app.EnableAdmin(listen)
	}

	// ⑧ Web 服务(含安全基线 + 路由级组件 + 业务路由)
	if modules.Web.Enabled && !app.IsEnableWeb {
		webModule := modules.Web
		app.EnableWeb(TimeFormat, webModule.Port, webModule.Level, func(webApp *iris.Application) {
			if webModule.Baseline {
				installWebBaseline(webApp, webModule)
			}
			// 健康探针
			webiris.RegisterHealth(webApp, nil)
			// 资源监控(路由级)
			if modules.Monitor.Enabled {
				path := modules.Monitor.Path
				if path == "" {
					path = "/monitor"
				}
				monitor.Register(webApp, path, monitor.Config{})
			}
			// 开放平台(路由级)
			if modules.OpenAPI.Enabled {
				installOpenAPI(webApp, modules.OpenAPI)
			}
			// 业务路由(最后挂载,安全基线已就位)
			for _, route := range webRoutes {
				if route != nil {
					route(webApp)
				}
			}
		})
	}

	return app
}

// installWebBaseline 挂载安全基线中间件(DoS 防护 + SQL 注入拦截 + 链路日志)。
func installWebBaseline(webApp *iris.Application, module apploader.WebModule) {
	rate := module.RatePerSecond
	if rate <= 0 {
		rate = 100
	}
	burst := module.Burst
	if burst <= 0 {
		burst = 200
	}
	maxBody := module.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 1 << 20
	}
	timeout := module.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	webApp.Use(webiris.ErrorHandler, webiris.RequestID, webiris.AccessLog, webiris.SecurityHeaders)
	webApp.Use(webiris.Limit(rate, burst, nil))
	webApp.Use(webiris.BodyLimit(maxBody))
	webApp.Use(webiris.Timeout(timeout))
	webApp.Use(webiris.SQLGuard())
}

// installOpenAPI 按配置注册开放平台应用并挂载网关。
func installOpenAPI(webApp *iris.Application, module apploader.OpenAPIModule) {
	registry := openapi.NewRegistry()
	for appKey, appConfig := range module.Apps {
		if !appConfig.Enabled {
			continue
		}
		algorithm := openapi.Algorithm(appConfig.Algorithm)
		if algorithm == "" {
			algorithm = openapi.AlgHMAC
		}
		_ = registry.Register(&openapi.App{
			AppKey:    appKey,
			AppSecret: appConfig.Secret,
			PublicKey: appConfig.Secret, // RSA 模式下 Secret 字段承载 PEM 公钥
			Algorithm: algorithm,
			Enabled:   true,
		})
	}
	openapi.New(webApp, openapi.Config{Registry: registry})
}
