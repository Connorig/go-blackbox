package openapi

import (
	"context"
	"errors"
	"sync"
	"time"
)

// NonceStore 定义 nonce 防重放存储。
// TrySet 语义与 Redis SETNX 一致:key 不存在时写入并返回 true;
// 已存在(重复请求)返回 false。ttl 过后 key 自动失效(允许同 nonce 再次使用)。
type NonceStore interface {
	TrySet(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// MemNonceStore 内存版 nonce 存储(单实例部署;多实例需换 RedisNonceStore)。
type MemNonceStore struct {
	mu    sync.Mutex
	items map[string]time.Time // key → 过期时间
}

// NewMemNonceStore 创建内存 nonce 存储。
func NewMemNonceStore() *MemNonceStore {
	return &MemNonceStore{items: make(map[string]time.Time)}
}

// TrySet 实现 NonceStore;并发安全,自动清理过期项。
func (s *MemNonceStore) TrySet(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if s == nil {
		return false, errors.New("openapi: nonce store is nil")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if expires, exists := s.items[key]; exists && expires.After(now) {
		return false, nil
	}
	s.items[key] = now.Add(ttl)
	// 简单防膨胀:超过 10 万条时清理过期项
	if len(s.items) > 100000 {
		for k, expires := range s.items {
			if !expires.After(now) {
				delete(s.items, k)
			}
		}
	}
	return true, nil
}

// redisNonceStore Redis 版 nonce 存储(多实例共享,生产推荐)。
// 使用 RedisCache 的分布式锁原语(SetNX)实现。
type redisNonceStore struct {
	setNX func(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
}

// SetNXFunc 是 Redis 原子写入的抽象(避免 openapi 包强依赖具体缓存实现)。
type SetNXFunc func(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)

// NewRedisNonceStore 创建 Redis 版 nonce 存储。
// setNX 传入缓存实现的 SETNX 能力(如 framework/cache 的 TryLock 等价原语)。
func NewRedisNonceStore(setNX SetNXFunc) NonceStore {
	return &redisNonceStore{setNX: setNX}
}

func (s *redisNonceStore) TrySet(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if s == nil || s.setNX == nil {
		return false, errors.New("openapi: redis nonce store is not configured")
	}
	return s.setNX(ctx, key, 1, ttl)
}
