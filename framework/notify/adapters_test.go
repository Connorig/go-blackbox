package notify

import (
	"context"
	"os"
	"testing"

	mail "github.com/Connorig/go-blackbox/framework/mail"
	"github.com/Connorig/go-blackbox/framework/sms"
)

// TestAdapterChannels 验证适配器渠道标识。
func TestAdapterChannels(t *testing.T) {
	if NewSMSAdapter(nil, "", "").Channel() != "sms" {
		t.Fatal("sms adapter channel must be sms")
	}
	if NewMailAdapter(nil).Channel() != "email" {
		t.Fatal("mail adapter channel must be email")
	}
}

// TestAdapterNilSafety 验证 nil 客户端安全。
func TestAdapterNilSafety(t *testing.T) {
	smsAdapter := NewSMSAdapter(nil, "", "")
	if err := smsAdapter.Send(context.Background(), "138", Content{Body: "x"}); err == nil {
		t.Fatal("sms adapter with nil client must return error")
	}
	mailAdapter := NewMailAdapter(nil)
	if err := mailAdapter.Send(context.Background(), "a@b.com", Content{Body: "x"}); err == nil {
		t.Fatal("mail adapter with nil client must return error")
	}
}

// TestMailAdapterCanceledContext 验证取消的 Context 被拒绝。
func TestMailAdapterCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	adapter := NewMailAdapter(mail.GetClient(&mail.MailConnConf{User: "u", Pass: "p", Host: "smtp.example.com"}))
	if err := adapter.Send(ctx, "a@b.com", Content{Title: "t", Body: "b"}); err == nil {
		t.Fatal("canceled context must be rejected")
	}
}

// TestAdaptersRegisterWithManager 验证适配器可注册进通知中心。
func TestAdaptersRegisterWithManager(t *testing.T) {
	manager := NewManager()
	if err := manager.Register(NewSMSAdapter(nil, "", "")); err != nil {
		t.Fatalf("register sms adapter failed: %v", err)
	}
	if err := manager.Register(NewMailAdapter(nil)); err != nil {
		t.Fatalf("register mail adapter failed: %v", err)
	}
	channels := manager.Channels()
	if len(channels) != 2 || channels[0] != "email" || channels[1] != "sms" {
		t.Fatalf("unexpected channels: %v", channels)
	}
}

// TestSMSAdapterIntegration 验证短信适配器真实发送(需阿里云凭据,默认跳过)。
func TestSMSAdapterIntegration(t *testing.T) {
	key := os.Getenv("GO_BLACKBOX_SMS_KEY")
	secret := os.Getenv("GO_BLACKBOX_SMS_SECRET")
	phone := os.Getenv("GO_BLACKBOX_SMS_PHONE")
	if key == "" || secret == "" || phone == "" {
		t.Skip("SMS integration test requires GO_BLACKBOX_SMS_KEY/SECRET/PHONE")
	}
	client, err := sms.NewClient(sms.Config{AccessKeyID: key, AccessKeySecret: secret})
	if err != nil {
		t.Fatalf("create sms client failed: %v", err)
	}
	adapter := NewSMSAdapter(client, os.Getenv("GO_BLACKBOX_SMS_SIGN"), os.Getenv("GO_BLACKBOX_SMS_TPL"))
	if err := adapter.Send(context.Background(), phone, Content{Params: map[string]string{"code": "123456"}}); err != nil {
		t.Fatalf("send sms failed: %v", err)
	}
}
