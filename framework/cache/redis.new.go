package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Connorig/go-blackbox/framework/log"
	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
)

// RedisOptions 与 go-redis 的 Options 保持等价，供配置结构直接使用。
type RedisOptions redis.Options

var (
	redisMu     sync.Mutex
	redisCacher *RedisCache
)

// Init 初始化 Redis 缓存客户端并执行连通性检查。
// 初始化失败时关闭已创建的客户端并返回错误；失败后可安全重试（不再被 sync.Once 锁死）。
// 成功后实例会写入进程级全局变量，供 GetGlobalCache 访问。
func Init(ctx context.Context, redisOptions RedisOptions) (*RedisCache, error) {
	if ctx == nil {
		return nil, errors.New("initialize Redis cache: context is nil")
	}
	options := redis.Options(redisOptions)
	rdb := redis.NewClient(&options)

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		zaplog.WithComponent("cache").Errorw("ping redis failed", "error", err)
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	cacheProxy := cache.New(&cache.Options{
		Redis:      rdb,
		LocalCache: cache.NewTinyLFU(1000, time.Minute),
	})

	instance := &RedisCache{
		ctx:        ctx,
		rdb:        rdb,
		proxy:      cacheProxy,
		defaultTtl: 0, // 默认不设置过期时间，可通过 SetDefaultTTL 修改
	}

	redisMu.Lock()
	redisCacher = instance
	redisMu.Unlock()

	zaplog.WithComponent("cache").Infow("redis cache initialized",
		"addr", options.Addr, "db", options.DB)
	return instance, nil
}

// GetGlobalCache 返回最近一次成功初始化的 Redis 缓存实例。
// 未初始化时返回 nil，调用方必须判空。
func GetGlobalCache() *RedisCache {
	redisMu.Lock()
	defer redisMu.Unlock()
	return redisCacher
}
