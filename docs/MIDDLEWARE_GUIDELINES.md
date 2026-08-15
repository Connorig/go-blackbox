# 中间件指南(MIDDLEWARE_GUIDELINES)

go-blackbox 已集成的中间件全景、Template 使用方式(对标 Spring 的 RedisTemplate/MongoTemplate/RabbitTemplate),以及推荐的扩展方向。

## 一、已集成中间件全景

| 中间件 | 包 | 启用方式 | Template/封装 | 原生实例获取 |
|---|---|---|---|---|
| **Redis** | `framework/cache` | `builder.EnableCache(redisOptions)` | `RedisCache`:缓存 Get/Set + **Template 高频操作**(Incr/Decr/Del/Expire/SetNX/Hash/List)+ 分布式锁 + 防击穿/防雪崩 | `Client()` → `*redis.Client` |
| **MongoDB** | `framework/mongo` | `builder.EnableMongoDB(cfg)` | `Client`:Find/FindOne/FindTyped/InsertOne/UpdateOne/DeleteOne/Aggregate + **Template 补充**(InsertMany/Count/UpdateMany) | `Client()` → `*mongo.Client`、`Collection(name)` |
| **RabbitMQ** | `framework/mq` | 独立调用 / `InitCronJob` 无关 | Connection 状态机(自动重连)+ `Producer.Publish/PublishDelay/PublishRetry`(= RabbitTemplate.convertAndSend)+ Consumer(重试/死信) | `Channel()` → `*amqp.Channel` |
| 关系数据库 | `framework/database` | `builder.EnableDatabase` | GORM(标准 ORM,天然 template)+ 多实例 + 迁移 + 数据权限 | `datasource.Get()` → `*Instance` → `DB()` |
| 邮件 | `framework/mail` | 独立调用 | `mail.NewSender(cfg)` 发送 | — |
| Cron | `framework/cron` | `builder.InitCronJob` | 具名任务 + 单例防重入 | — |
| 事件总线 | `framework/event` | 独立调用 | 同步/异步订阅发布 | — |
| SSE / WebSocket | `framework/push/*` | 独立调用 | 实时推送 Hub | — |
| 开放平台 | `framework/openapi` | `openapi.New` | 注册式网关 | — |
| 第三方调用 | `framework/thirdparty` | `thirdparty.NewClient` | 签名客户端 + 熔断 | — |
| 配置 | `framework/config` | `LoadConfig` | 分层加载 + 热更新 | — |
| 监控/告警 | `framework/monitor|alert` | 路由/独立 | 页面 + webhook | — |

## 二、Template 使用方式(统一模式)

三个中间件遵循同一模式:**封装常用操作 + 暴露原生实例**——日常开发用 Template 方法(类型安全、带默认处理),特殊需求用原生 API。

```go
// RedisTemplate:String/Hash/List 高频操作
rc := cache.GetGlobalCache()          // 或从 gbxioc 注入
rc.Incr(ctx, "visit:count")           // 计数器
rc.SetNX(ctx, "order:1:paid", 1, time.Hour)   // 幂等标记
rc.HSet(ctx, "user:1", "name", "connor")      // 哈希
rc.LPush(ctx, "task:queue", jobID)            // FIFO 队列
raw := rc.Client()                    // 原生 *redis.Client(ZAdd/Geo/Stream...)

// MongoTemplate:批量/计数 + 原生
client, _ := mongodb.GetClient(config, ctx)
client.InsertMany(ctx, "orders", []interface{}{doc1, doc2})
count, _ := client.Count(ctx, "orders", bson.M{"status": "paid"})
raw := client.Client()                // 原生 *mongo.Client
coll := client.Collection("orders")   // 原生 *mongo.Collection

// RabbitTemplate:Publish 系列 + 原生 Channel
producer := mq.NewProducer(conn, queue)
producer.Publish(ctx, orderMsg)              // JSON 序列化发送
producer.PublishDelay(ctx, msg, 30*time.Second) // 延迟消息
ch, _ := conn.Channel()               // 原生 *amqp.Channel(声明/消费/事务)
```

## 三、推荐扩展中间件(按业务价值排序)

| 优先级 | 中间件 | 场景 | 对标 Spring |
|---|---|---|---|
| ★★★ | **ElasticSearch** | 全文检索/日志分析/商品搜索 | ElasticsearchTemplate |
| ★★★ | **Nacos / etcd** | 注册中心 + 配置中心(服务发现、动态配置) | Spring Cloud Alibaba Nacos |
| ★★★ | **对象存储(MinIO/OSS)** | 文件/图片上传下载 | OSS SDK |
| ★★☆ | **Kafka** | 高吞吐消息/流处理(替代/补充 RabbitMQ) | KafkaTemplate |
| ★★☆ | **OpenTelemetry** | 链路追踪/指标(与 monitor 互补) | Spring Cloud Sleuth |
| ★★☆ | **ClickHouse** | OLAP 分析查询 | — |
| ★☆☆ | **gRPC 客户端/服务** | 微服务内部高性能 RPC | gRPC starter |
| ★☆☆ | **分布式调度(XXL-Job 风格)** | 分布式定时任务(带控制台) | XXL-Job |
| ★☆☆ | **WebSocket 网关 + Stomp** | 消息推送协议完善 | Spring WebSocket |

**集成模式**(沿用 Template 模式):`framework/es` 提供 `EsTemplate`(Index/Search/Bulk + 原生 `Client()`);`framework/registry` 提供注册发现 + 配置拉取;`framework/storage` 提供 `StorageTemplate`(Put/Get/Delete + 原生)。与现有组件同构,业务零学习成本。

## 四、约定

- **命名**:`framework/<middleware>` + `<Xxx>Template` 接口 + 原生实例 `Client()/Channel()/DB()`
- **生命周期**:中间件实例注册进 gbxioc,关闭钩子接入 builder 优雅关闭
- **配置**:进 `config.Modules`(v1.20 自动配置体系),`enabled` 开关
