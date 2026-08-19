package configcenter

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CachedClient 是带本地缓存的配置中心客户端:
// 启动/定时刷新拉取,配置中心不可用时回退最后成功值(进程内缓存),变更时推送订阅者。
// 解决 Watch 场景下配置中心短暂不可用导致配置丢失的问题,对标 Nacos 客户端体验。
type CachedClient struct {
	client *Client

	mu      sync.RWMutex
	content string
	loaded  bool      // 是否成功加载过(未加载前 Get 会尝试拉取)
	updated time.Time // 最近成功更新时间

	subMu sync.Mutex
	subs  map[chan string]struct{}
}

// NewCachedClient 创建缓存客户端;client 为底层拉取客户端(不可为 nil)。
func NewCachedClient(client *Client) *CachedClient {
	if client == nil {
		client = &Client{} // 空客户端,Get/Refresh 返回错误
	}
	return &CachedClient{client: client, subs: make(map[chan string]struct{})}
}

// Get 返回配置内容:未加载时拉取并缓存;已加载直接返回缓存(配置中心不可用也可用旧值)。
func (c *CachedClient) Get(ctx context.Context) (string, error) {
	if c == nil || c.client == nil {
		return "", errNilClient()
	}
	c.mu.RLock()
	loaded := c.loaded
	content := c.content
	c.mu.RUnlock()
	if loaded {
		return content, nil
	}
	if err := c.Refresh(ctx); err != nil {
		return "", err
	}
	c.mu.RLock()
	content = c.content
	c.mu.RUnlock()
	return content, nil
}

// Content 返回当前缓存内容(无锁读取;可能为空字符串)。
func (c *CachedClient) Content() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.content
}

// UpdatedAt 返回最近成功更新时间(零值表示从未成功)。
func (c *CachedClient) UpdatedAt() time.Time {
	if c == nil {
		return time.Time{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.updated
}

// Loaded 返回是否成功加载过。
func (c *CachedClient) Loaded() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loaded
}

// Refresh 强制拉取并更新缓存;失败时保留旧值并返回错误。
// 内容变化时向订阅者广播(非阻塞,订阅通道缓冲 1)。
func (c *CachedClient) Refresh(ctx context.Context) error {
	if c == nil || c.client == nil {
		return errNilClient()
	}
	content, err := c.client.Fetch(ctx)
	if err != nil {
		return err
	}
	c.mu.Lock()
	changed := !c.loaded || content != c.content
	c.content = content
	c.loaded = true
	c.updated = time.Now()
	c.mu.Unlock()
	if changed {
		c.broadcast(content)
	}
	return nil
}

// Subscribe 订阅配置变更:返回的通道在内容变化时收到新内容,
// 首次订阅立即收到当前缓存值(若有);Close 后通道关闭。
func (c *CachedClient) Subscribe() <-chan string {
	channel := make(chan string, 1)
	if c == nil {
		close(channel)
		return channel
	}
	c.subMu.Lock()
	c.subs[channel] = struct{}{}
	c.subMu.Unlock()
	if content := c.Content(); content != "" {
		channel <- content
	}
	return channel
}

// Start 后台轮询刷新(阻塞):interval 内拉取配置,变化即广播;
// ctx 取消时退出并关闭所有订阅通道。
func (c *CachedClient) Start(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	// 启动即刷新一次(失败不退出,保留旧值)
	_ = c.Refresh(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.Close()
			return nil
		case <-ticker.C:
			_ = c.Refresh(ctx) // 网络抖动跳过,下轮重试
		}
	}
}

// Close 关闭所有订阅通道。
func (c *CachedClient) Close() {
	if c == nil {
		return
	}
	c.subMu.Lock()
	defer c.subMu.Unlock()
	for channel := range c.subs {
		close(channel)
		delete(c.subs, channel)
	}
}

// broadcast 向订阅者推送新内容(非阻塞:通道满则跳过该订阅者)。
func (c *CachedClient) broadcast(content string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	for channel := range c.subs {
		select {
		case channel <- content:
		default: // 订阅者消费慢,跳过本次
		}
	}
}

func errNilClient() error {
	return errors.New("configcenter: cached client is nil")
}
