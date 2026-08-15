# openapi 示例:第三方对接与开放 API

展示 go-blackbox v1.10 的开放平台能力:**入站开放网关 + 出站签名客户端**。
业务项目只需要注册 handler,签名/验签/防重放/限流全部由脚手架完成。

## 运行

```bash
go run ./examples/openapi
```

启动后监听 `:9530`:
- 开放接口:`/openapi/v1/order/query`(GET)、`/openapi/v1/order/update`(POST)
- 内部接口:`/health`(不经过网关)

## 验证:生成签名请求头

本项目示例应用注册了第三方应用:

| 字段 | 值 |
|---|---|
| AppKey | `company-001` |
| AppSecret | `change-me-secret` |
| 算法 | `HMAC-SHA256` |

### 方式一:PowerShell 脚本生成签名

```powershell
# 生成 GET 请求签名头
$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$nonce = [Guid]::NewGuid().ToString("N")
$body = ""  # GET 请求体为空
$bodyHash = [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($body))).ToLower()
$stringToSign = "GET`n/openapi/v1/order/query`n$ts`n$nonce`n$bodyHash"
$secret = "change-me-secret"
$hmac = New-Object System.Security.Cryptography.HMACSHA256([Text.Encoding]::UTF8.GetBytes($secret))
$sig = [Convert]::ToHexString($hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($stringToSign))).ToLower()

# 调用
Invoke-RestMethod -Uri "http://127.0.0.1:9530/openapi/v1/order/query?order_id=100" -Headers @{
    "X-App-Key"     = "company-001"
    "X-Timestamp"   = "$ts"
    "X-Nonce"       = $nonce
    "X-Signature"   = $sig
    "X-Body-SHA256" = $bodyHash
}
```

预期响应:

```json
{"code":"00000","message":"ok","data":{"order_id":"100","status":"paid","app":"company-001"}}
```

### 方式二:POST 带请求体

```powershell
$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$nonce = [Guid]::NewGuid().ToString("N")
$body = '{"order_id":"100","status":"shipped"}'
$bodyHash = [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($body))).ToLower()
$stringToSign = "POST`n/openapi/v1/order/update`n$ts`n$nonce`n$bodyHash"
$hmac = New-Object System.Security.Cryptography.HMACSHA256([Text.Encoding]::UTF8.GetBytes("change-me-secret"))
$sig = [Convert]::ToHexString($hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($stringToSign))).ToLower()

Invoke-RestMethod -Uri "http://127.0.0.1:9530/openapi/v1/order/update" -Method Post -Body $body -ContentType "application/json" -Headers @{
    "X-App-Key"     = "company-001"
    "X-Timestamp"   = "$ts"
    "X-Nonce"       = $nonce
    "X-Signature"   = $sig
    "X-Body-SHA256" = $bodyHash
}
```

### 失败场景对照

| 场景 | 现象 |
|---|---|
| 签名错误/缺头 | `401` + `A0301` / `A0340` |
| 时间戳超过 ±5 分钟 | `401` + `A0230` |
| 重复使用同一 nonce | `400` + `A0506`(防重放) |
| 应用被禁用(registry.Set) | `401` + `A0301` |
| 超过应用限流 | `429` + `B0210` |

## 出站调用(我方调第三方)

```go
client := thirdparty.NewClient(thirdparty.Config{
    BaseURL: "https://sms.partner.com",
    Signer:  thirdparty.NewHMACSigner("our-app-key", "our-app-secret"),
    Timeout: 5 * time.Second,
})

var balance struct {
    Balance int `json:"balance"`
}
if err := client.Get(ctx, "/api/v1/balance", nil, &balance); err != nil {
    return apperr.Wrap(err, apperr.CodeThirdPartyError, "查询短信余额失败")
}
```

- 自动携带 `X-Timestamp / X-Nonce / X-Signature / X-Body-SHA256 / X-App-Key`
- 5xx 自动重试(指数退避 + 抖动),4xx 不重试
- 错误统一映射为 C 系列码(如 `C0001` 第三方错误、`C0200` 第三方超时)

## 修改指南

- **新增开放接口**:在 `main.go` 的 `api.GET/POST` 处追加一行注册即可,handler 保持纯业务
- **换密钥/禁用应用**:改 `openAPIRegistry()` 或运行时调用 `registry.Set(&openapi.App{...})` 热更新,立即生效
- **换 RSA 算法**:`Algorithm: openapi.AlgRSA` + `PublicKey: "<PEM 公钥>"`
- **生产多实例**:配置 `NonceStore: openapi.NewRedisNonceStore(redisSetNXFunc)`,防重放跨实例共享
