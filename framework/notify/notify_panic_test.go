package notify

import (
	"context"
	"strings"
	"testing"
)

// panicSender 渠道发送时主动 panic。
type panicSender struct{}

func (panicSender) Channel() string { return "panic-channel" }

func (panicSender) Send(context.Context, string, Content) error {
	panic("sender exploded")
}

// TestSendAllPanicSender 验证渠道 panic 被捕获并转为聚合错误,进程不崩溃。
func TestSendAllPanicSender(t *testing.T) {
	manager := NewManager()
	manager.Register(panicSender{})

	err := manager.SendAll(context.Background(), "ops", Content{
		Title: "test",
		Body:  "body",
	})
	if err == nil {
		t.Fatal("panic sender must produce aggregated error")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Fatalf("error must mention panic, got: %v", err)
	}
}
