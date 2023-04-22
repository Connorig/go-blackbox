package cache

import (
	"context"
	"fmt"
	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
	"sync"
	"time"
)

type RedisOptions redis.Options

var (
	once        sync.Once
	redisCacher *RedisCache
)

// Init 初始化缓存配置
func Init(ctx context.Context, redisOptions RedisOptions) *RedisCache {

	once.Do(func() {
		options := redis.Options(redisOptions)
		rdb := redis.NewClient(&options)

		ping := rdb.Ping(ctx)
		if ping != nil {
			fmt.Errorf("init redis error")
		}

		cache := cache.New(&cache.Options{
			Redis:      rdb,
			LocalCache: cache.NewTinyLFU(1000, time.Minute),
		})

		redisCacher = &RedisCache{
			ctx:   ctx,
			proxy: cache,
			//defaultTtl: 0,
		}
	})

	return redisCacher
}
