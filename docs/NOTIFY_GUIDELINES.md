# 通知中心指南(NOTIFY_GUIDELINES)

`framework/notify` 提供统一通知中心:多渠道发送器插拔、并发发送错误聚合、
模板注册与渲染。支持短信/邮件等渠道,业务只依赖一个入口。

## 一、快速使用

```go
import "github.com/Connorig/go-blackbox/framework/notify"

// ① 创建通知中心
manager := notify.NewManager()

// ② 注册渠道适配器(短信/邮件,内置 adapter)
manager.Register(notify.SMSAdapter(smsSender))   // framework/sms 包装
manager.Register(notify.MailAdapter(mailSender)) // framework/mail 包装

// ③ 注册模板(占位符 {{key}},支持嵌套键)
notify.RegisterTemplate("verify-code", "您的验证码是 {{code}},{{expire}} 分钟内有效")

// ④ 发送:全部渠道并发,错误聚合(单个失败不影响其他渠道)
errs := manager.SendAll(ctx, "ops", notify.Content{
    Template: "verify-code",          // 模板名
    Params:   map[string]interface{}{"code": "123456", "expire": 5},
    To:       "13800138000",          // 手机号/邮箱
})
// errs 为 []error;全成功为 nil
```

## 二、API

| API | 说明 |
| --- | --- |
| `NewManager()` | 创建通知中心 |
| `Register(sender Sender)` | 注册渠道(可多个) |
| `Send(ctx, channel, content) error` | 单渠道发送 |
| `SendAll(ctx, channel, content) []error` | 全渠道并发发送,错误聚合 |
| `RegisterTemplate(name, content)` | 注册模板(同名覆盖) |
| `Template(name) (string, bool)` | 查询模板 |
| `RenderTemplate(name, params) (string, error)` | 按模板渲染(缺失参数报错) |
| `Render(content, params) (string, error)` | 直接渲染 |

## 三、自定义渠道

```go
type WebhookSender struct{ url string }

func (s *WebhookSender) Channel() string { return "webhook" }

func (s *WebhookSender) Send(ctx context.Context, content notify.Content) error {
    text := content.Title + " " + content.Body
    if content.Template != "" { // 适配器负责渲染
        rendered, err := notify.RenderTemplate(content.Template, content.Params)
        if err != nil { return err }
        text = rendered
    }
    _, err := http.Post(s.url, "application/json", strings.NewReader(text))
    return err
}

manager.Register(&WebhookSender{url: "https://hooks.example.com/ops"})
```

## 四、模板管理

模板中心统一管理文案(避免文案散落代码):

```go
notify.RegisterTemplate("order.paid", "您的订单 {{order_no}} 已支付,金额 {{amount}} 元")
notify.RegisterTemplate("order.shipped", "您的订单 {{order_no}} 已发货")

errs := manager.SendAll(ctx, "user", notify.Content{
    Template: "order.paid",
    Params:   map[string]interface{}{"order_no": "ORD-001", "amount": 99.5},
    To:       user.Phone,
})
```


## 五、频控(防短信轰炸)

`RateLimiter` 按 key(channel:target)滑动窗口限次,防止验证码刷接口与短信轰炸:

```go
import "github.com/Connorig/go-blackbox/framework/notify"

// ① 创建频控:同一手机号 1 分钟内最多 3 条
limiter := notify.NewRateLimiter(time.Minute, 3)

// ② 发送前检查(推荐与 SendAll 组合)
if err := limiter.AllowSend(ctx, "sms", "13800138000", content); err != nil {
    return webiris.Fail(ctx, 429, apperr.CodeTooManyRequests, "发送过于频繁,请稍后再试")
}
errs := manager.SendAll(ctx, "sms", content, "sms", "mail")
```

API:

| API | 说明 |
| --- | --- |
| `NewRateLimiter(window, max)` | 窗口内每 key 限 max 次;window/max 非正数 = 关闭频控 |
| `Allow(key)` | 滑动窗口判断;key 建议 `channel:target` |
| `AllowSend(ctx, channel, target, content)` | 包装 Allow,拒绝时返回明确错误 |
| `Clean()` | 清理过期 key(可周期调用;窗口滑动后自动失效) |

注意:
- 进程内实现:多实例部署时各实例独立计数(总量 = 实例数 × max);需要全局频控请在 Sender 内接入 Redis 计数
- 验证码场景建议同时叠加图形验证码/行为校验,频控是第二道防线

## 六、注意事项

- SendAll 并发发送:某渠道失败不影响其他渠道;返回聚合错误便于排查
- 短信/邮件适配器:内容模板优先(content.Template 覆盖渠道默认模板)
- 敏感文案(验证码等)注意渠道合规与频控;验证码场景建议走短信专用接口
- 死信告警等运维通知建议用独立 channel 与通知中心组合(见 REDQUEUE_GUIDELINES)
