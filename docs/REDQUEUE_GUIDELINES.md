# 可靠任务队列指南(REDQUEUE_GUIDELINES)

`framework/taskqueue/redqueue` 提供基于 Redis 的可靠任务队列:
即时任务 + 延迟任务 + 有限重试 + 死信治理 + 多实例并行消费。
适合:订单超时关单、延迟通知、异步任务、定时补偿等需要持久化与可靠性的场景。

## 一、快速使用

```go
import "github.com/Connorig/go-blackbox/framework/taskqueue/redqueue"

// ① 创建队列(用全局 Redis;keyPrefix 多队列隔离)
queue := redqueue.NewQueue(cache.RedisCache().Client(), "order-timeout")

// ② 提交任务:即时
_ = queue.Submit(ctx, []byte(`{"order_id":1001}`), 0)

// ③ 提交任务:延迟 30 分钟(订单超时关单)
_ = queue.Submit(ctx, []byte(`{"order_id":1001}`), 30*time.Minute)

// ④ 消费(常驻 goroutine,ctx 取消优雅退出)
go func() {
    _ = queue.Consume(ctx, func(ctx context.Context, payload []byte) error {
        var task struct{ OrderID int64 `json:"order_id"` }
        _ = json.Unmarshal(payload, &task)
        return service.CloseOrder(ctx, task.OrderID) // 返回 error 自动重试
    })
}()
```

## 二、重试与死信

| 配置 | 说明 |
| --- | --- |
| `WithMaxRetries(n)` | 重试上限(默认 5;0 = 无限重试) |
| `WithDeadLetterHook(fn)` | 死信回调:进死信时触发,可接 alert/notify 告警 |

行为:handler 返回错误 → 重试计数 +1 → 延迟 1s 重投 → 超限进死信队列(触发回调)。
多实例部署时每实例都会收到回调,业务侧自行去重(推荐 Redis SETNX 指纹)。

```go
queue := redqueue.NewQueue(client, "order-timeout").
    WithMaxRetries(3).
    WithDeadLetterHook(func(ctx context.Context, letter redqueue.DeadLetter) {
        logger.Errorw("task dead-lettered", "payload", string(letter.Payload),
            "retries", letter.Retries, "failed_at", letter.FailedAt)
        _ = notify.GetManager().SendAll(ctx, "ops-alert", "task dead letter", ...)
    })
```

## 三、死信治理

```go
total, _ := queue.DeadLetterCount(ctx)              // 死信数量
letters, _ := queue.DeadLetters(ctx, 0, 20)         // 倒序分页查询
_ = queue.RequeueDeadLetter(ctx, 0)                 // 重投最新一条(保留原重试计数)
```

排查流程:死信 → 查日志定位失败原因 → 修复 → RequeueDeadLetter 重投。

## 四、监控

```go
pending, _ := queue.Pending(ctx) // 即时 + 延迟待处理总数(可接入监控面板)
```

## 五、多实例说明

- 消费:BRPop 原子竞争,同一任务只会被一个实例消费
- 延迟任务:Lua 原子搬移,不会重复搬移
- 死信回调:每实例都会触发(见上,业务去重)
- 队列容量:Redis 单 key 存储,超大积压需评估(可加 keyPrefix 分片)

## 六、与进程内 taskqueue 对比

| | taskqueue | redqueue |
| --- | --- | --- |
| 存储 | 内存 | Redis(持久化) |
| 多实例 | 不支持 | 支持 |
| 延迟任务 | 不支持 | 支持(精度秒级) |
| 失败重试 | 立即 | 计数 + 延迟重投 + 死信 |
| 适用 | 单机轻量异步 | 生产可靠任务 |
