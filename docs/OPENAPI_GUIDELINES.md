# 开放 API 指南(OPENAPI_GUIDELINES)

go-blackbox 开放平台能力:入站开放网关(`framework/openapi`)+ 出站签名客户端(`framework/thirdparty`)。
目标:**业务项目只关注业务处理,加密/验签/防重放/限流/审计全部由脚手架接管**。

## 一、通信模式选型(HTTP vs MQ)

| 场景 | 推荐 | 原因 |
|---|---|---|
| 查询 / 强一致写 / 低频交互 | **HTTP 开放接口** | 零额外服务器、同步语义、调试简单,覆盖 80% 场景 |
| 事件通知 / 削峰 / 最终一致 | **MQ 消息**(可选) | 解耦,但需要 MQ 服务器且伙伴侧也要具备 MQ 能力 |
| 典型组合 | 查询走 HTTP,状态变更事件走 MQ | 对齐支付宝/微信开放平台模式 |

## 二、入站:开放 API 网关(framework/openapi)

### 2.1 业务接入三步

```go
// ① 启动期注册第三方应用(生产从配置/数据库加载)
registry := openapi.NewRegistryWith(&openapi.App{
    AppKey:    "company-001",
    AppSecret: "change-me-secret",   // HMAC 对称密钥
    Algorithm: openapi.AlgHMAC,      // 或 AlgRSA(非对称)
    Enabled:   true,
})

// ② 注册开放接口——与普通路由完全一致,加密由网关自动完成
api := openapi.New(app, openapi.Config{Registry: registry})
api.GET("/v1/order/query", QueryOrder)   // 实际路径 /openapi/v1/order/query
api.POST("/v1/order/update", UpdateOrder)

// ③ handler 就是纯业务函数,通过 openapi.AppKey(ctx) 读取调用方标识
func QueryOrder(ctx iris.Context) {
    webiris.OK(ctx, queryOrder(ctx.URLParam("order_id")))
}
```

### 2.2 请求签名规范(对齐行业通用)

请求头:

| 头 | 说明 |
|---|---|
| `X-App-Key` | 第三方应用标识 |
| `X-Timestamp` | Unix 秒,±5 分钟窗口(可配置) |
| `X-Nonce` | 随机串,防重放(默认保留 10 分钟,可配置) |
| `X-Body-SHA256` | 请求体 SHA256 hex(空体为空串摘要) |
| `X-Signature` | 签名值(HMAC 为 hex,RSA 为 base64) |

签名串(规范化,防篡改):

```
StringToSign = METHOD + "\n" + Path + "\n" + Timestamp + "\n" + Nonce + "\n" + BodySHA256
```

- `HMAC-SHA256`:`hex(HMAC-SHA256(AppSecret, StringToSign))`
- `RSA-SHA256`:`base64(RSA-SHA256(业务方私钥, StringToSign))`,网关用注册的 PEM 公钥验签

### 2.3 网关自动完成的安全能力

| 能力 | 说明 | 错误码 |
|---|---|---|
| 签名校验 | HMAC/RSA 双模式 | 失败 `A0340` |
| 应用校验 | AppKey 存在且启用 | `A0301` |
| 时间戳窗口 | 防过期重放 | `A0230` |
| nonce 防重放 | SETNX 语义,内存/Redis 双实现 | `A0506` |
| 每 App 限流 | 令牌桶,按应用独立配额 | `B0210` |
| 审计钩子 | `Config.OnAudit` 回调每次调用(含失败) | — |

### 2.4 与内部 JWT 认证并存

- 内部接口:`webiris.Auth`(JWT + scope),前缀 `/api/v1/*`
- 开放接口:`openapi.New`(AppKey 签名),前缀 `/openapi/*`
- 两者挂同一 Web 服务,互不干扰

### 2.5 密钥管理建议

- 应用密钥通过配置中心/环境变量注入,勿硬编码
- 密钥轮换:`registry.Set(&App{...新密钥...})` 热更新,立即生效
- 生产多实例部署时 `NonceStore` 使用 `NewRedisNonceStore`(SETNX 原语),防重放跨实例共享
- 每应用独立 `RatePerSecond/Burst` 配额,防单伙伴打爆服务

## 三、出站:第三方客户端(framework/thirdparty)

```go
client := thirdparty.NewClient(thirdparty.Config{
    BaseURL:   "https://sms.partner.com",
    Signer:    thirdparty.NewHMACSigner("app-key", "app-secret"), // 或 RSA/Bearer
    Timeout:   5 * time.Second,
    MaxRetries: 2,          // 默认 2 次,指数退避 + 抖动
})
resp, err := client.Get(ctx, "/api/v1/balance", nil, &out)  // GET 自动签名
resp, err = client.Post(ctx, "/api/v1/order", body, &out)   // POST JSON
```

| 能力 | 说明 |
|---|---|
| 签名器 | `NewHMACSigner`(对称)/ `NewRSASignerFromPEM`(非对称)/ `NewBearerSigner`(平台 token) |
| 超时重试 | 总超时 + 网络错误/5xx 重试,4xx 不重试 |
| 错误映射 | 统一映射 C 系列:`C0001` 第三方错误、`C0200` 第三方超时 |
| 请求头 | 自动携带 `X-Timestamp/X-Nonce/X-Signature/X-Body-SHA256/X-App-Key` |
| 集成 | 通过 simpleioc 注册为 bean,业务 `GetBean` 注入 |

## 四、接入清单(业务项目落地模板)

1. 注册第三方应用(DB 表或配置文件)→ `openapi.Registry`
2. 定义开放接口 handler(纯业务)→ `api.GET/POST` 注册
3. 配置 `OnAudit`(审计落库)
4. 生产启用 Redis nonce 存储
5. 定义出站客户端 → `thirdparty.NewClient` + simpleioc 注册
6. HTTPS 强制(网关部署要求)

## 三.5 熔断保护(framework/circuit,可选)

出站客户端支持熔断器,防止下游故障雪崩:

`go
breaker := circuit.New(circuit.DefaultConfig()) // 失败率 50% / 最小 10 请求 / 10s 窗口 / 10s 冷却
client := thirdparty.NewClient(thirdparty.Config{
    BaseURL: "https://sms.partner.com",
    Signer:  thirdparty.NewHMACSigner("key", "secret"),
    Breaker: breaker, // 启用熔断
})
`

行为:
- **closed → open**:窗口内失败率 >= 阈值(默认 50%)且请求数达标(默认 10)时熔断
- **open**:请求快速失败(错误码 B0200 系统容灾被触发),不发起真实请求
- **half-open → closed**:冷却期(默认 10s)后放行 1 个试探请求,成功即恢复,失败重新熔断
- **失败口径**:网络错误与 5xx 计入失败;4xx 业务错误不计(不会因下游业务报错误熔断)

状态可观测: reaker.State()(closed/open/half-open),可接入监控告警。
