// Package ws 提供 WebSocket 实时双向通信能力：
// 连接管理（Hub）、广播、业务消息回调与心跳保活。
// 标准 http.Handler 实现，可配合 iris.FromStd 或直接用于 net/http。
package ws

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// 默认写队列容量与心跳间隔。
const (
	writeQueueSize       = 64
	defaultPingInterval  = 30 * time.Second
	writeWait            = 10 * time.Second
	pongWait             = 60 * time.Second
	maxMessageSize int64 = 4096
)

// Client 表示一个 WebSocket 连接。
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	id   string
}

// ID 返回客户端唯一标识。
func (c *Client) ID() string {
	return c.id
}

// Send 向该客户端发送文本消息（非阻塞，队列满时丢弃）。
func (c *Client) Send(data []byte) {
	if c == nil || c.closed() {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

// closed 判断连接是否已关闭。
func (c *Client) closed() bool {
	return c.hub.isClosed(c)
}

// Hub 管理全部 WebSocket 客户端。
// 消息处理通过 onMessage 回调注入；广播用 Broadcast。
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	onMessage  func(client *Client, data []byte)
	upgrader   websocket.Upgrader
	closed     atomic.Bool

	mu      sync.RWMutex
	count   int
	nextID  atomic.Uint64
	pingFor time.Duration
}

// NewHub 创建 WebSocket Hub；onMessage 为收到业务消息的回调（可 nil）。
func NewHub(onMessage func(client *Client, data []byte)) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, writeQueueSize),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		onMessage:  onMessage,
		pingFor:    defaultPingInterval,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}
}

// WithPingInterval 设置心跳间隔。
func (h *Hub) WithPingInterval(interval time.Duration) *Hub {
	if interval > 0 {
		h.pingFor = interval
	}
	return h
}

// Count 返回当前在线连接数。
func (h *Hub) Count() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}

// Run 启动 Hub 事件循环（注册/注销/广播），ctx 取消时关闭全部连接。
// 应在 Handle 之前启动。
func (h *Hub) Run(ctx context.Context) {
	go func() {
		<-ctx.Done()
		h.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.count = len(h.clients)
			h.mu.Unlock()
		case client := <-h.unregister:
			h.removeClient(client)
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				client.Send(message)
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 向全部连接广播文本消息。
func (h *Hub) Broadcast(data []byte) {
	if h == nil {
		return
	}
	select {
	case h.broadcast <- data:
	default:
	}
}

// Close 关闭全部连接并停止事件循环（幂等）。
func (h *Hub) Close() {
	if h == nil {
		return
	}
	if h.closed.CompareAndSwap(false, true) {
		h.mu.Lock()
		for client := range h.clients {
			_ = client.conn.Close()
		}
		h.clients = make(map[*Client]bool)
		h.count = 0
		h.mu.Unlock()
	}
}

// Handle 是标准 WebSocket 升级处理器。
// 用法：http.Handle("/ws", hub.Handle) 或 app.Get("/ws", iris.FromStd(hub.Handle))
func (h *Hub) Handle(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.Error(w, "websocket hub is nil", http.StatusInternalServerError)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, writeQueueSize),
		id:   "ws-" + strconv.FormatUint(h.nextID.Add(1), 10),
	}
	h.register <- client

	go h.writePump(client)
	h.readPump(client)
}

// readPump 读取客户端消息；连接异常或关闭时注销客户端。
func (h *Hub) readPump(client *Client) {
	defer func() {
		h.unregister <- client
		_ = client.conn.Close()
	}()
	client.conn.SetReadLimit(maxMessageSize)
	_ = client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error {
		return client.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			return
		}
		if h.onMessage != nil {
			h.onMessage(client, message)
		}
	}
}

// writePump 写队列消费与心跳保活。
func (h *Hub) writePump(client *Client) {
	ticker := time.NewTicker(h.pingFor)
	defer func() {
		ticker.Stop()
		_ = client.conn.Close()
	}()
	for {
		select {
		case message, ok := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// removeClient 从注册表移除客户端并关闭发送队列。
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
		h.count = len(h.clients)
	}
}

// isClosed 判断客户端是否仍注册在 Hub 中。
func (h *Hub) isClosed(client *Client) bool {
	if client == nil {
		return true
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[client]
	return !ok
}

