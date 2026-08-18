package notify

import (
	"context"
	"errors"
	"fmt"

	mail "github.com/Connorig/go-blackbox/framework/mail"
	"github.com/Connorig/go-blackbox/framework/sms"
)

// SMSAdapter 把 framework/sms 客户端包装为 notify.Sender(渠道 "sms")。
// content.Template 覆盖默认模板 Code;content.Params 作为模板变量。
type SMSAdapter struct {
	client       *sms.Client
	signName     string
	templateCode string
}

// NewSMSAdapter 创建短信适配器。
// signName/templateCode 为默认值,单次发送可通过 content.Template 覆盖模板。
func NewSMSAdapter(client *sms.Client, signName, templateCode string) *SMSAdapter {
	return &SMSAdapter{client: client, signName: signName, templateCode: templateCode}
}

// Channel 返回渠道标识。
func (a *SMSAdapter) Channel() string { return "sms" }

// Send 发送短信;target 为接收号码。
func (a *SMSAdapter) Send(ctx context.Context, target string, content Content) error {
	if a == nil || a.client == nil {
		return errors.New("notify sms: client is nil")
	}
	templateCode := content.Template
	if templateCode == "" {
		templateCode = a.templateCode
	}
	response, err := a.client.Send(ctx, sms.SendRequest{
		PhoneNumbers:  target,
		SignName:      a.signName,
		TemplateCode:  templateCode,
		TemplateParam: content.Params,
	})
	if err != nil {
		return fmt.Errorf("notify sms: send: %w", err)
	}
	if !response.IsSuccess() {
		return fmt.Errorf("notify sms: upstream rejected: %s", response.Message)
	}
	return nil
}

// MailAdapter 把 framework/mail 客户端包装为 notify.Sender(渠道 "email")。
// content.Title 为邮件主题,content.Body 为 HTML 正文。
type MailAdapter struct {
	client *mail.Client
}

// NewMailAdapter 创建邮件适配器。
func NewMailAdapter(client *mail.Client) *MailAdapter {
	return &MailAdapter{client: client}
}

// Channel 返回渠道标识。
func (a *MailAdapter) Channel() string { return "email" }

// Send 发送邮件;target 为收件邮箱(单个)。
func (a *MailAdapter) Send(ctx context.Context, target string, content Content) error {
	if a == nil || a.client == nil {
		return errors.New("notify mail: client is nil")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("notify mail: context canceled: %w", err)
	}
	if err := a.client.SendMail([]string{target}, content.Title, content.Body, "", ""); err != nil {
		return fmt.Errorf("notify mail: send: %w", err)
	}
	return nil
}
