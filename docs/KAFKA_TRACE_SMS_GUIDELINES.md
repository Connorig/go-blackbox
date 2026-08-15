# Kafka / OpenTelemetry / SMS 指南(KAFKA_TRACE_SMS_GUIDELINES)

## 一、Kafka(KafkaTemplate)

```go
// 生产者(对标 KafkaTemplate.send)
producer, _ := kafka.NewProducer(kafka.Config{
    Brokers: []string{"127.0.0.1:9092"},
    Topic:   "order-events",
})
producer.Send(ctx, "order-events", "order-1001", payload)      // 同 key 保序
producer.SendJSON(ctx, "order-events", "order-1001", order)    // JSON 序列化

// 消费者(对标 @KafkaListener,自动提交偏移)
consumer, _ := kafka.NewConsumer(kafka.Config{
    Brokers: []string{"127.0.0.1:9092"},
    Topic:   "order-events",
    GroupID: "order-service",
})
go func() {
    consumer.Consume(ctx, func(ctx context.Context, message kafka.Message) error {
        log.Printf("received %s: %s", message.Key, message.Value)
        return nil // nil 后自动提交;返回 error 停止消费
    })
}()

// 原生客户端
producer.Writer()  // *kafka.Writer
consumer.Reader()  // *kafka.Reader
```

## 二、OpenTelemetry(trace)

```go
// 初始化(应用启动时;Endpoint 为空则仅本地,不导出)
shutdown, err := trace.Init(ctx, trace.Config{
    ServiceName: "order-service",
    Endpoint:    "http://127.0.0.1:4318",   // OTLP HTTP(jaeger/collector)
    Environment: "prod",
    SampleRatio: 0.3,                        // 采样率
})
defer shutdown(ctx)                          // 退出时刷出 Span

// 业务埋点(自动父子关系)
err := trace.Span(ctx, "create-order", func(ctx context.Context) error {
    trace.WithAttribute(ctx, "order_id", "1001")
    return createOrder(ctx)                  // 内部再 Span 自动嵌套
})

// 日志关联 trace_id
log.Printf("order %s trace_id=%s", id, trace.TraceID(ctx))
```

集成点:`webiris.RequestID` 已透传 request_id;trace 的 TraceID 可与之并行,或按需接入采集器展示调用链。

## 三、SMS(阿里云短信,零依赖)

```go
client, _ := sms.NewClient(sms.Config{
    AccessKeyID:     "LTAI***",
    AccessKeySecret: "***",
    SignName:        "公司名",
    TemplateCode:    "SMS_123456789",
})

// 发送(对齐阿里云 SendSms API)
response, err := client.Send(ctx, sms.SendRequest{
    PhoneNumbers:  "13800138000",
    TemplateParam: map[string]string{"code": "888888"},
    OutId:         "order-1001",            // 外部流水号(可选)
})
if response.IsSuccess() {
    // 发送成功,BizID 可用于后续查询状态
} else {
    // response.Code 为阿里云错误码(如 isv.MOBILE_NUMBER_ILLEGAL)
}
```

实现说明:零第三方依赖,自实现阿里云 RPC 签名(HMAC-SHA1 + PercentEncode),接口语义完全对齐 dysmsapi.aliyuncs.com 的 SendSms;批量发送用逗号分隔多个号码(最多 1000)。

## 测试环境变量

| 变量 | 服务 |
|---|---|
| `GO_BLACKBOX_KAFKA_ADDR` | Kafka 地址 |
| `GO_BLACKBOX_SMS_ACCESS_KEY_ID/_SECRET/_PHONE/_SIGN_NAME/_TEMPLATE_CODE` | 真实短信发送(默认跳过) |
| `GO_BLACKBOX_ES_ADDR` / `GO_BLACKBOX_STORAGE_ENDPOINT` / `GO_BLACKBOX_INFLUX_URL` | ES/对象存储/InfluxDB |

未配置自动跳过,不影响 CI。
