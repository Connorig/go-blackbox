package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// testDialer 是测试用 WebSocket 客户端。
var testDialer = websocket.Dialer{}

// connect 建立测试 WebSocket 连接。
func connect(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()
	conn, _, err := testDialer.Dial(strings.Replace(serverURL, "http://", "ws://", 1), nil)
	if err != nil {
		t.Fatalf("dial websocket failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestHubEcho 验证消息回调：客户端发送消息，onMessage 收到并回显。
func TestHubEcho(t *testing.T) {
	hub := NewHub(func(client *Client, data []byte) {
		client.Send(append([]byte("echo:"), data...))
	})
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	t.Cleanup(cancel)

	server := httptest.NewServer(http.HandlerFunc(hub.Handle))
	t.Cleanup(server.Close)

	conn := connect(t, server.URL)
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(message) != "echo:hello" {
		t.Fatalf("unexpected echo: %q", message)
	}
}

// TestHubBroadcast 验证广播：第二个连接收到广播消息。
func TestHubBroadcast(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	t.Cleanup(cancel)

	server := httptest.NewServer(http.HandlerFunc(hub.Handle))
	t.Cleanup(server.Close)

	conn := connect(t, server.URL)
	// 等待注册
	deadline := time.Now().Add(3 * time.Second)
	for hub.Count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.Count() != 1 {
		t.Fatalf("expected 1 connection, got %d", hub.Count())
	}

	hub.Broadcast([]byte("broadcast-message"))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read broadcast failed: %v", err)
	}
	if string(message) != "broadcast-message" {
		t.Fatalf("unexpected broadcast: %q", message)
	}
}

// TestHubDisconnectCleanup 验证断开后客户端被注销。
func TestHubDisconnectCleanup(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	t.Cleanup(cancel)

	server := httptest.NewServer(http.HandlerFunc(hub.Handle))
	t.Cleanup(server.Close)

	conn := connect(t, server.URL)
	deadline := time.Now().Add(3 * time.Second)
	for hub.Count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	_ = conn.Close()

	deadline = time.Now().Add(3 * time.Second)
	for hub.Count() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.Count() != 0 {
		t.Fatalf("client must be unregistered after disconnect, count=%d", hub.Count())
	}
}

// TestHubCloseIdempotent 验证 Close 幂等并清空连接。
func TestHubCloseIdempotent(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(hub.Handle))
	conn := connect(t, server.URL)
	deadline := time.Now().Add(3 * time.Second)
	for hub.Count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	hub.Close()
	cancel()
	if hub.Count() != 0 {
		t.Fatalf("count must be 0 after close, got %d", hub.Count())
	}
	hub.Close() // 幂等
	_ = conn.Close()
	server.Close()
}

// TestHubConcurrentBroadcast 验证并发广播安全。
func TestHubConcurrentBroadcast(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	t.Cleanup(cancel)

	server := httptest.NewServer(http.HandlerFunc(hub.Handle))
	t.Cleanup(server.Close)
	conn := connect(t, server.URL)

	deadline := time.Now().Add(3 * time.Second)
	for hub.Count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			hub.Broadcast([]byte("m"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent broadcast timed out")
	}
	_ = conn.Close()
}

// TestHubNilSafety 验证 nil Hub 上方法调用安全。
func TestHubNilSafety(t *testing.T) {
	var hub *Hub
	hub.Broadcast([]byte("x")) // 不应 panic
	if hub.Count() != 0 {
		t.Fatal("nil hub count must be 0")
	}
	hub.Close() // 不应 panic
}
