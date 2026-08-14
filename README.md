# go-blackbox

`go-blackbox` 是一个供其他 Go 项目引用的组件化 Web 脚手架依赖库。项目通过根包 `appbox` 的 Builder 统一装配 Web、数据源、缓存、定时任务、日志、静态资源和启动钩子，同时提供 MongoDB、RabbitMQ、JWT、RSA、邮件、构建脚本等独立能力。

> 本仓库有意不提供固定业务 `main` 包。依赖方在自己的项目中创建入口程序，通过 Builder 按需启用 Web、数据库和中间件组件。

## 当前状态

- Go 版本：`1.20`
- Module：`github.com/Connorig/go-blackbox`
- 根包名：`appbox`
- 默认分支：`main`
- 配置方式：Viper + TOML + 环境变量 + 文件热更新
- Web 框架：Iris v12（内置 RequestID/访问日志/CORS/安全头/健康探针/统一响应）
- 数据库：PostgreSQL + GORM（统一 Dialector 注册，支持 MySQL/Oracle）
- 日志：Zap + file-rotatelogs（严格单级文件、结构化字段、安全关闭）
- 依赖容器：simpleioc v2（单例/多例/具名注册/生命周期钩子，Spring 式）
- 许可证：MIT

> 从 `v1.2.0` 开始，Go Module 地址统一为 `github.com/Connorig/go-blackbox`。旧的 `github.com/Domingor/go-blackbox` 引用需要更新后再执行 `go mod tidy`。
>
> `v1.3.0` 完成安全加固与组件升级：simpleioc v2、Redis/MongoDB/Cron/邮件/配置热更新/Web 中间件优化，详见 [v1.3.0 升级摘要](#v130-升级摘要)。

项目已有较完整的组件雏形，但部分组件仍存在初始化、错误处理、测试隔离和配置一致性问题。新功能开发前请先阅读 [项目分析与升级基线](docs/PROJECT_ANALYSIS.md)。

各功能模块的实施顺序、当前状态和验收目标见 [功能优化路线图](docs/OPTIMIZATION_ROADMAP.md)。

## 能力清单

| 能力 | 主要位置 | 当前接入方式 | 状态说明 |
| --- | --- | --- | --- |
| Iris Web 服务 | `server/webiris` | `EnableWeb` / `EnableWebWithConfig` | 已完成生命周期、优雅关闭与中间件体系升级 |
| Web 中间件 | `server/webiris` | `app.Use(RequestID, AccessLog, SecurityHeaders)` / `app.Use(CORS())` | RequestID、访问日志、CORS、安全头 |
| 健康探针 | `server/webiris` | `RegisterHealth(app, ready)` | `/health/live` 与 `/health/ready` |
| 统一响应 | `server/webiris` | `OK(ctx, data)` / `Fail(ctx, status, code, msg)` | 业务码 + 消息 + 数据 |
| 关系数据库 / GORM | `server/datasource` | `EnableDb` / `EnableDatabase` | PostgreSQL 内置，MySQL/Oracle 可通过统一 Dialector 注册机制接入 |
| Redis 缓存 | `server/cache` | `EnableCache` | 已修复 Ping 判断、支持重试与关闭接入，提供 Health 与默认 TTL |
| MongoDB | `server/mongodb` | `EnableMongoDB` | 已修复开关与解码，支持凭据 URI、超时 Ping、泛型 FindTyped |
| Cron 定时任务 | `server/cronjobs` | `SetSeeds` / `Register(name, spec, fn)` | 具名任务注册表 + panic 恢复 + 结构化日志 |
| Zap 分级日志 | `server/zaplog` | `InitLog` | 严格单级文件、结构化字段、Close 释放句柄 |
| Web 生命周期回调 | 根包 Builder | `BeforeSetup` / `AfterSetup` | 分别在 Web 启动前和 Ready 后执行 |
| 静态资源服务 | `static_`、`server/webiris` | `EnableStaticSource` | 已接入 Builder |
| 配置加载 | `server/apploader` | `LoadConfig` / `Watch` | 文件/环境变量加载 + Validator 校验 + 热更新回调 |
| 依赖注入容器 | `simpleioc` | `Register` / `GetBean` | v2：单例/多例/具名/构造注入/生命周期钩子，兼容旧 API |
| RabbitMQ 重试队列 | `server/rabbitmqretry/rabbitmq` | 独立调用 | 已迁移 amqp091-go，修复确认语义与 panic 恢复 |
| JWT | `apputils/apptoken` | `SetSecretKey` + `GenToken` | 密钥注入、算法白名单、签发者校验 |
| RSA | `apputils/rsa` | 独立调用 | nil 安全，Safe 版 API 返回错误 |
| 邮件发送 | `server/email` | 独立调用 | 端口/TLS 配置化、附件校验、超时 |
| 构建脚本生成 | `buildscript` | `Generate` | 可生成 `build.sh` 和 `Dockerfile`（Go 1.20） |
| 前端代码 | 独立维护 | 未接入 | `v1.2.0` 已移除不可构建的 UI 代码片段 |

## 启动流程

调用 `appbox.New().Start(...)` 后，当前启动顺序如下：

1. 执行 Builder 回调，读取配置并标记需要启用的组件。
2. 初始化 Zap 日志；未显式配置时使用默认日志配置。
3. 按顺序初始化 PostgreSQL、Redis（Ping 校验）、MongoDB（超时 Ping），并注册到 IOC 容器。
4. 启动 IOC 容器（按注册顺序构造单例并执行 OnInit 钩子）。
5. 执行 `BeforeSetup` 注册的 Web 启动前回调。
6. 在 goroutine 中启动 Iris Web 服务并等待 Ready。
7. 执行 `AfterSetup` 注册的 Web Ready 后回调。
8. 执行 `SetSeeds` 注册的 Cron 任务创建函数，并启动 Cron 调度器。
9. 主 goroutine 等待 `SIGINT`、`SIGTERM` 或 `shutdown.Exit(...)`。
10. 收到退出请求后，在统一关闭期限内按逆序停止 IOC 容器、业务关闭钩子、Cron、Web 运行 Context、Redis、MongoDB 和 PostgreSQL。

Web 服务会在 Iris 路由构建完成且 TCP Listener 真正进入 Serve 阶段后发布 Ready 信号。未启用 Web 时不会执行 BeforeSetup 和 AfterSetup；SetSeeds 不依赖 Web，可用于 Cron-only 服务。

下游 Worker 或其他自定义资源可以通过 `OnShutdown` 注册关闭函数，并使用 `WithShutdownTimeout` 设置全部资源共享的关闭期限。关闭函数必须响应传入 Context，且不得记录敏感连接信息。

## 目录结构

```text
.
├── application.builder.go       # 组件启用与配置入口
├── application.starter.go       # 应用启动、服务初始化和退出等待
├── apputils/                    # JWT、RSA、GORM 条件、通用工具
├── buildscript/                 # Dockerfile / build.sh 模板生成器
├── seed/                        # 生命周期回调函数类型与兼容执行入口
├── server/
│   ├── apploader/               # Viper 配置加载 + 热更新
│   ├── cache/                   # Redis + 本地 TinyLFU 缓存
│   ├── cronjobs/                # Cron 调度器 + 具名任务注册表
│   ├── datasource/              # PostgreSQL / GORM
│   ├── email/                   # SMTP 邮件发送（端口/TLS 可配置）
│   ├── mongodb/                 # MongoDB 客户端封装（泛型查询）
│   ├── rabbitmqretry/           # RabbitMQ 消息与失败重试
│   ├── shutdown/                # 信号监听与全局 Context
│   ├── webiris/                 # Iris Web 服务封装 + 中间件体系
│   └── zaplog/                  # Zap 日志和日志轮转
├── simpleioc/                   # Spring 式依赖容器（单例/多例/生命周期）
├── static_/                     # Go embed 静态资源示例
├── version/                     # 构建版本信息注入与输出
└── config.toml                  # 示例配置，禁止直接用于生产环境
```

## 快速开始

### 1. 环境准备

安装 Go 1.20 或兼容版本，然后下载依赖：

```bash
go mod download
```

日志初始化会自动创建 `<日志目录>/zap/`。debug、info、warn 文件只记录对应等级，error 文件记录 error 及更高等级；应用退出时会在关闭栈最后刷新日志缓冲区。

需要为日志增加功能模块字段时，可以使用：

```go
zaplog.WithComponent("order-worker").Infow("worker started", "queue", queueName)
```

JSON 格式使用稳定字段名，便于 Loki、Elasticsearch 等日志平台检索：

```json
{
  "timestamp": "2026-07-12T19:49:24.464+08:00",
  "level": "info",
  "service": "go-blackbox",
  "component": "order-worker",
  "caller": "appbox/worker.go:42",
  "function": "example.com/project/appbox.(*Worker).Start",
  "message": "worker started"
}
```

`timestamp` 使用带时区的 RFC3339 毫秒格式；`caller` 用于快速定位文件和行号，`function` 用于定位包、类型和方法。生产环境建议使用 JSON，开发环境可以继续使用 console 格式。

### 2. 创建入口程序

可以在独立应用中引用本仓库，也可以在仓库内新增 `cmd/demo/main.go`：

```go
package main

import (
	"context"
	"log"

	appbox "github.com/Connorig/go-blackbox"
	"github.com/kataras/iris/v12"
)

func main() {
	err := appbox.New().Start(func(_ context.Context, builder *appbox.ApplicationBuild) error {
		// 当前示例仅启用日志和 Web，避免依赖外部数据库、中间件服务。
		builder.
			InitLog(".", "info").
			EnableWeb(appbox.TimeFormat, ":9528", "info", func(app *iris.Application) {
				app.Get("/health", func(ctx iris.Context) {
					if _, err := ctx.WriteString("ok"); err != nil {
						log.Printf("write health response failed: %v", err)
					}
				})
			})

		return nil
	})
	if err != nil {
		log.Fatalf("start application failed: %v", err)
	}
}
```

启动后访问：

```text
GET http://127.0.0.1:9528/health
```

使用 `Ctrl+C` 发送退出信号。

需要配置优雅关闭超时时，可使用结构化入口：

```go
builder.EnableWebWithConfig(webiris.Config{
	Address:         ":9528",
	TimeFormat:      appbox.TimeFormat,
	LogLevel:        "info",
	ShutdownTimeout: 15 * time.Second,
}, registerRoutes)
```

### 3. 中间件与健康探针

```go
builder.EnableWeb(appbox.TimeFormat, ":9528", "info", func(app *iris.Application) {
	app.Use(webiris.RequestID, webiris.AccessLog, webiris.SecurityHeaders)
	app.Use(webiris.CORS("https://trusted.example.com")) // 或 webiris.CORS() 允许所有
	webiris.RegisterHealth(app, func() error {
		return datasource.Health(context.Background()) // 就绪依赖检查
	})
	app.Get("/api/v1/hello", func(ctx iris.Context) {
		webiris.OK(ctx, map[string]string{"message": "hello"})
	})
})
```

## 配置加载

Builder 可通过 `LoadConfig` 使用 Viper 读取配置文件：

```go
var cfg AppConfig

if err := builder.LoadConfig(&cfg, func(loader apploader.Loader) {
	loader.SetConfigFileSearcher("config", ".")
}); err != nil {
	return fmt.Errorf("load application config: %w", err)
}
```

建议业务项目定义自己的配置结构，并使用 `mapstructure` 标签与 Viper 对齐。仓库内置的 `apploader.Configuration` 只覆盖 Web、数据库、Redis 和日志的一部分字段，尚未覆盖 RabbitMQ 与 MongoDB。

启用环境变量时，嵌套字段使用 `PREFIX_SECTION_FIELD` 命名。例如：

```go
loader.SetConfigFileSearcher("config", ".").EnableEnvSearcher("BLACKBOX")
```

`BLACKBOX_WEB_LISTEN` 会覆盖 `web.listen`。业务配置可以实现 `apploader.Validator`，在反序列化完成后执行必填项和范围校验；校验错误会从 `LoadConfig` 返回并终止应用启动。

配置文件热更新（v1.3.0 新增）：

```go
if err := loader.SetConfigFileSearcher("config", ".").LoadToStruct(&cfg); err != nil {
	return err
}
if err := loader.Watch(func() {
	// cfg 已重载；在此重建受影响的组件
}); err != nil {
	return err
}
```

`config.toml` 目前包含开发环境连接示例。使用前必须替换主机、账号和密码；生产密钥、数据库密码、SMTP 授权码等敏感信息应通过密钥管理系统或环境变量注入，不应提交到仓库。

## 组件访问

启用并成功初始化组件后，可以通过根包提供的访问函数获取全局实例：

```go
db := appbox.GormDb()
redisCache := appbox.RedisCache()
mongoClient := appbox.MongoDb()
cron := appbox.CronJobSingle()
globalCtx := appbox.GlobalCtx()
```

调用前必须确认对应组件已经启用且初始化成功。

### simpleioc v2：Spring 式依赖容器

v1.3.0 起 `simpleioc` 升级为线程安全、支持作用域与生命周期钩子的依赖容器，推荐使用泛型 API：

```go
// 注册：类型单例（懒加载）、具名单例、原型（多例）、直接实例
simpleioc.Register(func() *UserService { return &UserService{Repo: repo} })
simpleioc.RegisterNamed("primary-db", func() *gorm.DB { return primaryDB })
simpleioc.RegisterPrototype(func() *RequestContext { return &RequestContext{} })
simpleioc.RegisterInstance(repo)

// 获取
service, err := simpleioc.Get[UserService]()        // (*UserService, error)
repo := simpleioc.MustGet[Repository]()             // 启动期 panic 版本
named, err := simpleioc.GetNamed[gorm.DB]("primary-db")

// 生命周期：实现 Initializer/Disposer 接口的实例自动获得 OnInit/OnDestroy
type CacheWarmer struct{}
func (c *CacheWarmer) OnInit(ctx context.Context) error   { return warmCache(ctx) }
func (c *CacheWarmer) OnDestroy(ctx context.Context) error { return closePool() }
```

- 单例懒加载：首次 `GetBean` 或容器 `Start` 时构造；`Start(ctx)` 按注册顺序调用 OnInit，`Shutdown(ctx)` 逆序调用 OnDestroy（已接入应用关闭栈）。
- 错误语义：`ErrNotFound` / `ErrDuplicate` / `ErrInvalidProvider` / `ErrCircularDependency` / `ErrContainerClosed`。
- 多实例场景使用 `simpleioc.NewContainer()` 创建独立容器（测试隔离）。
- 旧 API（`Set` / `Get` / `GetDb` 等）保留为兼容层，新代码请使用泛型 API。

PostgreSQL 默认不会自动迁移表结构。需要迁移时必须显式配置：

```go
builder.EnableDb(&datasource.PostgresConfig{
	Host:           "127.0.0.1",
	Port:           5432,
	UserName:       "postgres",
	Password:       os.Getenv("POSTGRES_PASSWORD"),
	DbName:         "application",
	SSL:            "disable",
	MaxIdleConns:   10,
	MaxOpenConns:   20,
	ConnectTimeout: 10 * time.Second,
	AutoMigrate:    true,
}, models...)
```

脚手架启动时会执行 `PingContext`，退出时自动关闭连接池。独立使用数据源包时，可以调用 `datasource.Health(ctx)` 和 `datasource.Close()`。

统一关系数据库入口使用 `datasource.Config`：

```go
builder.EnableDatabase(&datasource.Config{
	Driver:          datasource.DriverPostgreSQL,
	Host:            "127.0.0.1",
	Port:            5432,
	UserName:        "postgres",
	Password:        os.Getenv("DATABASE_PASSWORD"),
	Database:        "application",
	SSLMode:         "disable",
	ConnectTimeout:  10 * time.Second,
	MaxIdleConns:    10,
	MaxOpenConns:    20,
	ConnMaxLifetime: time.Hour,
	AutoMigrate:     false,
}, models...)
```

MySQL/MariaDB 由业务项目选择 GORM Driver 并注册，基础库负责其余生命周期：

```go
if err := datasource.RegisterDialector(datasource.DriverMySQL, func(config datasource.Config) (gorm.Dialector, error) {
	dsn, err := datasource.MySQLDSN(config)
	if err != nil {
		return nil, err
	}
	return mysql.Open(dsn), nil
}); err != nil {
	return fmt.Errorf("register MySQL dialector: %w", err)
}
```

Oracle 使用同一个 `RegisterDialector(datasource.DriverOracle, ...)` 接口注册项目选定的 Oracle GORM Driver。Oracle DSN、Wallet、Service Name 和客户端库由注册工厂负责，禁止记录完整连接串。

## 静态资源

`static_` 使用 `embed.FS` 嵌入静态文件：

```go
builder.
	EnableStaticSource(static_.StaticFile).
	EnableWeb(appbox.TimeFormat, ":9528", "info", registerRoutes)
```

Iris 会将嵌入文件映射到 `/`。如果同时注册根路径路由，需要提前确认路由优先级和 SPA fallback 行为。

## 测试说明

当前测试同时包含单元测试、真实中间件集成测试和人工演示用例。以下测试依赖外部环境或会长时间阻塞：

- `application.builder_test.go`：真实 Web 集成测试，默认跳过；设置 `GO_BLACKBOX_WEB_INTEGRATION=1` 后执行。
- `server/mongodb/*_test.go`、`server/mongdbdemo/*_test.go`：连接真实 MongoDB，设置 `GO_BLACKBOX_MONGO_ADDR` 后执行。
- `server/rabbitmqretry/*_test.go`：连接真实 RabbitMQ，设置 `GO_BLACKBOX_RABBITMQ_DNS` 后执行。
- `server/email/index_test.go`：连接真实 SMTP，设置 `GO_BLACKBOX_SMTP_USER/PASS/HOST/TO` 后执行。
- `server/cache/redis_test.go`：连接真实 Redis，设置 `GO_BLACKBOX_REDIS_ADDR` 后执行。

默认执行 `go test ./...` 即可完成全部单元测试与快速集成测试（未设置环境变量的集成测试自动跳过）。

## 构建与部署

`buildscript.Generate(...)` 可以生成：

- `build.sh`：读取 Git 标签、Commit 和构建时间，构建或推送镜像。
- `Dockerfile`：多阶段构建 Go 服务（Go 1.20），可选构建 UI，并通过 `ldflags` 注入版本信息。

推送 `v*` Tag 时，GitHub Actions 会校验 Module 地址、执行全包编译检查、生命周期单元测试和 `go vet`，验证通过后自动创建 GitHub Release。当前仓库作为依赖库发布，不再要求根目录存在 Dockerfile。

## v1.3.0 升级摘要

本轮升级聚焦安全加固、容器能力与组件完备度：

- **安全**：JWT 密钥强制注入（≥32 字节）+ HS256 算法白名单 + 签发者校验；RSA 全部解析函数 nil 安全并提供 Safe 版；移除仓库内真实邮箱凭据；MongoDB 支持凭据 URI（URL 编码）。
- **simpleioc v2**：单例/原型作用域、具名注册、构造注入、OnInit/OnDestroy 生命周期钩子、循环依赖检测、并发安全、Reset 测试隔离；旧 API 兼容。
- **Redis**：修复 Ping 判断、失败可重试、Close/Health、默认 TTL。
- **MongoDB**：修复 Builder 开关与解码 bug、启动期超时 Ping、泛型 `FindTyped`。
- **Cron**：具名任务注册表 `Register/List/Remove` + panic 恢复 + 结构化日志。
- **邮件**：端口/TLS 配置化（465/587）、附件校验、未配置明确报错。
- **配置**：`Watch` 文件热更新（重载 + 校验 + 回调）。
- **Web**：RequestID / AccessLog / CORS / SecurityHeaders 中间件、`/health/live` 与 `/health/ready` 探针、统一响应 `OK/Fail`。
- **RabbitMQ**：迁移到维护中的 amqp091-go，修复单条确认语义、panic 恢复与信道关闭。
- **工程化**：测试全量隔离（环境变量门控）、移除固定 Sleep、go.mod tidy、buildscript Go 版本对齐、version JSON 输出、appcommon 包名对齐。

## 开发约定

- 每次修改前必须阅读并遵守仓库根目录的 [AGENTS.md](AGENTS.md)。
- 所有业务功能返回的 `error` 必须被处理，不允许静默丢弃。
- 错误日志应包含功能点、关键上下文和原始错误，包装错误时使用 `%w` 保留错误链。
- 不在日志中输出密码、Token、私钥、完整连接串等敏感信息。
- 关键功能注释需要说明用途、输入约束、执行时机、失败行为和资源释放方式。
- 新增外部依赖能力时，需要同时提供超时、取消、重试边界、健康检查和关闭逻辑。
- 新增功能必须补充可重复执行的测试，真实外部服务测试与单元测试分离。

## 后续优化

已确认的问题、风险等级和建议实施顺序见 [docs/PROJECT_ANALYSIS.md](docs/PROJECT_ANALYSIS.md)。建议首先处理启动生命周期、敏感配置、测试隔离和错误处理，再扩展新业务功能。
