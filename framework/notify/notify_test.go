package notify

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// memorySender 是测试用内存渠道。
type memorySender struct {
	channel string
	sent    atomic.Int64
	mu      sync.Mutex
	targets []string
	fail    bool // 模拟发送失败
}

// Channel 返回渠道名。
func (s *memorySender) Channel() string { return s.channel }

// Send 记录发送内容;fail 时返回错误。
func (s *memorySender) Send(_ context.Context, target string, _ Content) error {
	if s.fail {
		return errors.New("simulated failure")
	}
	s.sent.Add(1)
	s.mu.Lock()
	s.targets = append(s.targets, target)
	s.mu.Unlock()
	return nil
}

// TestRegisterAndChannels 验证注册、重复注册拒绝与渠道列表。
func TestRegisterAndChannels(t *testing.T) {
	manager := NewManager()
	if err := manager.Register(&memorySender{channel: "sms"}); err != nil {
		t.Fatalf("register sms failed: %v", err)
	}
	if err := manager.Register(&memorySender{channel: "email"}); err != nil {
		t.Fatalf("register email failed: %v", err)
	}
	if err := manager.Register(&memorySender{channel: "sms"}); err == nil {
		t.Fatal("duplicate channel must be rejected")
	}
	if err := manager.Register(nil); err == nil {
		t.Fatal("nil sender must be rejected")
	}
	channels := manager.Channels()
	if len(channels) != 2 || channels[0] != "email" || channels[1] != "sms" {
		t.Fatalf("unexpected channels: %v", channels)
	}
}

// TestSendValidation 验证发送参数校验。
func TestSendValidation(t *testing.T) {
	manager := NewManager()
	_ = manager.Register(&memorySender{channel: "sms"})

	if err := manager.Send(context.Background(), "missing", "138", Content{Body: "x"}); err == nil {
		t.Fatal("unregistered channel must return error")
	}
	if err := manager.Send(context.Background(), "sms", "", Content{Body: "x"}); err == nil {
		t.Fatal("empty target must return error")
	}
	if err := manager.Send(context.Background(), "sms", "138", Content{}); err == nil {
		t.Fatal("content without template/body must return error")
	}
	if err := manager.Send(nil, "sms", "138", Content{Body: "x"}); err != nil {
		t.Fatalf("nil ctx must be tolerated: %v", err)
	}
}

// TestSendSuccess 验证发送成功路径。
func TestSendSuccess(t *testing.T) {
	manager := NewManager()
	sms := &memorySender{channel: "sms"}
	_ = manager.Register(sms)

	if err := manager.Send(context.Background(), "sms", "13800138000", Content{Template: "tpl-login", Params: map[string]string{"code": "123456"}}); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if sms.sent.Load() != 1 {
		t.Fatalf("expected 1 send, got %d", sms.sent.Load())
	}
}

// TestSendAllAggregatesErrors 验证多渠道并发发送与错误聚合。
func TestSendAllAggregatesErrors(t *testing.T) {
	manager := NewManager()
	sms := &memorySender{channel: "sms"}
	email := &memorySender{channel: "email", fail: true}
	_ = manager.Register(sms)
	_ = manager.Register(email)

	err := manager.SendAll(context.Background(), "target-1", Content{Body: "hello"}, "sms", "email")
	if err == nil {
		t.Fatal("SendAll must aggregate the email failure")
	}
	if !strings.Contains(err.Error(), "email") {
		t.Fatalf("error must mention failed channel: %v", err)
	}
	// 成功渠道不受影响
	if sms.sent.Load() != 1 {
		t.Fatalf("sms must be sent despite email failure, got %d", sms.sent.Load())
	}
}

// TestSendAllDefaultsToAllChannels 验证未指定渠道时发送全部。
func TestSendAllDefaultsToAllChannels(t *testing.T) {
	manager := NewManager()
	sms := &memorySender{channel: "sms"}
	email := &memorySender{channel: "email"}
	_ = manager.Register(sms)
	_ = manager.Register(email)

	if err := manager.SendAll(context.Background(), "target-2", Content{Body: "broadcast"}); err != nil {
		t.Fatalf("send all failed: %v", err)
	}
	if sms.sent.Load() != 1 || email.sent.Load() != 1 {
		t.Fatalf("both channels must receive: sms=%d email=%d", sms.sent.Load(), email.sent.Load())
	}
}

// TestNilManagerSafety 验证 nil 管理器安全。
func TestNilManagerSafety(t *testing.T) {
	var manager *Manager
	if err := manager.Register(&memorySender{channel: "x"}); err == nil {
		t.Fatal("nil manager register must return error")
	}
	if manager.Channels() != nil {
		t.Fatal("nil manager channels must be nil")
	}
	if _, ok := manager.Sender("x"); ok {
		t.Fatal("nil manager sender lookup must be false")
	}
	if err := manager.Send(context.Background(), "x", "t", Content{Body: "x"}); err == nil {
		t.Fatal("nil manager send must return error")
	}
	if err := manager.SendAll(context.Background(), "t", Content{Body: "x"}); err == nil {
		t.Fatal("nil manager send all must return error")
	}
}
