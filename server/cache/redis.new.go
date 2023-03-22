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
	once    sync.Once
	rediser Rediser
)

func Init(ctx context.Context, redisOptions RedisOptions) Rediser {

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

		rediser = &redisCache{
			ctx:   ctx,
			proxy: cache,
			//defaultTtl: 0,
		}
	})

	return rediser
}
