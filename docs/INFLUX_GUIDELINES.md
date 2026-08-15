# InfluxDB 指南(INFLUX_GUIDELINES)

`framework/influx` 提供 InfluxDB 时序数据库集成(对标 Spring Data InfluxDB 的 InfluxDBTemplate):
指标写入(点/行协议)+ Flux 查询 + 桶管理 + 原生客户端暴露。支持 InfluxDB 2.x
(及 1.8+ 兼容模式:ServerURL 指向 8086 根地址 + 1.x token)。

## 接入

```go
client, err := influx.NewClient(influx.Config{
    ServerURL: "http://127.0.0.1:8086",
    Token:     "***",            // InfluxDB 2.x Token
    Org:       "company",        // 组织
    Bucket:    "metrics",        // 默认桶(可空,操作时显式指定)
    Timeout:   10 * time.Second, // 连接失败启动即报错
})
gbxioc.RegisterInstance(client)
```

## 写入

```go
// ① 便捷写入(measurement + tags + fields + 时间戳)
client.Write(ctx, "metrics", "temperature",
    map[string]interface{}{"sensor": "s1", "room": "r1"},   // tags(自动转 string)
    map[string]interface{}{"value": 26.5},                  // fields
    time.Now())                                             // 零值 = 当前时间

// ② 行协议(批量/兼容已有脚本)
client.WriteRaw(ctx, "metrics",
    "temperature,sensor=s2,room=r1 value=25.5\ncpu,host=h1 usage=0.42")

// ③ 预构造点(高级字段类型)
point := write.NewPoint("cpu_usage",
    map[string]string{"host": "h1"},
    map[string]interface{}{"value": 42.5, "cores": 8},
    time.Now())
client.WritePoint(ctx, "metrics", point)
```

## 查询(Flux)

```go
flux := `from(bucket:"metrics")
  |> range(start: -1h)
  |> filter(fn: (r) => r._measurement == "temperature")
  |> mean()`

// 结构化结果:每行 map(含 _time/_measurement/_field/_value + 全部 tag)
records, err := client.Query(ctx, "metrics", flux)
value := records[0]["_value"]

// 原始 CSV(调试/透传)
csv, err := client.QueryRaw(ctx, flux)
```

## 桶管理

```go
client.EnsureBucket(ctx, "metrics")   // 不存在则创建(初始化用)
buckets, _ := client.Buckets(ctx)     // 列出全部桶
```

## 原生客户端

```go
raw := client.Client() // influxdb2.Client(OrganizationAPI/QueryAPI 全部原生能力)
```

## 测试

真实服务测试通过环境变量启用(未配置自动跳过):

| 环境变量 | 说明 |
|---|---|
| `GO_BLACKBOX_INFLUX_URL` | InfluxDB 地址(如 http://127.0.0.1:8086) |
| `GO_BLACKBOX_INFLUX_TOKEN` | Token(默认 test-token) |
| `GO_BLACKBOX_INFLUX_ORG` | 组织(默认 test) |

## 典型场景

- 监控指标持久化(与 framework/monitor 采集配合:时序入库 + 历史趋势查询)
- 业务埋点(订单量/延迟/错误率按时间聚合)
- IoT/设备数据流
