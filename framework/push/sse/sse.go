// Package sse 提供 Server-Sent Events 实时推送能力：
// 客户端通过 HTTP 长连接订阅事件，服务端按事件名定向或广播推送。
// 配合 eventbus 可把进程内事件桥接到浏览器端。
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Connorig/go-blackbox/framework/event"
	"github.com/kataras/iris/v12"
)

// 心跳间隔与发送队列容量。
const (
	defaultHeartbeatInterval = 15 * time.Second
	sendQueueSize            = 64
)

// Client 表示一个 SSE 长连接客户端。
type Client struct {
	id     string
	events map[string]bool // 订阅的事件集合
	mu     sync.RWMutex
	send   chan string
	closed atomic.Bool
}

// ID 返回客户端唯一标识。
func (c *Client) ID() string {
	return c.id
}

// IsClosed 判断连接是否已关闭。
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}

// Subscribe 追加订阅事件。
func (c *Client) Subscribe(events ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, event := range events {
		c.events[event] = true
	}
}

// IsSubscribed 判断是否订阅了指定事件。
func (c *Client) IsSubscribed(event string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.events[event]
}

// close 标记关闭并释放发送队列。
func (c *Client) close() {
	if c.closed.CompareAndSwap(false, true) {
		close(c.send)
	}
}

// Manager 管理全部 SSE 客户端。
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
	nextID  atomic.Uint64

	heartbeat time.Duration
}

// NewManager 创建 SSE 客户端管理器。
func NewManager() *Manager {
	return &Manager{
		clients:   make(map[string]*Client),
		heartbeat: defaultHeartbeatInterval,
	}
}

// WithHeartbeat 设置心跳间隔（非正数使用默认值）。
func (m *Manager) WithHeartbeat(interval time.Duration) *Manager {
	if interval > 0 {
		m.heartbeat = interval
	}
	return m
}

// Count 返回当前在线客户端数。
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// Handle 是 Iris SSE 处理器：升级连接、按 events 订阅、循环推送直到连接断开。
// 用法：app.Get("/events", sseManager.Handle, ...) —— 注意事件参数无法通过路由传递，
// 如需指定订阅事件，使用 HandleEvents(ctx, events...)。
func (m *Manager) Handle(ctx iris.Context) {
	m.HandleEvents(ctx)
}

// Handler 返回标准库 SSE 处理器；可配合 iris.FromStd 使用，或直接用于 net/http。
// events 为该连接订阅的事件；连接断开时自动注销。
func (m *Manager) Handler(events ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		client := m.addClient(events...)
		defer m.removeClient(client)

		ticker := time.NewTicker(m.heartbeat)
		defer ticker.Stop()
		flusher, _ := w.(http.Flusher)

		for {
			select {
			case <-r.Context().Done():
				return
			case message, ok := <-client.send:
				if !ok {
					return
				}
				if _, err := w.Write([]byte(message)); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			case <-ticker.C:
				if _, err := w.Write([]byte(": ping\n\n")); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}
}

// HandleEvents 是带订阅事件列表的 Iris SSE 处理器。
func (m *Manager) HandleEvents(ctx iris.Context, events ...string) {
	m.Handler(events...)(ctx.ResponseWriter(), ctx.Request())
}

// SendTo 定向推送事件到指定客户端；客户端未订阅该事件时忽略。
func (m *Manager) SendTo(clientID, event string, data interface{}) error {
	client := m.client(clientID)
	if client == nil || client.IsClosed() {
		return nil
	}
	if !client.IsSubscribed(event) {
		return nil
	}
	payload, err := formatEvent(event, data)
	if err != nil {
		return err
	}
	select {
	case client.send <- payload:
		return nil
	default:
		return fmt.Errorf("sse: client %s send queue is full", clientID)
	}
}

// Broadcast 广播事件到所有订阅了该事件的客户端。
func (m *Manager) Broadcast(event string, data interface{}) error {
	payload, err := formatEvent(event, data)
	if err != nil {
		return err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, client := range m.clients {
		if client.IsClosed() || !client.IsSubscribed(event) {
			continue
		}
		select {
		case client.send <- payload:
		default:
			// 单客户端队列满不阻塞广播
		}
	}
	return nil
}

// Close 关闭全部客户端连接。
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, client := range m.clients {
		client.close()
	}
	m.clients = make(map[string]*Client)
	return nil
}

// BridgeEventBus 把 eventbus 事件桥接到 SSE 广播（事件名作为 SSE event 名）。
// 返回取消函数；ctx 取消或调用取消函数时自动解除桥接。
func (m *Manager) BridgeEventBus(ctx context.Context, bus *eventbus.Bus) func() {
	if bus == nil {
		return func() {}
	}
	unsubscribe := bus.SubscribeAll(func(_ context.Context, event eventbus.Event) error {
		return m.Broadcast(event.Name, event.Data)
	})
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-stop:
		}
		unsubscribe()
	}()
	return func() { close(stop) }
}

// addClient 注册客户端。
func (m *Manager) addClient(events ...string) *Client {
	client := &Client{
		id:     fmt.Sprintf("sse-%d", m.nextID.Add(1)),
		events: make(map[string]bool),
		send:   make(chan string, sendQueueSize),
	}
	client.Subscribe(events...)
	m.mu.Lock()
	m.clients[client.id] = client
	m.mu.Unlock()
	return client
}

// removeClient 注销客户端。
func (m *Manager) removeClient(client *Client) {
	if client == nil {
		return
	}
	client.close()
	m.mu.Lock()
	if m.clients[client.id] == client {
		delete(m.clients, client.id)
	}
	m.mu.Unlock()
}

// client 查找客户端。
func (m *Manager) client(id string) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[id]
}

// formatEvent 按 SSE 协议格式化事件消息。
func formatEvent(event string, data interface{}) (string, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("sse: marshal event %q: %w", event, err)
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event, payload), nil
}

