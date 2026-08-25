# 邮件服务指南(MAIL_GUIDELINES)

`framework/mail` 提供 SMTP 邮件发送(gomail 封装):TLS/SSL、附件、多收件人、发送别名。

## 一、快速使用

```go
import "github.com/Connorig/go-blackbox/framework/mail" // 包名 email

// ① 配置(QQ 邮箱示例)
client := mail.GetClient(&mail.MailConnConf{
    User:  "sender@qq.com",
    Pass:  "授权码",               // 邮箱服务商签发的 SMTP 授权码(非登录密码)
    Host:  "smtp.qq.com",
    Alias: "系统通知",              // 显示名称(可选)
    Port:  465,                   // 465=SSL;587=STARTTLS
})
mail.SetGlobal(client) // 全局便捷入口(可选)

// ② 发送(支持附件)
err := client.SendMail(
    []string{"receiver@example.com"}, // 收件人(可多个)
    "订单通知",                        // 主题
    "<h1>您的订单已发货</h1>",          // 正文(HTML)
    "发货单.pdf",                      // 附件文件名(无附件传 "")
    "/tmp/shipping.pdf",              // 附件路径(无附件传 "")
)
```

## 二、API

| API | 说明 |
| --- | --- |
| `GetClient(conf) *Client` | 创建客户端;配置缺省时回退全局默认(见下) |
| `SendMail(to, subject, body, fileName, filePath) error` | 发送;附件可选 |
| `SetGlobal(client)` / `Get()` | 全局便捷入口(或 `appbox.Mail()`) |

### MailConnConf

| 字段 | 说明 |
| --- | --- |
| `User` | 发送人邮箱 |
| `Pass` | SMTP 授权码(邮箱服务商签发;敏感字段,禁止写日志) |
| `Host` | SMTP 服务器(如 smtp.qq.com / smtp.163.com / smtp.gmail.com) |
| `Alias` | 发送显示名称 |
| `Port` | 465(SSL)或 587(STARTTLS);非正数默认 465 |

## 三、与通知中心组合

```go
manager := notify.NewManager()
manager.Register(notify.MailAdapter(mailClient)) // 邮件渠道适配器

// 模板渲染后群发(通知/告警)
errs := manager.SendAll(ctx, "ops@example.com", notify.Content{
    Template: "alert.disk",
    Params:   map[string]interface{}{"disk": "/", "usage": 92.5},
}, "mail", "sms")
```

## 四、常见邮箱 SMTP 配置

| 服务商 | Host | SSL 端口 | 备注 |
| --- | --- | --- | --- |
| QQ 邮箱 | smtp.qq.com | 465 | 需开启 SMTP 并生成授权码 |
| 163 邮箱 | smtp.163.com | 465 | 需开启 IMAP/SMTP 服务 |
| Gmail | smtp.gmail.com | 465 | 需应用专用密码 |
| Outlook | smtp.office365.com | 587 | STARTTLS |

## 五、注意事项

- `Pass` 使用**授权码**而非登录密码(QQ/163 等在网页端生成)
- 附件路径需真实存在;文件名建议避免中文特殊字符(部分服务商限制)
- 高频发送注意服务商每日配额;批量场景建议走 notify.SendAll 并发 + 频控
- 部署环境:容器内发送失败先检查网络出口(SMTP 25/465/587 端口放行)
- 邮件正文支持 HTML;敏感内容(密码重置链接等)建议短时效并加密参数
