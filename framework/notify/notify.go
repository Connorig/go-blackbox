// Package notify 提供统一通知中心:多渠道(短信/邮件/站内信等)发送抽象。
// 业务只依赖统一入口,渠道通过 Register 插拔;与 framework/sms、framework/mail 组合使用。
package notify

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Sender 是发送渠道抽象(短信/邮件/钉钉/企业微信等)。
// 实现必须线程安全、支持 Context 取消。
type Sender interface {
	// Channel 返回渠道唯一标识(如 "sms"、"email")。
	Channel() string
	// Send 发送单条通知;失败返回错误(不得包含敏感信息)。
	Send(ctx context.Context, target string, content Content) error
}

// Content 是通知内容。
// Template 为渠道模板 ID(如短信签名模板、邮件模板),Body 为直接内容。
// 两者至少提供一个;实现按渠道能力选择模板渲染或直发。
type Content struct {
	Title    string            // 标题(邮件/站内信)
	Body     string            // 正文内容
	Template string            // 渠道模板 ID
	Params   map[string]string // 模板参数
}

// Manager 是多渠道通知管理器。
type Manager struct {
	mu      sync.RWMutex
	senders map[string]Sender
}

// NewManager 创建通知管理器。
func NewManager() *Manager {
	return &Manager{senders: make(map[string]Sender)}
}

// Register 注册发送渠道;同渠道重复注册返回错误。
func (m *Manager) Register(sender Sender) error {
	if m == nil {
		return errors.New("notify: manager is nil")
	}
	if sender == nil {
		return errors.New("notify: sender is nil")
	}
	channel := sender.Channel()
	if channel == "" {
		return errors.New("notify: sender channel is empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.senders[channel]; exists {
		return fmt.Errorf("notify: sender channel %q already registered", channel)
	}
	m.senders[channel] = sender
	return nil
}

// Sender 按渠道查找发送器。
func (m *Manager) Sender(channel string) (Sender, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	sender, ok := m.senders[channel]
	return sender, ok
}

// Channels 返回已注册渠道列表(排序稳定)。
func (m *Manager) Channels() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	channels := make([]string, 0, len(m.senders))
	for channel := range m.senders {
		channels = append(channels, channel)
	}
	sort.Strings(channels)
	return channels
}

// Send 通过指定渠道同步发送;渠道未注册或发送失败返回错误。
func (m *Manager) Send(ctx context.Context, channel, target string, content Content) error {
	sender, ok := m.Sender(channel)
	if !ok {
		return fmt.Errorf("notify: sender channel %q not registered", channel)
	}
	if target == "" {
		return errors.New("notify: target is empty")
	}
	if content.Template == "" && content.Body == "" {
		return errors.New("notify: content must provide template or body")
	}
	if err := sender.Send(ctx, target, content); err != nil {
		return fmt.Errorf("notify: send via %q: %w", channel, err)
	}
	return nil
}

// SendAll 并发发送多个渠道;任一失败都会返回聚合错误(成功渠道不受影响)。
func (m *Manager) SendAll(ctx context.Context, target string, content Content, channels ...string) error {
	if m == nil {
		return errors.New("notify: manager is nil")
	}
	if len(channels) == 0 {
		channels = m.Channels()
	}
	var waitGroup sync.WaitGroup
	errCh := make(chan error, len(channels))
	for _, channel := range channels {
		waitGroup.Add(1)
		go func(ch string) {
			defer waitGroup.Done()
			if err := m.Send(ctx, ch, target, content); err != nil {
				errCh <- err
			}
		}(channel)
	}
	waitGroup.Wait()
	close(errCh)
	var sendErrors []error
	for err := range errCh {
		sendErrors = append(sendErrors, err)
	}
	return errors.Join(sendErrors...)
}
