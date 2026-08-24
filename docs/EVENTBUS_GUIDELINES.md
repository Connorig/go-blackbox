# 事件总线指南(EVENTBUS_GUIDELINES)

`framework/event` 提供进程内事件总线(发布/订阅):同步与异步两种投递模式,
订阅者失败重试(指数退避),以及跨实例 Redis 桥接。
典型用途:业务模块解耦(订单创建后通知库存/通知/审计)、进程内观察者模式。

## 一、快速使用

```go
import "github.com/Connorig/go-blackbox/framework/event"

// ① 创建总线:async=false 同步(默认),true 异步
bus := event.New(false)

// ② 订阅(返回取消函数,幂等)
unsubscribe := bus.Subscribe("order.created", func(ctx context.Context, e event.Event) error {
    order := e.Data.(*model.Order)
    return stockService.Deduct(ctx, order) // 返回 error 中止后续订阅者(同步)
})
defer unsubscribe()

// ③ 发布
_ = bus.Publish(ctx, event.Event{Name: "order.created", Data: order})

// ④ 通配订阅(日志/审计横切关注点)
bus.SubscribeAll(func(ctx context.Context, e event.Event) error {
    logger.Infow("event", "name", e.Name)
    return nil
})
```

## 二、同步 vs 异步

| 模式 | 行为 | 适用 |
| --- | --- | --- |
| 同步 | 按订阅顺序执行;任一失败即中止并返回错误 | 强一致联动(库存扣减必须成功) |
| 异步 | 每订阅者独立 goroutine;失败仅记日志 | 通知/审计等可容忍延迟与丢失 |

## 三、订阅者失败重试

`SubscribeRetry` 针对依赖外部资源(DB/HTTP)的订阅者,瞬态失败自动恢复:

```go
bus.SubscribeRetry("order.paid", handler, 3, 100*time.Millisecond)
// 失败按 100ms → 200ms → 400ms 指数退避重试,最多 3 次(总尝试 4 次)
// ctx 取消立即中断退避;全部失败返回最后一次错误
```

注意:异步总线(Publish 异步投递)下重试语义不适用,退化为普通订阅。

## 四、跨实例桥接(Redis)

多实例部署时,实例间事件通过 Redis Pub/Sub 广播:

```go
// 每个实例:本地总线 + Redis 桥
bus := event.New(true)
bridge := eventbus.NewRedisBridge(redisClient, "app-events", bus)
go bridge.Start(ctx) // 订阅 Redis 频道 → 投递到本地总线
_ = bridge.Publish(ctx, event.Event{Name: "cache.invalidate", Data: "user:1"})
// 其他实例的桥接器收到后投递到各自本地总线
```

典型场景:缓存失效广播、配置变更通知、多实例任务协调。

## 五、与 safe 治理的关系

异步订阅者 panic 由 `framework/safe` 自动恢复(记录组件/堆栈,进程不崩);
业务仍应处理可预期错误(返回 error 而非 panic)。

## 六、注意事项

- 事件名统一小写点分命名(`order.created`、`user.registered`),避免与包名冲突
- 同步模式 handler 内不要做长耗时操作(阻塞发布方);长任务请用 redqueue
- 订阅 Data 类型建议用具体 struct 而非 map,便于编译期校验
- 取消订阅函数幂等,可安全 defer;总线零订阅发布不报错
