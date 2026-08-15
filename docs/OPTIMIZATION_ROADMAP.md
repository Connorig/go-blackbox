# go-blackbox 功能优化路线图

本文以“供其他 Go 项目引用的 Web 脚手架依赖库”为定位，列出当前功能模块、优化内容、实施顺序和验收目标。每个模块应独立完成设计、实现、测试和文档更新后，再进入下一个模块。

## 统一开发要求

- 保持公开 API 兼容；必须调整时提供兼容入口、废弃说明和迁移示例。
- 所有 `error` 必须处理，失败路径既要返回错误，也要在合适层级记录日志。
- 错误使用 `%w` 保留错误链，日志不得输出密码、Token、私钥或完整连接串。
- 关键功能注释说明用途、输入约束、执行时机、并发行为、失败状态和资源释放责任。
- 所有外部服务调用支持 Context、超时和取消。
- 初始化成功的资源必须可以关闭；初始化失败不得把 nil 或半初始化实例注册到容器。
- 单元测试默认无网络、无固定长时间 Sleep；真实服务测试显式标记为集成测试。

## v1.2.0 模块迁移

仓库所有权和 Go Module 地址已从 `github.com/Domingor/go-blackbox` 迁移到 `github.com/Connorig/go-blackbox`。依赖项目升级到 `v1.2.0` 时，需要同步替换 import 路径并执行 `go mod tidy`。

## 模块清单

| 顺序 | 模块 | 主要目录 | 优化重点 | 状态 |
| --- | --- | --- | --- | --- |
| 1 | Web / Iris | `server/webiris`、Starter | 配置校验、Ready、启动错误、优雅关闭、静态资源 | 首轮已完成 |
| 2 | 应用生命周期 | 根包、`server/shutdown` | Start/Ready/Stop、资源逆序关闭、运行模式 | 首轮已完成 |
| 3 | 配置加载 | `server/apploader` | 错误返回、标签统一、默认值、校验、环境变量 | 首轮已完成 |
| 4 | 日志 | `server/zaplog` | 目录、分级策略、结构化字段、Sync、日志脱敏 | 首轮已完成 |
| 5 | 关系数据库 / GORM | `server/datasource` | PostgreSQL、MySQL、Oracle 扩展、Ping、连接池、迁移、关闭 | 首轮已完成 |
| 6 | Redis | `server/cache` | Ping 错误、TTL、序列化、关闭、健康检查 | 待开始 |
| 7 | MongoDB | `server/mongodb` | Builder 开关、Ping、超时、关闭、类型化 API | 待开始 |
| 8 | RabbitMQ | `server/rabbitmqretry/rabbitmq` | 客户端升级、确认语义、重连、重试、死信、关闭 | 待开始 |
| 9 | Cron / Seed | `server/cronjobs`、`seed` | 重复启动、任务错误、panic、退出等待、测试 | 待开始 |
| 10 | IOC / 依赖访问 | `gbxioc` | 并发安全、显式错误、可重置、减少全局状态 | 待开始 |
| 11 | JWT / RSA | `apputils/apptoken`、`apputils/rsa` | 密钥注入、算法限制、错误返回、轮换 | 待开始 |
| 12 | 邮件 | `server/email` | 端口/TLS 配置、Context、附件校验、连接错误 | 待开始 |
| 13 | 静态资源 / UI | `static_`、`ui` | SPA fallback、缓存、目录定位、构建边界 | 待开始 |
| 14 | 构建与版本 | `buildscript`、`version`、`.github` | Dockerfile、Go 版本、CI、版本注入、发布 | 待开始 |
| 15 | 通用工具 | `apputils/appcommon`、`apputils/gormp` | 重复代码、边界错误、随机数、文件与证书处理 | 待开始 |

## 1. Web / Iris

本轮已完成：

- 保留原有 `Init`、`EnableWeb` 调用方式。
- 新增 `webiris.Config`、`webiris.New`、`InitWithConfig` 和 `EnableWebWithConfig`。
- 校验监听地址、端口范围和 Iris 日志级别。
- 补齐默认时间格式、日志级别和优雅关闭超时。
- 启动前构建 Iris 路由，同步返回路由构建和端口监听错误。
- 使用 Iris Host OnServe 回调发布 Ready Channel，移除固定 Sleep 依赖。
- Context 取消时执行限时 Shutdown，并兜底关闭 Listener。
- 限制同一 WebIris 实例只能运行一次。
- 静态文件系统增加 nil 和调用时机检查。
- Starter 删除重复后置任务信号和共享 `err` 数据竞争。
- Web 异常停止时记录错误并结束应用等待流程。
- 新增 `BeforeSetup` 和 `AfterSetup`，明确区分 Web 启动前与 Ready 后回调。
- `SetSeeds` 专用于 Cron 任务注册，注册有效函数后自动启用 Cron。

后续 Web 增强项：

- 标准化 Request ID、访问日志、Recovery 响应和错误响应结构。
- 增加 CORS、安全 Header、请求体大小、读取/写入/空闲超时配置。
- 增加 `/health/live`、`/health/ready` 和依赖健康状态聚合。
- 增加 TLS、自定义 `http.Server`、可信代理和真实客户端 IP 配置。
- 明确静态资源与 API 根路由冲突、SPA fallback 和缓存策略。

验收目标：下游项目可以只启用 Web；端口冲突会从 `Start` 返回；收到退出信号后在超时内停止；启动和关闭测试不使用固定 Sleep。

## 2. 应用生命周期与 Shutdown

本轮已完成：

- 定义单个 Application 实例的 `created -> starting -> running -> stopping -> stopped` 状态，重复 Start 返回明确错误。
- 新增 `OnShutdown` 和 `WithShutdownTimeout`，允许下游 Worker 或自定义组件注册带 Context 的关闭函数。
- 成功初始化的 PostgreSQL、MongoDB、运行 Context 和 Cron 自动注册关闭函数，应用退出或启动失败时逆序释放。
- 多个关闭错误使用 `errors.Join` 聚合并保留错误链，Starter 记录功能点后将错误返回调用方。
- 系统信号、组件主动退出和 Web 致命错误统一进入退出通道；重复主动退出不再阻塞故障 goroutine。
- 每次系统信号等待结束后停止 `signal.Notify`，自定义信号不会修改包级默认配置。
- 删除 Shutdown 无限阻塞测试，新增无真实信号、无固定长时间 Sleep 的单元测试和 race 验证。

后续生命周期增强项：

- Redis 模块提供可靠 Close 后接入自动关闭；日志模块完成 Sync 错误分类后接入关闭栈。
- 明确 CLI 一次性任务完成后直接返回、Worker 主循环就绪信号以及组件健康状态聚合。
- 逐步移除进程级 Application、Context 和 IOC 单例，使多个应用实例可以在同一测试进程独立运行。

## 3. 配置与日志

配置首轮已完成：

- 保留 Loader 链式 API，配置阶段错误暂存并在 `LoadToStruct` 统一返回。
- 配置文件不存在、名称为空、解析失败、目标不是结构体指针时返回带功能点的错误。
- 环境变量前缀不再被清空，嵌套字段使用 `PREFIX_SECTION_FIELD` 格式覆盖。
- 内置配置统一补充 `mapstructure` 和 `toml` 标签，并修正连接池字段拼写。
- 保留 `MaxIdleCones`、`MaxOpenCones` 作为兼容字段，并同步正确字段的值。
- 为 Web、数据库连接池、Redis 连接池和日志配置增加安全默认值。
- 新增 `apploader.Validator`，业务配置可以在反序列化后执行必填项和范围校验。
- 新增无网络配置文件、环境变量、默认值、错误链和目标类型测试。

配置后续增强项：增加敏感字段脱敏快照、配置来源追踪、动态配置边界以及更完整的内置 MongoDB、RabbitMQ 配置模型。

日志首轮已完成：

- Init 创建实际写入使用的 `<日志根目录>/zap` 完整目录。
- 轮转写入器创建失败返回错误，不再在基础库内部 panic。
- 明确采用严格单级文件策略：debug、info、warn 分别只记录本级，error 文件记录 error 及更高等级。
- 每个等级使用独立的轮转文件和软链接，避免共享 `latest_log` 相互覆盖。
- 全局 Logger 在 Init 前使用 Nop 实现，避免初始化前日志调用 nil panic。
- 新增 `WithComponent`，为日志附加结构化 component 字段。
- 标准化 `timestamp`、`level`、`service`、`caller`、`function`、`message` 字段；时间使用带时区的 RFC3339 毫秒格式。
- 新增 `Sync` 并处理终端不支持 fsync 的平台错误，Starter 将日志关闭注册为关闭栈的最早资源，因此退出时最后刷新。
- 链式 `InitLog` 保持兼容，初始化错误暂存并在 Application.Start 中返回。
- 新增目录、严格分级、结构化字段、非法配置和 Sync 错误分类测试。

日志后续增强项：标准化 Request ID、Trace ID、敏感字段脱敏器、采样策略和日志写入失败指标。

配置应先完成基础读取；日志完成后，后续数据库和中间件模块统一使用新的错误记录规范。

## 4. 数据访问

### PostgreSQL / GORM

首轮已完成：

- 新增 `Initialize(ctx, config, models...)`，外部 Context 可以取消数据库启动和 Ping。
- 保留 `GormInit` 兼容入口，并在内部设置有上限的连接超时。
- 校验 nil 配置、主机、用户、数据库名、端口、SSL 模式、连接池范围和超时。
- 初始化时获取 `db.DB()` 并执行 `PingContext`，失败时关闭连接池且不注册全局实例。
- 移除失败后永久锁死的 `sync.Once`，初始化失败后允许安全重试。
- `AutoMigrate` 默认关闭，只有显式设置 `AutoMigrate` 或旧版 `InitDb` 时才执行。
- nil Model 使用新切片过滤，连续 nil 不会跳过且不修改调用方切片。
- 增加连接池默认值、最大生命周期配置、`Health(ctx)` 和幂等 `Close()`。
- Starter 直接使用 Context 初始化，并把 datasource.Close 注册到应用逆序关闭栈。
- 日志只记录主机、端口和数据库名，不输出密码或完整 DSN。

后续增强项：支持多数据源实例、只读副本、事务辅助接口、指标采集和可注入 Dialector 集成测试。

### 统一关系数据库适配

- 新增 `datasource.Driver` 和 `datasource.Config`，统一连接信息、连接池、超时和迁移配置。
- 新增 `InitializeDatabase`，不同关系数据库共享 Ping、失败清理、迁移、Health 和 Close 生命周期。
- 新增 `ApplicationBuild.EnableDatabase`，旧 `EnableDb(PostgresConfig)` 继续兼容。
- PostgreSQL 使用仓库已有官方 Dialector，兼容 `postgres`、`postgresql`、`pgsql` 名称。
- MySQL/MariaDB 使用统一字段和 `MySQLDSN` 生成连接串，调用方注册所选 GORM MySQL Dialector。
- Oracle 使用 `DriverOracle` 和 `RegisterDialector` 接入具体实现，避免基础库强制绑定 Oracle Client、CGO 或特定第三方驱动。
- 自定义数据库同样可以通过 `RegisterDialector` 接入，工厂返回 nil 或未注册驱动会返回明确错误。
- 日志和错误上下文只包含驱动、主机、端口和数据库名，不输出密码或完整 DSN。

### Redis

- 将 `Ping(ctx)` 判断改为检查 `.Err()`。
- 初始化返回 `(*RedisCache, error)`，失败时关闭客户端。
- 定义默认 TTL、无过期语义和序列化错误。
- 提供原生客户端、健康检查和关闭方法。

### MongoDB

- 修复 `EnableMongoDB` 错误设置 PostgreSQL 开关的问题。
- 初始化时校验配置、建立超时 Context 并执行 Ping。
- 为 Cursor 统一增加 Close 错误处理。
- 提供健康检查和 Disconnect 关闭钩子。

## 5. RabbitMQ

- 从停止维护的 `streadway/amqp` 迁移到维护中的客户端。
- 移除全局连接和 Channel，明确生产者、消费者实例所有权。
- 修复 Ack multiple、panic 恢复、重连次数和并发消费者行为。
- 统一声明 Exchange、Queue、Binding、DLX 和延时重试策略。
- Context 取消后停止消费并关闭 Channel/Connection。
- 区分可重试错误、不可重试错误和最终失败处理。

验收目标：断线能够按有上限的退避策略恢复；消息不会因错误 Ack 丢失；关闭时不新增消费任务。

## 建议实施顺序

1. 完成 Web 首轮升级并建立可执行测试环境。
2. 重构应用生命周期、Shutdown、配置和日志。
3. 依次优化 PostgreSQL、Redis、MongoDB。
4. 重写 RabbitMQ 连接与重试模型。
5. 完善 Cron、IOC、安全工具、邮件、静态资源和构建发布。

每完成一个模块，需要同步更新本文件的状态、README 示例和项目分析文档。
