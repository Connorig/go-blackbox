package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// 连接状态机错误。
var (
	// ErrClosed 表示连接已关闭。
	ErrClosed = errors.New("rabbitmq: connection is closed")
	// ErrNotConnected 表示连接尚未建立或已断开。
	ErrNotConnected = errors.New("rabbitmq: not connected")
	// ErrConnectTimeout 表示重连超过最大尝试次数。
	ErrConnectTimeout = errors.New("rabbitmq: reconnect attempts exhausted")
)

// State 是连接状态机状态。
type State int

// 连接状态。
const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
	StateClosed
)

// String 返回状态可读名称。
func (s State) String() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateClosed:
		return "closed"
	default:
		return "disconnected"
	}
}

// Connection 是带状态机的 RabbitMQ 连接。
// 状态流转：disconnected → connecting → connected；断线后自动按退避重连；
// Close 后进入 closed，禁止再连接。
type Connection struct {
	dns   string
	state State

	conn *amqp.Connection

	mu                sync.RWMutex
	closed            bool
	reconnectInterval time.Duration
	maxRetries        int // 0 表示不限制

	ctx    context.Context
	cancel context.CancelFunc
}

// Option 是 Connection 的可选配置。
type Option func(*Connection)

// WithReconnect 配置重连退避间隔与最大尝试次数（maxRetries 为 0 表示不限制）。
func WithReconnect(interval time.Duration, maxRetries int) Option {
	return func(connection *Connection) {
		if interval > 0 {
			connection.reconnectInterval = interval
		}
		connection.maxRetries = maxRetries
	}
}

// Dial 建立 RabbitMQ 连接并启动自动重连监控。
func Dial(dns string, options ...Option) (*Connection, error) {
	if strings.TrimSpace(dns) == "" {
		return nil, errors.New("rabbitmq: dns is empty")
	}
	ctx, cancel := context.WithCancel(context.Background())
	connection := &Connection{
		dns:               dns,
		state:             StateDisconnected,
		reconnectInterval: 5 * time.Second,
		ctx:               ctx,
		cancel:            cancel,
	}
	for _, option := range options {
		option(connection)
	}
	if err := connection.connect(); err != nil {
		cancel()
		return nil, err
	}
	go connection.watchReconnect()
	SetGlobal(connection)
	return connection, nil
}

// connect 执行 connecting → connected 状态流转。
func (c *Connection) connect() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	if c.state == StateConnecting || c.state == StateConnected {
		c.mu.Unlock()
		return nil
	}
	c.state = StateConnecting
	dns := c.dns
	c.mu.Unlock()

	conn, err := amqp.Dial(dns)
	if err != nil {
		c.mu.Lock()
		c.state = StateDisconnected
		c.mu.Unlock()
		return fmt.Errorf("dial rabbitmq: %w", err)
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = conn.Close()
		return ErrClosed
	}
	c.conn = conn
	c.state = StateConnected
	c.mu.Unlock()
	return nil
}

// State 返回当前连接状态。
func (c *Connection) State() State {
	if c == nil {
		return StateDisconnected
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// IsClosed 判断连接是否已关闭。
func (c *Connection) IsClosed() bool {
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// Channel 创建新的 AMQP 信道；连接不可用时返回错误。
func (c *Connection) Channel() (*amqp.Channel, error) {
	if c == nil {
		return nil, ErrNotConnected
	}
	c.mu.RLock()
	conn := c.conn
	closed := c.closed
	c.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	if conn == nil {
		return nil, ErrNotConnected
	}
	channel, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}
	return channel, nil
}

// Close 关闭连接并停止自动重连（幂等）。
func (c *Connection) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.state = StateClosed
	conn := c.conn
	c.conn = nil
	c.cancel()
	c.mu.Unlock()

	if conn != nil {
		if err := conn.Close(); err != nil {
			return fmt.Errorf("close rabbitmq connection: %w", err)
		}
	}
	return nil
}

// watchReconnect 监听连接异常关闭并按退避自动重连。
func (c *Connection) watchReconnect() {
	for {
		c.mu.RLock()
		conn := c.conn
		closed := c.closed
		c.mu.RUnlock()
		if closed || conn == nil {
			return
		}

		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		select {
		case <-c.ctx.Done():
			return
		case closeErr := <-notifyClose:
			if closeErr == nil {
				return // 主动关闭
			}
		}

		// 异常断开：按线性退避重连，达到上限后停止
		attempts := 0
		for {
			c.mu.RLock()
			closed = c.closed
			c.mu.RUnlock()
			if closed {
				return
			}
			if c.maxRetries > 0 && attempts >= c.maxRetries {
				c.mu.Lock()
				if c.state != StateClosed {
					c.state = StateDisconnected
				}
				c.mu.Unlock()
				return
			}
			attempts++
			wait := c.reconnectInterval * time.Duration(attempts)
			select {
			case <-time.After(wait):
			case <-c.ctx.Done():
				return
			}
			if err := c.connect(); err == nil {
				break
			}
		}
	}
}

// ensureConnected 供 Consumer/Producer 在操作前确保连接可用。
func (c *Connection) ensureConnected(ctx context.Context) error {
	if c == nil {
		return ErrNotConnected
	}
	c.mu.RLock()
	closed := c.closed
	state := c.state
	c.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	if state == StateConnected {
		return nil
	}
	// 尝试立即重连（例如重连失败后的首次操作）
	return c.connect()
}
