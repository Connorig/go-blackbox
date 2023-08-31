package cache

import (
	"context"
	"github.com/go-redis/cache/v9"
	"time"
)

type Rediser interface {
	Get(key string, value interface{}) (err error)                       // 获取key-value
	GetRedisClient() *cache.Cache                                        // 操作redis客户端
	IsExists(key string) bool                                            // 判断key是否存在
	Set(Key string, value interface{}) (err error)                       // 添加key-value
	SetTtl(key string, value interface{}, ttl time.Duration) (err error) // 设置key超时时间
}

// RedisCache 封装操作客户端
type RedisCache struct {
	ctx        context.Context
	proxy      *cache.Cache
	defaultTtl time.Duration // 默认过期时间
}

func (rc *RedisCache) Get(key string, value interface{}) (err error) {
	err = rc.proxy.Get(rc.ctx, key, value)
	return
}
func (rc *RedisCache) GetRedisClient() *cache.Cache {
	return rc.proxy
}
func (rc *RedisCache) IsExists(key string) bool {
	return rc.proxy.Exists(rc.ctx, key)
}
func (rc *RedisCache) Set(Key string, value interface{}) (err error) {
	rc.SetTtl(Key, value, 0)
	return
}

// SetTtl 设置key过期时间
func (rc *RedisCache) SetTtl(key string, value interface{}, ttl time.Duration) (err error) {
	if ttl <= 0 {
		ttl = rc.defaultTtl
	}
	item := cache.Item{
		Ctx:   rc.ctx,
		Key:   key,
		Value: value,
	}
	if ttl > 0 {
		item.TTL = ttl
	}
	err = rc.proxy.Set(&item)
	return
}
