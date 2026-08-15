# go-blackbox

企业级 Go Web 应用脚手架(依赖库),**开箱即用、模块热插拔**——像 Spring Boot 一样:
`gbx new` 一键生成项目 → 按需 `Enable*` 开启模块 → 部署即得安全基线/认证/监控/开放平台。

| 项 | 值 |
|---|---|
| Go | 1.20 |
| Module | `github.com/Connorig/go-blackbox` |
| 根包 | `appbox` |
| Web | Iris v12 |
| ORM | GORM(PostgreSQL 内置,MySQL/Oracle/SQLite 可注册) |
| 日志 | Zap + rotatelogs |
| IOC | simpleioc v2(Spring 式:单例/多例/具名/生命周期) |
| 配置 | Viper + 文件热更新 |
| 发布 | 每功能点 tag 发布,GitHub Action 自动验证 + Release |

## 快速开始(3 分钟)

```bash
# 1. 用 CLI 生成项目(或直接引用本库)
go run github.com/Connorig/go-blackbox/tool/gbx new -name demo-app
cd demo-app

# 2. 拉依赖并启动
go mod tidy
go run .
```

启动即得:

| 入口 | 说明 |
|---|---|
| `http://localhost:8080/health/live` `/health/ready` | 健康探针 |
| `http://localhost:8080/monitor` | 服务器资源监控页(内存/CPU/磁盘/负载) |
| `POST /api/v1/login` | 登录签发 JWT(scope + 组织身份) |
| `http://localhost:6060` | Admin:pprof / metrics / 运行时日志级别切换 |

生成的骨架自带:**安全基线中间件、JWT 认证、数据权限隔离、SQLite + 迁移、监控**。
完整可运行示例见 [`examples/web-basic`](examples/web-basic/README.md)(全家桶)与 [`examples/openapi`](examples/openapi/README.md)(开放平台)。

## 分层架构(泰山版规范)

```
component/   通用能力(与业务无关)
├── auth/     JWT(token 轮换/scope/组织身份)、RSA 加解密
├── error/    阿里手册 A/B/C 错误码体系 + 统一错误(自动映射 HTTP 状态)
├── security/ SQL 注入检测(17 模式)+ 日志注入防护
├── util/     工具集:CopyProperties/DeepCopy/MD5/SHA/UUID/RandomString/时间...
└── mask/     脱敏器(Phone/Email/Name/IDCard/BankCard)

framework/   框架能力(面向业务)
├── web/      webiris:中间件体系/认证/限流/SQL 防护/统一响应/Admin/健康探针
├── database/ 多实例数据源 + 迁移 + DataScope 数据权限 + 雪花/UUID + 公共 Model
├── openapi/  开放 API 网关(AppKey 签名/防重放/限流/审计,业务零加密负担)
├── thirdparty/ 出站签名客户端(HMAC/RSA/Bearer + 重试 + 错误映射)
├── circuit/  熔断器(closed/open/half-open 状态机)
├── aop/      方法级切面(@Before/@After/@Around,Service 层日志/校验/耗时)
├── monitor/  服务器资源监控(跨平台采集 + 内置页面)
├── alert/    监控告警(规则引擎 + 企微/钉钉/飞书 webhook)
├── cache/    分布式锁/防击穿/防雪崩/Redis 封装
├── mq/       RabbitMQ 状态机(自动重连/Consumer/Producer)
├── push/      SSE 实时推送 / WebSocket Hub
├── event/    事件总线(同步/异步)
├── cron/     定时任务(含单例防重入)
├── mail/     邮件(gomail 封装)
├── mongo/    MongoDB(凭据 URI/泛型 FindTyped)
├── config/   配置加载(热更新/脱敏快照)
├── log/      Zap 结构化日志(运行时级别切换)
└── lifecycle/ 生命周期与优雅关闭

tool/gbx      代码生成 CLI:gbx new 一键生成业务项目骨架
```

## 模块文档总表(配置 / 调用 / 示例)

| 模块 | 启用/接入 | 关键 API | 文档 |
|---|---|---|---|
| Web 服务 | `builder.EnableWeb(timeFormat, port, level, func(app){...})` | `webiris.OK/Fail/RespondError` | [API_GUIDELINES](docs/API_GUIDELINES.md) |
| 中间件体系 | `app.Use(...)` | `RequestID/AccessLog/CORS/SecurityHeaders/ErrorHandler` | [SECURITY_GUIDELINES](docs/SECURITY_GUIDELINES.md) |
| 限流/DoS | `app.Use(webiris.Limit(...), webiris.BodyLimit(...), webiris.Timeout(...))` | 429 B0210 / 413 / 504 | [SECURITY_GUIDELINES](docs/SECURITY_GUIDELINES.md) |
| SQL 注入拦截 | `app.Use(webiris.SQLGuard())` | `security.IsSQLInjection()` | [SECURITY_GUIDELINES](docs/SECURITY_GUIDELINES.md) |
| JWT 认证 | `apptoken.SetSecretKey(...)` + `app.Use(webiris.Auth(cfg))` | `GenTokenFull/VerifyToken`, `webiris.UserID/DataScope` | [API_GUIDELINES](docs/API_GUIDELINES.md) |
| 数据权限 | `db.Scopes(webiris.DataScope(ctx).Condition())` | `model.SnowflakeModel/StringIDModel/OrgFields` | [DATABASE_STANDARDS](docs/DATABASE_STANDARDS.md) |
| 数据库 | `builder.EnableDatabase(&datasource.Config{...})` | `datasource.Get()/WithTx/NewMigrator` | [DATABASE_STANDARDS](docs/DATABASE_STANDARDS.md) |
| 开放平台(入站) | `openapi.New(app, cfg)` 注册式接口 | `api.GET/POST(handler)`,AppKey 签名 | [OPENAPI_GUIDELINES](docs/OPENAPI_GUIDELINES.md) |
| 第三方调用(出站) | `thirdparty.NewClient(cfg)` | `client.Get/Post` 自动签名 | [OPENAPI_GUIDELINES](docs/OPENAPI_GUIDELINES.md) |
| 熔断器 | `Config.Breaker: circuit.New(...)` | 失败率阈值/冷却/半开 | [OPENAPI_GUIDELINES](docs/OPENAPI_GUIDELINES.md) |
| 方法级 AOP | `aop.Around(fn, hook)` | `Before/After/Around` 装饰器 | 本文档下方 |
| 资源监控 | `monitor.Register(app, "/monitor", cfg)` | 页面 + `/monitor/api/stats` | [MONITOR_GUIDELINES](docs/MONITOR_GUIDELINES.md) |
| 监控告警 | `alert.NewWatcher(cfg)` + `go watcher.Start(ctx)` | CPU/内存/磁盘规则 + webhook | [ALERT_GUIDELINES](docs/ALERT_GUIDELINES.md) |
| 工具集 | `util.CopyProperties/DeepCopy/MD5/UUID...` | 对标 Java commons/hutool | [SECURITY_GUIDELINES](docs/SECURITY_GUIDELINES.md) |
| 代码生成 | `gbx new -name demo` | 一键生成项目骨架 | 本文档「快速开始」 |
| 事件总线 | `eventbus.New(...)` | `Subscribe/SubscribeAll/Publish` | — |
| SSE/WebSocket | `framework/push/sse`、`framework/push/ws` | 实时推送 | — |
| Cron | `builder.InitCronJob()` + `Register(name, spec, fn)` | 单例防重入 | — |
| Redis | `builder.EnableCache(redisOptions)` | 分布式锁/防击穿 | — |
| 邮件 | `mail.NewSender(cfg)` | TLS/附件 | — |

## AOP(面向切面)

**Web 层**:中间件即环绕切面——`AccessLog`(日志)、`Auth`(权限前置)、`Limit`(限流前置)、`ErrorHandler`(异常后置)。

**Service 层**:`framework/aop` 函数级切面(对标 Spring @Before/@After/@Around):

```go
// 原始方法(任意签名)
getUser := func(ctx context.Context, id int64) (*User, error) { ... }

// 前置校验:参数非法直接终止(不执行目标方法)
getUser = aop.Before(getUser, func(ctx context.Context, params []interface{}) error {
    if params[1].(int64) <= 0 { return errors.New("invalid id") }
    return nil
}).(func(context.Context, int64) (*User, error))

// 环绕切面:耗时 + 日志
getUser = aop.Around(getUser, func(ctx context.Context, params []interface{},
    next func() ([]interface{}, error)) ([]interface{}, error) {
    start := time.Now()
    results, err := next()
    log.Printf("GetUser cost=%s", time.Since(start))
    return results, err
}).(func(context.Context, int64) (*User, error))
```

切面可组合(Before + Around + After),签名不变,业务调用方无感知;配合 simpleioc 将装饰后的 Service 注册进容器。

## 热插拔模块(按需启用)

| 开关 | 能力 | 说明 |
|---|---|---|
| `builder.EnableWeb(...)` | Web 服务 | 核心 |
| `builder.EnableDatabase(...)` / `EnableDb(...)` | 关系数据库 | PostgreSQL 内置;MySQL/Oracle 注册 Dialector |
| `builder.EnableNamedDatabase(name, ...)` | 多实例数据库 | 并行实例,独立生命周期 |
| `builder.EnableCache(redisOptions)` | Redis 缓存/分布式锁 | 未启用时相关 API 返回明确错误 |
| `builder.EnableMongoDB(cfg)` | MongoDB | 凭据 URI、超时 Ping |
| `builder.EnableAdmin(:6060)` | Admin 服务 | pprof + metrics + 日志级别 + 业务路由 |
| `builder.InitCronJob()` | 定时任务 | 与 `SetSeeds` 配合 |
| `builder.SetupToken(...)` | JWT 参数 | 过期时间/签发者 |
| `builder.EnableStaticSource(embed.FS)` | 静态资源 | Vue 打包产物嵌入 |
| `builder.BeforeSetup / AfterSetup` | 生命周期钩子 | 数据就绪后/Web 启动前等时机 |
| `builder.OnShutdown(name, fn)` | 关闭钩子 | 逆序执行 |
| `monitor.Register(app, ...)` | 监控(路由级) | 按路由挂载,不启用则无 |
| `openapi.New(app, ...)` | 开放平台(路由级) | 同上 |

未启用的模块不初始化、不占资源——这就是「热插拔」:业务项目按需组合。

## 完整文档索引

| 文档 | 内容 |
|---|---|
| [API_GUIDELINES.md](docs/API_GUIDELINES.md) | URL/响应/错误码/分页/幂等规范 |
| [DATABASE_STANDARDS.md](docs/DATABASE_STANDARDS.md) | 建表规范/索引/公共 Model/数据权限 |
| [OPENAPI_GUIDELINES.md](docs/OPENAPI_GUIDELINES.md) | 开放平台签名规范/出站客户端/熔断 |
| [SECURITY_GUIDELINES.md](docs/SECURITY_GUIDELINES.md) | 安全基线组合/注入防护/工具集 |
| [MONITOR_GUIDELINES.md](docs/MONITOR_GUIDELINES.md) | 资源监控接入/数据结构/平台矩阵 |
| [ALERT_GUIDELINES.md](docs/ALERT_GUIDELINES.md) | 告警规则/机器人渠道/自定义 |
| [DEVELOPMENT_STANDARDS.md](docs/DEVELOPMENT_STANDARDS.md) | 泰山版开发规范(代码/命名/注释) |
| [PROJECT_ANALYSIS.md](docs/PROJECT_ANALYSIS.md) | 项目分析与升级基线 |
| [OPTIMIZATION_ROADMAP.md](docs/OPTIMIZATION_ROADMAP.md) | 功能优化路线图 |

## 版本历史(近期)

| 版本 | 内容 |
|---|---|
| v1.18.0 | 方法级 AOP + 文档体系完善 |
| v1.17.0 | 全家桶示例(一次跑通全部能力) |
| v1.16.0 | 监控告警(企微/钉钉/飞书) |
| v1.15.0 | 熔断器 |
| v1.14.0 | gbx 代码生成 CLI |
| v1.13.0 | 服务器资源监控 |
| v1.12.0 | SQL 注入防护 + DoS 中间件 + 工具集 |
| v1.11.0 | 数据权限(组织/部门隔离) |
| v1.10.0 | 开放平台(网关 + 出站客户端 + 雪花/UUID) |
| v1.9.0 | 泰山版规范落地(A/B/C 错误码 + StandardModel) |
| v1.8.0 | 分层目录重构 |

## 许可证

MIT
