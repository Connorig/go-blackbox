package sse

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Connorig/go-blackbox/framework/event"
)

// TestManagerClientLifecycle 验证客户端注册/注销与计数。
func TestManagerClientLifecycle(t *testing.T) {
	manager := NewManager()
	client := manager.addClient("order.update", "notification")
	if manager.Count() != 1 {
		t.Fatalf("expected 1 client, got %d", manager.Count())
	}
	if !client.IsSubscribed("order.update") || client.IsSubscribed("other") {
		t.Fatal("subscription check failed")
	}
	manager.removeClient(client)
	if manager.Count() != 0 {
		t.Fatalf("expected 0 clients after remove, got %d", manager.Count())
	}
	if !client.IsClosed() {
		t.Fatal("removed client must be closed")
	}
}

// TestManagerBroadcastFilterBySubscription 验证广播只投递给订阅了事件的客户端。
func TestManagerBroadcastFilterBySubscription(t *testing.T) {
	manager := NewManager()
	subscribed := manager.addClient("event-a")
	notSubscribed := manager.addClient("event-b")

	if err := manager.Broadcast("event-a", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("broadcast failed: %v", err)
	}

	select {
	case message := <-subscribed.send:
		if !strings.Contains(message, "event: event-a") || !strings.Contains(message, `"k":"v"`) {
			t.Fatalf("unexpected message: %s", message)
		}
	case <-time.After(time.Second):
		t.Fatal("subscribed client must receive broadcast")
	}
	select {
	case <-notSubscribed.send:
		t.Fatal("unsubscribed client must not receive broadcast")
	default:
	}
}

// TestManagerSendToUnknownClient 验证向不存在/未订阅客户端发送不报错。
func TestManagerSendToUnknownClient(t *testing.T) {
	manager := NewManager()
	if err := manager.SendTo("missing", "event", "data"); err != nil {
		t.Fatalf("send to unknown client must be no-op: %v", err)
	}
	client := manager.addClient("event")
	if err := manager.SendTo(client.id, "other-event", "data"); err != nil {
		t.Fatalf("send to unsubscribed client must be no-op: %v", err)
	}
}

// TestManagerClose 验证 Close 关闭全部客户端且可重复调用。
func TestManagerClose(t *testing.T) {
	manager := NewManager()
	client := manager.addClient("event")
	if err := manager.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if !client.IsClosed() {
		t.Fatal("client must be closed after manager close")
	}
	if manager.Count() != 0 {
		t.Fatalf("expected 0 clients after close, got %d", manager.Count())
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("second close must be idempotent: %v", err)
	}
}

// TestBridgeEventBus 验证 eventbus 事件桥接到 SSE 广播。
func TestBridgeEventBus(t *testing.T) {
	manager := NewManager()
	bus := eventbus.New(false)
	bridgeCtx, cancel := context.WithCancel(context.Background())
	unbridge := manager.BridgeEventBus(bridgeCtx, bus)
	defer unbridge()

	client := manager.addClient("order.created")
	if err := bus.Publish(context.Background(), eventbus.Event{
		Name: "order.created",
		Data: map[string]int{"orderId": 1001},
	}); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case message := <-client.send:
		if !strings.Contains(message, "event: order.created") || !strings.Contains(message, "1001") {
			t.Fatalf("unexpected bridged message: %s", message)
		}
	case <-time.After(time.Second):
		t.Fatal("bridged event must reach sse client")
	}

	cancel()
	client2 := manager.addClient("order.created")
	time.Sleep(50 * time.Millisecond)
	if err := bus.Publish(context.Background(), eventbus.Event{Name: "order.created", Data: "x"}); err != nil {
		t.Fatalf("publish after unbridge failed: %v", err)
	}
	select {
	case <-client2.send:
		t.Fatal("event must not be bridged after cancel")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestConcurrentBroadcast 验证并发广播安全。
func TestConcurrentBroadcast(t *testing.T) {
	manager := NewManager()
	client := manager.addClient("event")
	var waitGroup sync.WaitGroup
	for i := 0; i < 20; i++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			_ = manager.Broadcast("event", index)
		}(i)
	}
	waitGroup.Wait()
	if client.IsClosed() {
		t.Fatal("client must stay open after concurrent broadcast")
	}
}

// TestSSEStreamRawTCP 用原始 TCP 连接验证 SSE 数据到达。
func TestSSEStreamRawTCP(t *testing.T) {
	manager := NewManager()
	server := httptest.NewServer(manager.Handler("message"))
	t.Cleanup(server.Close)

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := fmt.Fprintf(conn, "GET /events HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for manager.Count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := manager.Broadcast("message", map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("broadcast failed: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 4096)
	read, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	content := string(buffer[:read])
	if !strings.Contains(content, "event: message") {
		t.Fatalf("unexpected raw content: %q", content)
	}
}