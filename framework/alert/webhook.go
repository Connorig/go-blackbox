package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookType 机器人平台类型。
type WebhookType string

const (
	// TypeWeCom 企业微信机器人。
	TypeWeCom WebhookType = "wecom"
	// TypeDingTalk 钉钉机器人。
	TypeDingTalk WebhookType = "dingtalk"
	// TypeFeishu 飞书机器人。
	TypeFeishu WebhookType = "feishu"
	// TypeGeneric 通用 JSON webhook(自定义格式,自定义实现时使用)。
	TypeGeneric WebhookType = "generic"
)

// WebhookNotifier 通过机器人 webhook 推送告警(企业微信/钉钉/飞书)。
type WebhookNotifier struct {
	name    string
	url     string
	typ     WebhookType
	timeout time.Duration
	client  *http.Client
}

// NewWeComWebhook 企业微信机器人通知器。
// webhookURL 为机器人地址(https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx)。
func NewWeComWebhook(webhookURL string) *WebhookNotifier {
	return newWebhook("wecom", webhookURL, TypeWeCom)
}

// NewDingTalkWebhook 钉钉机器人通知器。
func NewDingTalkWebhook(webhookURL string) *WebhookNotifier {
	return newWebhook("dingtalk", webhookURL, TypeDingTalk)
}

// NewFeishuWebhook 飞书机器人通知器。
func NewFeishuWebhook(webhookURL string) *WebhookNotifier {
	return newWebhook("feishu", webhookURL, TypeFeishu)
}

func newWebhook(name, url string, typ WebhookType) *WebhookNotifier {
	return &WebhookNotifier{
		name:    name,
		url:     url,
		typ:     typ,
		timeout: 10 * time.Second,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Name 实现 Notifier。
func (w *WebhookNotifier) Name() string { return w.name }

// Notify 实现 Notifier:按平台格式发送 markdown 消息。
func (w *WebhookNotifier) Notify(ctx context.Context, message Message) error {
	if w == nil || w.url == "" {
		return fmt.Errorf("alert: %s webhook url is empty", w.name)
	}
	payload, err := w.buildPayload(message)
	if err != nil {
		return fmt.Errorf("alert: build %s payload: %w", w.name, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("alert: build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := w.client.Do(request)
	if err != nil {
		return fmt.Errorf("alert: send %s webhook: %w", w.name, err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("alert: %s webhook status %d", w.name, response.StatusCode)
	}
	return nil
}

// buildPayload 按平台组装请求体。
func (w *WebhookNotifier) buildPayload(message Message) ([]byte, error) {
	switch w.typ {
	case TypeWeCom:
		// {"msgtype":"markdown","markdown":{"content":"..."}}
		return json.Marshal(map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": message.Title + "\n" + message.Content,
			},
		})
	case TypeDingTalk:
		// {"msgtype":"markdown","markdown":{"title":"...","text":"..."}}
		return json.Marshal(map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": message.Title,
				"text":  "### " + message.Title + "\n\n" + message.Content,
			},
		})
	case TypeFeishu:
		// {"msg_type":"text","content":{"text":"..."}}(飞书 markdown 需 interactive card,text 最简)
		return json.Marshal(map[string]interface{}{
			"msg_type": "text",
			"content": map[string]string{
				"text": message.Title + "\n" + message.Content,
			},
		})
	default:
		return json.Marshal(message)
	}
}
