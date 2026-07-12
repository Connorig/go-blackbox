# go-blackbox

`go-blackbox` 是一个供其他 Go 项目引用的组件化 Web 脚手架依赖库。项目通过根包 `appbox` 的 Builder 统一装配 Web、数据源、缓存、定时任务、日志、静态资源和启动钩子，同时提供 MongoDB、RabbitMQ、JWT、RSA、邮件、构建脚本等独立能力。

> 本仓库有意不提供固定业务 `main` 包。依赖方在自己的项目中创建入口程序，通过 Builder 按需启用 Web、数据库和中间件组件。

## 当前状态

- Go 版本：`1.20`
- Module：`github.com/Connorig/go-blackbox`
- 根包名：`appbox`
- 默认分支：`main`
- 配置方式：Viper + TOML + 环境变量
- Web 框架：Iris v12
- 数据库：PostgreSQL + GORM
- 日志：Zap + file-rotatelogs
- 许可证：MIT

> 从 `v1.2.0` 开始，Go Module 地址统一为 `github.com/Connorig/go-blackbox`。旧的 `github.com/Domingor/go-blackbox` 引用需要更新后再执行 `go mod tidy`。

项目已有较完整的组件雏形，但部分组件仍存在初始化、错误处理、测试隔离和配置一致性问题。新功能开发前请先阅读 [项目分析与升级基线](docs/PROJECT_ANALYSIS.md)。

各功能模块的实施顺序、当前状态和验收目标见 [功能优化路线图](docs/OPTIMIZATION_ROADMAP.md)。

## 能力清单

| 能力 | 主要位置 | 当前接入方式 | 状态说明 |
| --- | --- | --- | --- |
| Iris Web 服务 | `server/webiris` | `EnableWeb` / `EnableWebWithConfig` | 已完成首轮生命周期与错误处理升级 |
| 关系数据库 / GORM | `server/datasource` | `EnableDb` / `EnableDatabase` | PostgreSQL 内置，MySQL/Oracle 可通过统一 Dialector 注册机制接入 |
| Redis 缓存 | `server/cache` | `EnableCache` | 已接入 Builder，初始化逻辑待修复 |
| MongoDB | `server/mongodb` | `EnableMongoDB` | 客户端已实现，Builder 开关待修复 |
| Cron 定时任务 | `server/cronjobs` | `SetSeeds` / `InitCronJob` | SetSeeds 注册任务后自动启用 Cron |
| Zap 分级日志 | `server/zaplog` | `InitLog` | 已接入 Builder，日志目录需提前准备 |
| Web 生命周期回调 | 根包 Builder | `BeforeSetup` / `AfterSetup` | 分别在 Web 启动前和 Ready 后执行 |
| 静态资源服务 | `static_`、`server/webiris` | `EnableStaticSource` | 已接入 Builder |
| 配置加载 | `server/apploader` | `LoadConfig` | 支持配置文件和环境变量，错误传递待完善 |
| RabbitMQ 重试队列 | `server/rabbitmqretry/rabbitmq` | 独立调用 | 尚未接入 Builder |
| JWT | `apputils/apptoken` | 独立调用 | 支持签发、验证、刷新，密钥配置待改造 |
| RSA | `apputils/rsa` | 独立调用 | 支持密钥生成、PEM 与 Base64 处理 |
| 邮件发送 | `server/email` | 独立调用 | 当前固定使用 SMTP 465 端口 |
| 构建脚本生成 | `buildscript` | `Generate` | 可生成 `build.sh` 和 `Dockerfile` |
| 前端代码 | 独立维护 | 未接入 | `v1.2.0` 已移除不可构建的 UI 代码片段 |

## 启动流程

调用 `appbox.New().Start(...)` 后，当前启动顺序如下：

1. 执行 Builder 回调，读取配置并标记需要启用的组件。
2. 初始化 Zap 日志；未显式配置时使用默认日志配置。
3. 按顺序初始化 PostgreSQL、Redis、MongoDB，并将实例写入简单 IOC 容器。
4. 执行 `BeforeSetup` 注册的 Web 启动前回调。
5. 在 goroutine 中启动 Iris Web 服务并等待 Ready。
6. 执行 `AfterSetup` 注册的 Web Ready 后回调。
7. 执行 `SetSeeds` 注册的 Cron 任务创建函数，并启动 Cron 调度器。
8. 主 goroutine 等待 `SIGINT`、`SIGTERM` 或 `shutdown.Exit(...)`。
9. 收到退出请求后，在统一关闭期限内按逆序停止业务关闭钩子、Cron、Web 运行 Context、MongoDB 和 PostgreSQL。

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
│   ├── apploader/               # Viper 配置加载
│   ├── cache/                   # Redis + 本地 TinyLFU 缓存
│   ├── cronjobs/                # Cron 调度器
│   ├── datasource/              # PostgreSQL / GORM
│   ├── email/                   # SMTP 邮件发送
│   ├── mongodb/                 # MongoDB 客户端封装
│   ├── rabbitmqretry/           # RabbitMQ 消息与失败重试
│   ├── shutdown/                # 信号监听与全局 Context
│   ├── webiris/                 # Iris Web 服务封装
│   └── zaplog/                  # Zap 日志和日志轮转
├── simpleioc/                   # 基于反射的全局实例容器
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

调用前必须确认对应组件已经启用且初始化成功。当前 IOC 容器不会为缺失实例返回结构化错误，直接使用空实例可能产生 panic，后续将改为显式依赖和错误返回。

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
- `server/mongodb/*_test.go`、`server/mongdbdemo/*_test.go`：连接真实 MongoDB。
- `server/rabbitmqretry/*_test.go`：连接真实 RabbitMQ，最长等待数分钟。
- `server/email/index_test.go`：连接真实 SMTP 并发送邮件。
- 部分 Cron、IOC 测试包含固定 `Sleep`。

因此，在完成测试分层和隔离前，不建议无条件执行 `go test ./...`。优先执行无外部依赖且可快速结束的目标测试，例如：

```bash
go test ./seed
go test ./simpleioc -run 'TestName|TestName2'
```

后续应通过 build tag、环境开关、容器化依赖或 mock 将集成测试与单元测试分离。

## 构建与部署

`buildscript.Generate(...)` 可以生成：

- `build.sh`：读取 Git 标签、Commit 和构建时间，构建或推送镜像。
- `Dockerfile`：多阶段构建 Go 服务，可选构建 UI，并通过 `ldflags` 注入版本信息。

推送 `v*` Tag 时，GitHub Actions 会校验 Module 地址、执行全包编译检查、生命周期单元测试和 `go vet`，验证通过后自动创建 GitHub Release。当前仓库作为依赖库发布，不再要求根目录存在 Dockerfile。

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
