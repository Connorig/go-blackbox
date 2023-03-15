package cache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var ErrRedisInit = errors.New("缓存初始化失败")
var (
	once        sync.Once
	cacheClient redis.UniversalClient
)

// init  initialize
func init() {
}

// Instance get instance
func Instance() redis.UniversalClient {
	once.Do(func() {
		universalOptions := &redis.UniversalOptions{
			Addrs:       strings.Split("127.0.0.1", ","),
			Password:    "123456",
			PoolSize:    1,
			IdleTimeout: 300 * time.Second,
		}
		cacheClient = redis.NewUniversalClient(universalOptions)
	})
	return cacheClient
}

// SetCache
func SetCache(key string, value interface{}, expiration time.Duration) error {
	err := Instance().Set(context.Background(), key, value, expiration).Err()
	if err != nil {
		return err
	}
	return nil
}

// DeleteCache
func DeleteCache(key string) (int64, error) {
	return Instance().Del(context.Background(), key).Result()
}

// GetCacheString
func GetCacheString(key string) (string, error) {
	value, err := GetCacheBytes(key)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// GetCacheBytes
func GetCacheBytes(key string) ([]byte, error) {
	return Instance().Get(context.Background(), key).Bytes()
}

// GetCacheUint
func GetCacheUint(key string) (uint64, error) {
	return Instance().Get(context.Background(), key).Uint64()
}
