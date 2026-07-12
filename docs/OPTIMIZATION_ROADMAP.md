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
| 3 | 配置加载 | `server/apploader` | 错误返回、标签统一、默认值、校验、环境变量 | 待开始 |
| 4 | 日志 | `server/zaplog` | 目录、分级策略、结构化字段、Sync、日志脱敏 | 待开始 |
| 5 | PostgreSQL / GORM | `server/datasource` | 配置校验、Ping、连接池、迁移策略、关闭 | 待开始 |
| 6 | Redis | `server/cache` | Ping 错误、TTL、序列化、关闭、健康检查 | 待开始 |
| 7 | MongoDB | `server/mongodb` | Builder 开关、Ping、超时、关闭、类型化 API | 待开始 |
| 8 | RabbitMQ | `server/rabbitmqretry/rabbitmq` | 客户端升级、确认语义、重连、重试、死信、关闭 | 待开始 |
| 9 | Cron / Seed | `server/cronjobs`、`seed` | 重复启动、任务错误、panic、退出等待、测试 | 待开始 |
| 10 | IOC / 依赖访问 | `simpleioc` | 并发安全、显式错误、可重置、减少全局状态 | 待开始 |
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

配置优化点：修复配置文件错误被吞、环境变量前缀失效、字段命名不一致，增加默认值、必填校验和脱敏输出。

日志优化点：创建完整日志目录，明确累计或独立分级策略，增加结构化字段、Request ID、组件名和 `Sync` 关闭处理。

配置应先完成基础读取；日志完成后，后续数据库和中间件模块统一使用新的错误记录规范。

## 4. 数据访问

### PostgreSQL / GORM

- 校验 nil 配置、DSN 必填项和连接池范围。
- 初始化时执行 `PingContext`，区分打开失败、认证失败和迁移失败。
- `AutoMigrate` 改为显式配置，生产环境默认关闭。
- 修复 nil Model 过滤和 `db.DB()` 错误忽略。
- 提供关闭连接池和健康检查入口。

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
