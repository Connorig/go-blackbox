package ws

import (
	"context"
	"sync"
	"testing"
	"time"
)

// newTestClient 构造内存客户端并注册进 Hub(不经过真实 WebSocket 升级,
// 聚焦房间逻辑;readPump 的断开清理用 unregister 通道模拟)。
func newTestClient(t *testing.T, hub *Hub) *Client {
	t.Helper()
	client := &Client{hub: hub, send: make(chan []byte, 16), id: "test-client"}
	hub.register <- client
	deadline := time.Now().Add(2 * time.Second)
	for {
		hub.mu.RLock()
		_, ok := hub.clients[client]
		hub.mu.RUnlock()
		if ok {
			return client
		}
		if time.Now().After(deadline) {
			t.Fatal("client register timeout")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// readMessage 从客户端发送队列读取一条消息(300ms 超时)。
func readMessage(t *testing.T, client *Client) []byte {
	t.Helper()
	select {
	case message := <-client.send:
		return message
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no message received")
		return nil
	}
}

// assertNoMessage 断言客户端队列 150ms 内无消息(隔离验证)。
func assertNoMessage(t *testing.T, client *Client) {
	t.Helper()
	select {
	case message := <-client.send:
		t.Fatalf("unexpected message: %q", message)
	case <-time.After(150 * time.Millisecond):
	}
}

// TestRoomIsolation 双房间隔离 + 全局广播仍全员可达。
func TestRoomIsolation(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	clientA := newTestClient(t, hub)
	clientB := newTestClient(t, hub)
	hub.Join("room-a", clientA)
	hub.Join("room-b", clientB)

	hub.BroadcastRoom("room-a", []byte("for-a-only"))
	if message := readMessage(t, clientA); string(message) != "for-a-only" {
		t.Fatalf("A message = %q", message)
	}
	assertNoMessage(t, clientB)

	hub.Broadcast([]byte("global"))
	if message := readMessage(t, clientA); string(message) != "global" {
		t.Fatalf("A global = %q", message)
	}
	if message := readMessage(t, clientB); string(message) != "global" {
		t.Fatalf("B global = %q", message)
	}
}

// TestJoinLeaveIdempotent Join/Leave 幂等,重复 Join 不重复计数。
func TestJoinLeaveIdempotent(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := newTestClient(t, hub)
	hub.Join("room", client)
	hub.Join("room", client)
	if hub.CountRoom("room") != 1 {
		t.Fatalf("count after duplicate join = %d, want 1", hub.CountRoom("room"))
	}
	if rooms := hub.Rooms(); len(rooms) != 1 || rooms[0] != "room" {
		t.Fatalf("Rooms() = %v", rooms)
	}
	hub.Leave("room", client)
	hub.Leave("room", client)
	if hub.CountRoom("room") != 0 {
		t.Fatalf("count after leave = %d, want 0", hub.CountRoom("room"))
	}
	if rooms := hub.Rooms(); len(rooms) != 0 {
		t.Fatalf("Rooms() after leave = %v", rooms)
	}
}

// TestMultiRoomMembership 一个连接同时属于多个房间,各房间广播独立可达。
func TestMultiRoomMembership(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := newTestClient(t, hub)
	hub.Join("a", client)
	hub.Join("b", client)
	hub.BroadcastRoom("a", []byte("msg-a"))
	hub.BroadcastRoom("b", []byte("msg-b"))

	received := map[string]bool{}
	first := readMessage(t, client)
	second := readMessage(t, client)
	received[string(first)] = true
	received[string(second)] = true
	if !received["msg-a"] || !received["msg-b"] {
		t.Fatalf("multi-room messages missing: %v", received)
	}
}

// TestDisconnectAutoLeave 断开自动清理房间并触发 OnLeave(回调内可安全调用房间 API)。
func TestDisconnectAutoLeave(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	var mu sync.Mutex
	var leftRooms []string
	hub.OnLeave(func(h *Hub, c *Client, room string) {
		_ = h.CountRoom(room) // 回调内调用房间 API 不死锁
		mu.Lock()
		leftRooms = append(leftRooms, room)
		mu.Unlock()
	})

	client := newTestClient(t, hub)
	hub.Join("live-123", client)
	hub.Join("live-456", client)
	if hub.CountRoom("live-123") != 1 || hub.CountRoom("live-456") != 1 {
		t.Fatal("join failed")
	}
	// 模拟 readPump 异常退出后的注销(等价真实断连)
	hub.unregister <- client
	deadline := time.Now().Add(2 * time.Second)
	for (hub.CountRoom("live-123")+hub.CountRoom("live-456") > 0) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.CountRoom("live-123") != 0 || hub.CountRoom("live-456") != 0 {
		t.Fatalf("rooms not cleaned: %d %d", hub.CountRoom("live-123"), hub.CountRoom("live-456"))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(leftRooms) != 2 {
		t.Fatalf("OnLeave calls = %v, want both rooms", leftRooms)
	}
}

// TestClientMeta 业务属性设置与读取。
func TestClientMeta(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := newTestClient(t, hub)
	client.SetMeta("uid", "user-1")
	client.SetMeta("nickname", "张三")
	if client.MetaValue("uid") != "user-1" {
		t.Fatalf("MetaValue uid = %v", client.MetaValue("uid"))
	}
	meta := client.Meta()
	if len(meta) != 2 || meta["nickname"] != "张三" {
		t.Fatalf("Meta() = %v", meta)
	}
	if client.MetaValue("missing") != nil {
		t.Fatal("missing key must be nil")
	}
}

// TestMetaConcurrent 并发 Meta 读写安全。
func TestMetaConcurrent(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := newTestClient(t, hub)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			client.SetMeta("k", "v")
			_ = client.Meta()
		}()
		go func() {
			defer wg.Done()
			_ = client.MetaValue("k")
		}()
	}
	wg.Wait()
}
