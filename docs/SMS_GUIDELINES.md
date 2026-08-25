# 短信服务指南(SMS_GUIDELINES)

`framework/sms` 提供短信服务集成,对标**阿里云短信 SendSms API**:
零第三方依赖,自实现阿里云 RPC 签名(HMAC-SHA1),支持单发/批量、模板变量、回执流水号。

## 一、快速使用

```go
import "github.com/Connorig/go-blackbox/framework/sms"

// ① 创建客户端(阿里云 AccessKey)
client, err := sms.NewClient(sms.Config{
    AccessKeyID:     "LTAI5tXXXX",
    AccessKeySecret: "your-secret",
    SignName:        "公司名",          // 短信签名(阿里云控制台申请)
    TemplateCode:    "SMS_123456789", // 模板 Code(阿里云控制台创建)
})
if err != nil {
    log.Fatalf("init sms: %v", err)
}
sms.SetGlobal(client) // 全局便捷入口(可选)

// ② 发送(模板变量)
response, err := client.Send(ctx, sms.SendRequest{
    PhoneNumbers:  "13800138000",
    TemplateParam: map[string]string{"code": "123456"}, // 模板变量 {"code":...}
    OutId:         "order-1001",                        // 外部流水号,回调定位用
})
if err != nil {
    return err // 网络/签名错误
}
if !response.IsSuccess() {
    return fmt.Errorf("sms failed: %s %s", response.Code, response.Message)
}
```

## 二、API

| API | 说明 |
| --- | --- |
| `NewClient(config) (*Client, error)` | 创建客户端;校验 AccessKey 必填 |
| `Send(ctx, request) (*SendResponse, error)` | 发送;自动 RPC 签名 + 防重放(Nonce/时间戳) |
| `SendResponse.IsSuccess()` | Code == "OK" |
| `SetGlobal(client)` / `Get()` | 全局便捷入口(或 `appbox.SMS()`) |

### Config

| 字段 | 说明 |
| --- | --- |
| `AccessKeyID` / `AccessKeySecret` | 阿里云密钥(Secret 禁止写日志) |
| `SignName` | 短信签名(默认签名) |
| `TemplateCode` | 模板 Code(默认模板) |
| `Endpoint` | 服务地址,默认 `https://dysmsapi.aliyuncs.com` |
| `Timeout` | 请求超时,默认 10s |

### SendRequest

| 字段 | 说明 |
| --- | --- |
| `PhoneNumbers` | 接收号码;单发一个,批量逗号分隔(最多 1000) |
| `SignName` / `TemplateCode` | 空时回退 Config 默认值 |
| `TemplateParam` | 模板变量 `{"code":"123456"}`;无变量可 nil |
| `OutId` | 外部流水号(业务侧对账/回调定位) |

## 三、与通知中心组合(推荐)

```go
import "github.com/Connorig/go-blackbox/framework/notify"

manager := notify.NewManager()
manager.Register(notify.SMSAdapter(smsClient)) // 短信渠道适配器

notify.RegisterTemplate("verify-code", "您的验证码是 {{code}},{{expire}} 分钟内有效")
errs := manager.SendAll(ctx, "13800138000", notify.Content{
    Template: "verify-code",
    Params:   map[string]interface{}{"code": "123456", "expire": 5},
}, "sms", "mail")
```

## 四、频控(防轰炸)

```go
limiter := notify.NewRateLimiter(time.Minute, 3) // 同一号码 1 分钟最多 3 条
if err := limiter.AllowSend(ctx, "sms", "13800138000", content); err != nil {
    return webiris.Fail(ctx, 429, apperr.CodeTooManyRequests, "发送过于频繁")
}
```

## 五、注意事项

- 短信签名与模板需在阿里云控制台申请/审核通过后使用;模板变量名必须与模板一致
- AccessKeySecret 禁止硬编码与写日志;建议环境变量注入(见 CONFIGURATION_GUIDELINES)
- 验证码场景:叠加频控 + 图形验证码,短信是第二道防线
- `OutId` 建议传业务流水号,配合短信服务商回执查询定位送达问题
- 接口幂等:同一请求重试可能重复计费,业务侧按 OutId 去重
