package cache

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/sync/singleflight"
)

// 缓存防护工具：防击穿（singleflight）、防雪崩（随机 TTL）。

var loadGroup singleflight.Group

// LoadOrStore 防击穿：多个并发请求命中同一个 key 时只执行一次 loader，
// 其余请求共享结果。loader 返回错误时不会缓存错误结果，后续请求可重试。
// 典型用法：缓存 miss 后回源数据库时包装 loader。
func LoadOrStore[T any](key string, loader func() (T, error)) (T, error) {
	var zero T
	if loader == nil {
		return zero, errors.New("load or store: loader is nil")
	}
	value, err, _ := loadGroup.Do(key, func() (interface{}, error) {
		return loader()
	})
	if err != nil {
		return zero, err
	}
	result, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf("load or store %q: unexpected result type %T", key, value)
	}
	return result, nil
}

// ForgetLoad 清除 singleflight 组内指定 key 的进行中状态（测试与手动失效用）。
func ForgetLoad(key string) {
	loadGroup.Forget(key)
}

// ttlJitterRatio 是随机 TTL 抖动比例（±20%）。
const ttlJitterRatio = 0.2

// SetTtlWithJitter 设置带随机抖动的过期时间，用于避免缓存同时过期导致的雪崩。
// 实际 TTL 在 baseTTL 的 ±20% 范围内随机；写入失败返回错误。
func (rc *RedisCache) SetTtlWithJitter(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	if rc == nil || rc.proxy == nil {
		return errors.New("set cache with jitter: cache proxy is nil")
	}
	if ctx == nil {
		return errors.New("set cache with jitter: context is nil")
	}
	if baseTTL <= 0 {
		return errors.New("set cache with jitter: base TTL must be positive")
	}
	jitter := time.Duration(float64(baseTTL) * ttlJitterRatio * (2*rand.Float64() - 1))
	ttl := baseTTL + jitter
	if ttl <= 0 {
		ttl = baseTTL
	}
	return rc.SetTtl(key, value, ttl)
}
