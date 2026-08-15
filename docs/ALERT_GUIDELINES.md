# 监控告警指南(ALERT_GUIDELINES)

`framework/alert` 提供资源水位告警:轮询采集指标,超阈值自动推送企业微信/钉钉/飞书机器人,
带连续触发确认、告警去重与恢复通知。

## 一、接入(与 monitor 组合)

```go
builder.EnableWeb(appbox.TimeFormat, ":8080", "info", func(app *iris.Application) {
    monitor.Register(app, "/monitor", monitor.Config{})   // 监控页面(原有)
})

// 告警监视器:Web 启动后运行
builder.AfterSetup(func(ctx context.Context) error {
    watcher := alert.NewWatcher(alert.Config{
        Interval:  15 * time.Second,                       // 轮询间隔
        Collector: monitor.NewCollector(),                 // 指标来源
        Notifiers: []alert.Notifier{
            alert.NewWeComWebhook("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"),
            // alert.NewDingTalkWebhook("https://oapi.dingtalk.com/robot/send?access_token=xxx"),
            // alert.NewFeishuWebhook("https://open.feishu.cn/open-apis/bot/v2/hook/xxx"),
        },
        Rules: []alert.Rule{
            alert.CPUUsage(90, 3),      // CPU > 90% 连续 3 次(15s×3=45s)告警
            alert.MemoryUsage(85, 3),   // 内存 > 85% 连续 3 次
            alert.DiskUsage(85, 3),     // 磁盘 > 85% 连续 3 次
        },
    })
    go watcher.Start(ctx)
    return nil
})
```

## 二、告警语义

| 语义 | 说明 |
|---|---|
| 连续触发 | 同一规则连续 N 次采样超阈值才告警(防瞬时抖动) |
| 去重 | 告警状态持续期间不重复推送,恢复后才可再次告警 |
| 恢复通知 | 连续 N 次低于阈值后推送恢复消息(level=recover) |
| 多规则 | CPU/内存/磁盘规则独立计数、独立告警 |

推送消息示例(企业微信 markdown):

```
告警:cpu 使用率超阈值
指标: cpu 使用率
当前: 95.0% (阈值 90.0%)
```

## 三、自定义规则与通知渠道

```go
// 自定义规则(任意指标)
alert.Rule{
    Name:        "goroutines",
    Threshold:   500,
    Consecutive: 3,
    Check: func(stats *monitor.Stats) float64 {
        return float64(stats.Goroutines) // 协程数超 500 告警
    },
}

// 自定义通知器(实现 Notifier 接口:Name() + Notify(ctx, Message))
type MyNotifier struct{ ... }
func (n *MyNotifier) Name() string { return "sms" }
func (n *MyNotifier) Notify(ctx context.Context, m alert.Message) error {
    // 调用短信服务商 API
    return nil
}
```

## 四、平台支持

| 平台 | 构造函数 | 消息格式 |
|---|---|---|
| 企业微信 | `NewWeComWebhook(url)` | markdown(`{"msgtype":"markdown",...}`) |
| 钉钉 | `NewDingTalkWebhook(url)` | markdown(标题+正文) |
| 飞书 | `NewFeishuWebhook(url)` | text(最简兼容) |

机器人地址在各平台群聊中添加「自定义机器人」获取;生产建议把 webhook 地址放入配置中心,勿硬编码。
