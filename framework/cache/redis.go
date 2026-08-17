package cache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
)

// Rediser 定义缓存操作接口。
type Rediser interface {
	Get(key string, value interface{}) (err error)                       // 获取key-value
	GetRedisClient() *cache.Cache                                        // 操作redis客户端
	IsExists(key string) bool                                            // 判断key是否存在
	Set(Key string, value interface{}) (err error)                       // 添加key-value
	SetTtl(key string, value interface{}, ttl time.Duration) (err error) // 设置key超时时间
}

// RedisCache 封装 go-redis 与本地缓存代理。
type RedisCache struct {
	ctx        context.Context
	rdb        *redis.Client // rdb 是底层客户端，Close/Health 使用
	proxy      *cache.Cache
	defaultTtl time.Duration // 默认过期时间，SetTtl 传入非正数时使用
}

// Get 获取 key 对应的值，反序列化到 value。
func (rc *RedisCache) Get(key string, value interface{}) (err error) {
	if rc == nil || rc.proxy == nil {
		return errors.New("get redis cache: cache proxy is nil")
	}
	return rc.proxy.Get(rc.ctx, key, value)
}

// GetRedisClient 返回底层缓存代理，用于高级操作。
func (rc *RedisCache) GetRedisClient() *cache.Cache {
	return rc.proxy
}

// RawClient 返回底层 go-redis 客户端(幂等/分布式锁/跨节点广播等高级场景)。
// 未初始化时返回 nil。
func (rc *RedisCache) RawClient() *redis.Client {
	if rc == nil {
		return nil
	}
	return rc.rdb
}

// IsExists 判断 key 是否存在。
func (rc *RedisCache) IsExists(key string) bool {
	if rc == nil || rc.proxy == nil {
		return false
	}
	return rc.proxy.Exists(rc.ctx, key)
}

// Set 使用默认 TTL 添加 key-value。
func (rc *RedisCache) Set(Key string, value interface{}) (err error) {
	return rc.SetTtl(Key, value, 0)
}

// SetTtl 设置 key 及过期时间；ttl 非正数时使用默认 TTL。
func (rc *RedisCache) SetTtl(key string, value interface{}, ttl time.Duration) (err error) {
	if rc == nil || rc.proxy == nil {
		return errors.New("set redis cache: cache proxy is nil")
	}
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
	return rc.proxy.Set(&item)
}

// SetDefaultTTL 设置默认过期时间，影响后续未显式指定 TTL 的写入。
func (rc *RedisCache) SetDefaultTTL(ttl time.Duration) {
	if rc == nil {
		return
	}
	rc.defaultTtl = ttl
}

// Health 检查 Redis 连通性。
func (rc *RedisCache) Health(ctx context.Context) error {
	if rc == nil || rc.rdb == nil {
		return errors.New("redis health: client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return rc.rdb.Ping(ctx).Err()
}

// Close 关闭底层 Redis 连接。
func (rc *RedisCache) Close() error {
	if rc == nil || rc.rdb == nil {
		return nil
	}
	return rc.rdb.Close()
}
