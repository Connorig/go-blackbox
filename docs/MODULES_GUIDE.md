# go-blackbox 模块使用手册(总索引)

本手册是全部模块/工具的使用文档入口:每个模块的定位、启用方式、关键 API、示例与详细文档链接。
**开发任何模块前先查这里;文档缺失先补文档再开发。**

## 一、核心框架

| 模块 | 定位 | 启用/接入 | 关键 API | 详细文档 |
|---|---|---|---|---|
| **Web 服务** | Iris 装配 + 中间件体系 | `builder.EnableWeb(...)` | `webiris.OK/Fail/RespondError` | [API_GUIDELINES](API_GUIDELINES.md) |
| **中间件** | 日志/链路/安全头/错误 | `app.Use(...)` | `RequestID/AccessLog/CORS/SecurityHeaders/ErrorHandler` | [SECURITY_GUIDELINES](SECURITY_GUIDELINES.md) |
| **安全防护** | DoS + SQL 注入 | `app.Use(Limit/BodyLimit/Timeout/SQLGuard)` | `security.IsSQLInjection` | [SECURITY_GUIDELINES](SECURITY_GUIDELINES.md) |
| **JWT 认证** | 登录/权限/组织身份 | `apptoken.SetSecretKey` + `app.Use(Auth)` | `GenTokenFull/VerifyToken`、`UserID/DataScope` | [API_GUIDELINES](API_GUIDELINES.md) |
| **错误码** | A/B/C 手册码 | `apperr.New/Wrap` | `Code*` 常量 + `HTTPStatus` 映射 | [API_GUIDELINES](API_GUIDELINES.md) |
| **gbxioc** | 依赖注入容器 | `gbxioc.Register/RegisterInstance` | `GetBean/MustGetBean/Start/Shutdown` | —(见 README 容器章节) |
| **分层配置** | 父子覆盖 + 模块开关 | `builder.LoadConfig` + `AutoConfigure` | `SetGlobalConfigFile/SetConfigFileSearcher` | [CONFIGURATION_GUIDELINES](CONFIGURATION_GUIDELINES.md) |
| **AOP** | 方法级切面 | `aop.NewProxy` / `aop.Before/After/Around` | `Aspect/JoinPoint/Proxy` | README AOP 章节 |
| **数据权限** | 组织/部门隔离 | `db.Scopes(DataScope(ctx).Condition())` | `WithScope/ScopeFrom` | [DATABASE_STANDARDS](DATABASE_STANDARDS.md) |

## 二、数据与存储

| 模块 | 定位 | 启用/接入 | 关键 API | 详细文档 |
|---|---|---|---|---|
| **关系数据库** | GORM 多实例 + 迁移 + 事务 | `builder.EnableDatabase` | `datasource.Get/WithTx/NewMigrator` | [DATABASE_STANDARDS](DATABASE_STANDARDS.md) |
| **公共 Model** | 标准字段/ID 策略 | 模型内嵌 | `StandardModel/SnowflakeModel/StringIDModel/OrgFields` | [DATABASE_STANDARDS](DATABASE_STANDARDS.md) |
| **Redis** | 缓存 + Template | `builder.EnableCache` | `RedisCache.Get/Set/Incr/HSet/LPush/Client()` | [MIDDLEWARE_GUIDELINES](MIDDLEWARE_GUIDELINES.md) |
| **MongoDB** | 文档库 | `builder.EnableMongoDB` | `Client.Find/InsertMany/Count/Client()` | [MIDDLEWARE_GUIDELINES](MIDDLEWARE_GUIDELINES.md) |
| **ElasticSearch** | 全文检索 | `es.NewClient` | `Index/Search/CreateIndex/Client()` | [ES_STORAGE_GUIDELINES](ES_STORAGE_GUIDELINES.md) |
| **对象存储** | MinIO/OSS | `storage.NewClient` | `Put/Get/PresignedPut/Client()` | [ES_STORAGE_GUIDELINES](ES_STORAGE_GUIDELINES.md) |
| **InfluxDB** | 时序库 | `influx.NewClient` | `Write/Query/WriteRaw/Client()` | [INFLUX_GUIDELINES](INFLUX_GUIDELINES.md) |

## 三、消息与集成

| 模块 | 定位 | 启用/接入 | 关键 API | 详细文档 |
|---|---|---|---|---|
| **RabbitMQ** | 消息队列(状态机) | `mq.Dial` + `NewProducer/NewConsumer` | `Publish/PublishDelay/Channel()` | [MIDDLEWARE_GUIDELINES](MIDDLEWARE_GUIDELINES.md) |
| **Kafka** | 高吞吐消息 | `kafka.NewProducer/NewConsumer` | `Send/SendJSON/Consume` | [KAFKA_TRACE_SMS_GUIDELINES](KAFKA_TRACE_SMS_GUIDELINES.md) |
| **开放平台** | 入站签名网关 | `openapi.New(app, cfg)` | `api.GET/POST` 注册式 | [OPENAPI_GUIDELINES](OPENAPI_GUIDELINES.md) |
| **第三方调用** | 出站签名客户端 | `thirdparty.NewClient` | `Get/Post` 自动签名 | [OPENAPI_GUIDELINES](OPENAPI_GUIDELINES.md) |
| **熔断器** | 雪崩保护 | `circuit.New` + `Config.Breaker` | `Execute/State` | [OPENAPI_GUIDELINES](OPENAPI_GUIDELINES.md) |
| **短信** | 阿里云 SMS | `sms.NewClient` | `Send/IsSuccess` | [KAFKA_TRACE_SMS_GUIDELINES](KAFKA_TRACE_SMS_GUIDELINES.md) |
| **事件总线** | 业务解耦 | `eventbus.New` | `Subscribe/Publish` | —(见包注释) |
| **SSE/WebSocket** | 实时推送 | `framework/push/*` | Hub 广播/心跳 | —(见包注释) |
| **Cron** | 定时任务 | `builder.InitCronJob` | `Register(name, spec, fn)` 单例防重入 | —(见包注释) |
| **邮件** | SMTP 发送 | `mail.NewSender` | TLS/附件 | —(见包注释) |

## 四、可观测与运维

| 模块 | 定位 | 启用/接入 | 关键 API | 详细文档 |
|---|---|---|---|---|
| **资源监控** | 服务器指标 + 页面 | `monitor.Register(app, "/monitor", cfg)` | `/monitor/api/stats` | [MONITOR_GUIDELINES](MONITOR_GUIDELINES.md) |
| **监控告警** | 水位 webhook | `alert.NewWatcher` | CPU/内存/磁盘规则 | [ALERT_GUIDELINES](ALERT_GUIDELINES.md) |
| **链路追踪** | OpenTelemetry | `trace.Init` | `Span/TraceID/WithAttribute` | [KAFKA_TRACE_SMS_GUIDELINES](KAFKA_TRACE_SMS_GUIDELINES.md) |
| **Admin 服务** | pprof/metrics/日志级别 | `builder.EnableAdmin` | `:6060` 端口 | [SECURITY_GUIDELINES](SECURITY_GUIDELINES.md) |
| **低代码生成** | Web 化代码生成 | `gencode.Register(app, "/gencode", cfg)` | 表管理/字段编辑/一键生成 | [GENCODE_GUIDELINES](GENCODE_GUIDELINES.md) |

## 五、工具链

| 工具 | 定位 | 用法 | 详细文档 |
|---|---|---|---|
| **gbx CLI** | 项目生成器 | `gbx new -name demo [-style code\|config\|gen]` | README 快速开始 |
| **util 工具集** | 常用函数 | `util.CopyProperties/DeepCopy/MD5/UUID` | [SECURITY_GUIDELINES](SECURITY_GUIDELINES.md) |
| **脱敏器** | 敏感数据 | `mask.Phone/Email/IDCard` | —(见包注释) |

## 六、规范类文档

| 文档 | 内容 |
|---|---|
| [DEVELOPMENT_STANDARDS.md](DEVELOPMENT_STANDARDS.md) | 泰山版开发规范(命名/错误码/数据库/API/并发/测试/安全,13 章) |
| [API_GUIDELINES.md](API_GUIDELINES.md) | 接口设计规范(URL/响应/分页/幂等) |
| [DATABASE_STANDARDS.md](DATABASE_STANDARDS.md) | 数据库规范(建表/索引/公共 Model/数据权限) |
| [PROJECT_ANALYSIS.md](PROJECT_ANALYSIS.md) | 项目分析与升级基线 |
| [OPTIMIZATION_ROADMAP.md](OPTIMIZATION_ROADMAP.md) | 功能优化路线图 |

## 七、Agent 开发须知

- 项目根 `AGENTS.md` 是 Agent 必读(核心约定 + 泰山版强制规范 + 工作流)
- 新模块落地步骤:实现包 → 测试 → `config.Modules` 登记 → 文档 `docs/<名>_GUIDELINES.md` → 本索引登记 → README 模块表更新 → tag 发布

## 新增模块(v1.56+ 直播刚需与企业通用)

| 模块 | 包 | 说明 |
|---|---|---|
| 幂等 | framework/idempotent | Redis 幂等防护:Check/Release/Status,防回调重试/连点/消息重投 |
| 敏感词 | framework/sensitive | DFA 过滤:Contains/Find/Replace/Validate,空格绕过防护 |
| 参数校验 | framework/validator | 统一校验:Struct/StructAll/Var,中文 label + A0400,自定义规则 |
| 在线计数 | framework/counter | 高并发计数:峰值/房间注册表(直播人数场景) |
| 分布式限流 | framework/ratelimit | Redis 令牌桶:Allow/AllowN 多实例共享配额 |
| 文件上传 | framework/upload | multipart 封装:大小/扩展名校验,iris 与 net/http 双入口 |
| 验证码 | framework/captcha | 图形验证码:PNG 生成 + TTL 存储,Store 可注入 Redis |
| Excel | framework/excel | 模板导入导出:泛型/多 sheet/分批/流式/行级错误 |
| WS 跨节点 | framework/push/ws | WithRedis 多实例房间路由,node_id 防双发 |

## 第三批模块(v1.63 异步/签名/灰度/配置中心)

| 模块 | 包 | 说明 |
|---|---|---|
| 异步任务队列 | framework/taskqueue | 延迟执行/并发控制/panic 捕获;抽奖开奖/定时下播/消息撤回 |
| 回调签名验签 | framework/live | HMAC-SHA256 签名中间件,防伪造 SRS 回调 |
| DVR 录制 | framework/live | StartRecord/StopRecord 录制客户端封装 |
| RBAC 权限 | framework/rbac | 角色权限判定:Enforcer/RequirePermission/RequireRole,带缓存 | —(见包注释) |

| 灰度路由 | framework/grayscale | 按比例/按用户稳定分流新旧版本;A/B 测试 |
| 配置中心 | framework/configcenter | Nacos 风格 HTTP 客户端:Fetch 拉取 + Watch 监听变更 |

