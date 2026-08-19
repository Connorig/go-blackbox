# 灰度路由指南(GRAYSCALE_GUIDELINES)

`framework/grayscale` 提供灰度路由:按比例或按用户稳定分流新旧版本处理器。
场景:新接口灰度发版(5% → 20% → 50% → 100%)、A/B 测试。

## 一、快速使用

```go
import "github.com/Connorig/go-blackbox/framework/grayscale"

// ① 按比例随机分流(10% 流量走新版本)
gray := grayscale.New(0.1)

// ② 按用户稳定分流(同一用户始终命中同一版本,体验一致)
gray := grayscale.New(0.1, func(ctx iris.Context) string {
    return webiris.UserEmail(ctx) // 或 UserID 字符串
})

// ③ 注册路由:命中走新 handler,否则走旧 handler
app.Get("/api/v1/orders", gray.Route(newOrderHandler, oldOrderHandler))
```

## 二、API

| API | 说明 |
| --- | --- |
| `New(ratio float64, userKey ...func(ctx) string)` | 创建策略;ratio 0~1;userKey 可选 |
| `Hit(ctx) bool` | 判断当前请求是否命中新版本 |
| `Route(newHandler, oldHandler) iris.Handler` | 分流处理器 |

## 三、分流算法

| 模式 | 算法 | 特点 |
| --- | --- | --- |
| 按用户(推荐) | FNV-1a 哈希用户标识 % 10000 < ratio*10000 | 同一用户稳定;跨进程/重启不变 |
| 按请求随机 | `rand.Float64() < ratio` | 简单;同一用户可能反复切换 |

边界:ratio<=0 恒走旧版本;ratio>=1 恒走新版本。

## 四、灰度发版流程

```
1. 新版本 handler 与旧版本并存,灰度 5% 观察
2. 验证无误 → 20% → 50% → 100%(ratio 通过配置中心下发,见 CONFIGCENTER_GUIDELINES)
3. 全量后删除旧 handler 与灰度代码
```

## 五、与配置中心联动

灰度比例通过配置中心动态调整,无需重启:

```go
cfgClient := configcenter.NewClient("http://127.0.0.1:8848", "gray-ratio", "DEFAULT_GROUP")
gray := grayscale.New(0.05)

go cfgClient.Watch(ctx, 10*time.Second, func(content string) {
    if ratio, err := strconv.ParseFloat(strings.TrimSpace(content), 64); err == nil {
        gray.Ratio = ratio // 动态调整灰度比例
    }
})
```

## 六、注意事项

- 按用户分流请用稳定标识(用户 ID/邮箱),勿用随机生成的 session
- 灰度期间新旧版本都要记录日志(便于对比错误率),建议加 `gray` 字段标记版本
- 涉及数据库结构变更的灰度,先做兼容性改造(新旧代码可并行读写),再灰度
- A/B 测试:分流比例 50% + 结果埋点对比,与灰度发版共用同一机制
