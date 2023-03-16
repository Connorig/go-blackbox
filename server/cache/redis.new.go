package cache

import (
	"context"
	"fmt"
	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
	"sync"
)

type RedisOptions redis.Options

var (
	once    sync.Once
	rediser Rediser
)

func CacheInit(ctx context.Context, redisOptions RedisOptions) Rediser {

	once.Do(func() {
		options := redis.Options(redisOptions)
		rdb := redis.NewClient(&options)

		ping := rdb.Ping(ctx)
		if ping != nil {
			fmt.Errorf("init redis error")
		}

		cache := cache.New(&cache.Options{
			Redis: rdb,
			//LocalCache: nil,
		})

		rediser = &redisCache{
			ctx:   ctx,
			proxy: cache,
			//defaultTtl: 0,
		}
	})

	return rediser
}
