# 配置中心指南(CONFIGCENTER_GUIDELINES)

`framework/configcenter` 提供配置中心客户端(Nacos 风格 HTTP API):
拉取配置内容 + 轮询监听变更(回调)。
场景:配置热更新(灰度开关/业务参数不改代码不重启)。

## 一、快速使用

```go
import "github.com/Connorig/go-blackbox/framework/configcenter"

client := configcenter.NewClient("http://127.0.0.1:8848", "order-config", "DEFAULT_GROUP")
// 多环境:命名空间隔离
client = client.WithNamespace("prod-tenant")

// ① 一次性拉取
content, err := client.Fetch(ctx)
// content 为配置文本(JSON/TOML/任意格式,业务自行解析)

// ② 监听变更(阻塞):首次拉取即回调,之后内容变化才回调
go client.Watch(ctx, 15*time.Second, func(content string) {
    // 解析并应用新配置(灰度比例/业务参数/开关)
})
```

## 二、API

| API | 说明 |
| --- | --- |
| `NewClient(baseURL, dataID, group string)` | 创建客户端;group 空默认 `DEFAULT_GROUP` |
| `WithNamespace(namespace string)` | 设置命名空间(tenant),多环境隔离 |
| `Fetch(ctx) (string, error)` | 拉取配置;404 返回明确错误(dataId/group/namespace 不匹配提示) |
| `Watch(ctx, interval, onChange)` | 轮询监听;interval 默认 30s;onChange 为 nil 报错;网络抖动跳过下轮重试 |

## 三、典型应用

### 3.1 灰度开关动态调整

```go
gray := grayscale.New(0.05)
go client.Watch(ctx, 10*time.Second, func(content string) {
    if ratio, err := strconv.ParseFloat(strings.TrimSpace(content), 64); err == nil {
        gray.Ratio = ratio
    }
})
```

### 3.2 业务参数热更新(不改代码不重启)

```go
var params struct {
    MaxOrderAmount int64  `json:"max_order_amount"`
    MaintenanceMode bool `json:"maintenance_mode"`
}
go client.Watch(ctx, 15*time.Second, func(content string) {
    if err := json.Unmarshal([]byte(content), &params); err == nil {
        // 应用新参数(注意并发读写安全)
    }
})
```

### 3.3 与本地配置合并(本地默认值 + 远端覆盖)

```go
// 本地 config.toml 提供默认值,远端配置优先
content, err := client.Fetch(ctx)
if err == nil {
    _ = json.Unmarshal([]byte(content), &params) // 远端覆盖
}
```

## 四、接入 Nacos(参考)

- 服务端:Nacos Server(8848),配置列表新建 dataId + group + 内容
- 接口路径:`GET /nacos/v1/cs/configs?dataId=&group=&tenant=`
- 轻量实现(HTTP 拉取 + 定时轮询);Nacos 长轮询与鉴权签名可在此基础上扩展
- 配置变更发布后,最长 interval 内生效

## 五、注意事项

- 配置内容敏感信息(密码/token)不建议放配置中心明文,或使用加密存储
- onChange 回调中避免长时间阻塞(解析+应用要快);应用失败保留旧值
- 多实例部署:每实例独立轮询,变更在 interval 内陆续生效(短暂不一致可接受)
- 配置内容格式建议 JSON,与结构体直接映射,校验失败保留旧值并告警
