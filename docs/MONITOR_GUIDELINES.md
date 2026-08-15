# 服务器资源监控指南(MONITOR_GUIDELINES)

`framework/monitor` 提供部署后服务器资源监控(对标阿里云 ECS 控制台):
内存/CPU/磁盘/负载实时采集 + 内置 HTML 监控页面 + JSON 数据接口,接口自带身份校验与限流。

## 一、接入(一行注册路由)

```go
builder.EnableWeb(appbox.TimeFormat, ":8080", "info", func(app *iris.Application) {
    // ① 基础安全链路(限流/超时/SQL 注入防护)
    app.Use(webiris.Limit(100, 200, nil), webiris.BodyLimit(1<<20), webiris.Timeout(10*time.Second), webiris.SQLGuard())

    // ② 注册监控(页面 /monitor + 数据接口 /monitor/api/stats)
    monitor.Register(app, "/monitor", monitor.Config{
        // 数据接口要求登录(页面白名单放行;也可配置 Auth 后页面用 token 参数访问)
        Auth: webiris.Auth(webiris.AuthConfig{Whitelist: []string{"/monitor"}}),
        // 数据接口每 IP 限流(默认 5 QPS,防接口轰炸)
        RatePerSecond: 5,
    })
})
```

启动后:
- 浏览器访问 `http://host:8080/monitor` → 监控页面(5 秒自动刷新,趋势曲线)
- 数据接口 `GET /monitor/api/stats` → JSON(需 Bearer token,受限流保护)

## 二、安全设计

| 防护 | 实现 | 说明 |
|---|---|---|
| token 身份校验 | `Config.Auth` 传入 `webiris.Auth` | 数据接口必须携带 `Authorization: Bearer <token>`,防未授权访问 |
| 接口轰炸/DoS | 内置令牌桶限流(每 IP) | 默认 5 QPS,`RatePerSecond/Burst` 可配;超限 429 `B0210` |
| 页面缓存 | `Cache-Control: no-store` | 敏感数据不落浏览器缓存 |
| 部署建议 | 仅内网/Admin 端口开放,或网关层再控制 | 监控页本身匿名(便于运维查看),生产建议限制访问来源 |

`Config.Auth` 为 nil 时数据接口仅限流不校验(仅限内网可信场景);页面 HTML 始终匿名。

## 三、数据接口返回结构

```json
{
  "hostname": "prod-01",
  "platform": "linux/amd64",
  "go_version": "go1.20.14",
  "version": "v1.13.0",
  "uptime_seconds": 864000,
  "process_uptime_seconds": 3600,
  "goroutines": 42,
  "time": 1786780000,
  "memory":  { "total": 16777216000, "used": 8388608000, "free": 8388608000, "usage_percent": 50.0 },
  "cpu":     { "usage_percent": 12.3 },
  "disk":    { "total": 107374182400, "used": 53687091200, "free": 53687091200, "usage_percent": 50.0 },
  "load":    { "load1": 0.5, "load5": 0.4, "load15": 0.3 }
}
```

业务方也可直接调用采集器:

```go
collector := monitor.NewCollector()
stats, err := collector.Stats() // 任意时刻手动采集
```

## 四、平台支持

| 指标 | Linux(生产推荐) | Windows(本地开发) |
|---|---|---|
| 内存 | /proc/meminfo(free 口径) | GlobalMemoryStatusEx |
| CPU 使用率 | /proc/stat 两次采样差值 | GetSystemTimes 采样差值 |
| 磁盘(根分区) | statfs | GetDiskFreeSpaceExW |
| 系统负载 | /proc/loadavg | 不支持(返回 0) |
| 运行时长 | /proc/uptime | GetTickCount64 |

说明:CPU 使用率基于采样间隔计算,页面首帧为预热值(0%),第二帧起显示真实均值。

## 五、修改指南

- **刷新频率**:监控页内 `setInterval(refresh, 5000)`(5 秒),改小注意限流配额
- **告警阈值**:页面 70% 黄色、90% 红色;对接告警可轮询 `/monitor/api/stats` 判断 `usage_percent`
- **挂到 Admin 端口**:`builder.EnableAdminRoutes(func(app){ monitor.Register(app, "/ops/monitor", ...) })`,独立端口更安全
- **扩展指标**(可选):在 `Collector` 上追加方法并在 `Stats` 结构扩展字段
