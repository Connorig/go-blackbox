// Package captcha 提供图形验证码:生成 PNG + 答案存储与校验。
// 底层基于 base64Captcha(Go 生态主流);答案默认内存存储(带 TTL 与限次),
// 可注入自定义 Store(如 Redis 分布式校验)。
// 场景:登录/注册防刷、短信发送前置验证。
package captcha

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
)

// Store 验证码答案存储接口(内存实现内置;分布式场景注入 Redis 实现)。
type Store interface {
	// Set 保存答案,ttl 过期。
	Set(id, answer string, ttl time.Duration) error
	// Get 读取并消费(一次性:校验后即失效)。
	Get(id string) (string, error)
	// Exists 判断是否存在(不消费)。
	Exists(id string) bool
}

// memoryStore 内存存储(带 TTL 清理)。
type memoryStore struct {
	mu    sync.Mutex
	items map[string]memoryItem
}

type memoryItem struct {
	answer    string
	expiresAt time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: make(map[string]memoryItem)}
}

// Set 保存答案。
func (m *memoryStore) Set(id, answer string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[id] = memoryItem{answer: answer, expiresAt: time.Now().Add(ttl)}
	return nil
}

// Get 读取并消费答案(一次性)。
func (m *memoryStore) Get(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, exists := m.items[id]
	if !exists {
		return "", errors.New("captcha not found")
	}
	delete(m.items, id)
	if time.Now().After(item.expiresAt) {
		return "", errors.New("captcha expired")
	}
	return item.answer, nil
}

// Exists 判断是否存在(不消费)。
func (m *memoryStore) Exists(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, exists := m.items[id]
	if !exists {
		return false
	}
	if time.Now().After(item.expiresAt) {
		delete(m.items, id)
		return false
	}
	return true
}

// Generator 验证码生成器。
type Generator struct {
	driver base64Captcha.Driver
	store  Store
	ttl    time.Duration
}

// NewGenerator 创建生成器(默认数字 4 位 + 内存存储 + 5 分钟 TTL)。
func NewGenerator() *Generator {
	return &Generator{
		driver: base64Captcha.NewDriverDigit(40, 120, 4, 0.7, 80),
		store:  newMemoryStore(),
		ttl:    5 * time.Minute,
	}
}

// WithStore 注入自定义存储(如 Redis 实现)。
func (g *Generator) WithStore(store Store) *Generator {
	if store != nil {
		g.store = store
	}
	return g
}

// WithTTL 设置答案有效期。
func (g *Generator) WithTTL(ttl time.Duration) *Generator {
	if ttl > 0 {
		g.ttl = ttl
	}
	return g
}

// Generate 生成验证码,返回 (id, base64 PNG 数据, 错误)。
// 前端:<img src="data:image/png;base64,<data>">。
func (g *Generator) Generate() (id, base64PNG string, err error) {
	if g == nil {
		return "", "", errors.New("captcha: generator is nil")
	}
	id, content, answer := g.driver.GenerateIdQuestionAnswer()
	item, err := g.driver.DrawCaptcha(content)
	if err != nil {
		return "", "", fmt.Errorf("captcha: draw: %w", err)
	}
	if err := g.store.Set(id, answer, g.ttl); err != nil {
		return "", "", fmt.Errorf("captcha: store: %w", err)
	}
	return id, item.EncodeB64string(), nil
}

// Verify 校验答案(一次性消费:验证失败也会销毁,防暴力破解)。
// 大小写不敏感;空白自动去除。
func (g *Generator) Verify(id, answer string) bool {
	if g == nil || g.store == nil {
		return false
	}
	stored, err := g.store.Get(id)
	if err != nil {
		return false
	}
	return normalizeAnswer(stored) == normalizeAnswer(answer)
}

// Exists 判断验证码 id 是否仍有效(不消费)。
func (g *Generator) Exists(id string) bool {
	if g == nil || g.store == nil {
		return false
	}
	return g.store.Exists(id)
}

// normalizeAnswer 归一化答案(去空白/统一小写)。
func normalizeAnswer(answer string) string {
	normalized := ""
	for _, r := range answer {
		if r == ' ' || r == '\t' || r == '\n' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			r += 32
		}
		normalized += string(r)
	}
	return normalized
}
