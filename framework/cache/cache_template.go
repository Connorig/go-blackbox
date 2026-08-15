package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisTemplate 高频操作层(对标 Spring Data Redis 的 RedisTemplate):
// 在 RedisCache 的缓存封装之外,提供常用数据结构操作(String/Hash/List),
// 同时暴露原生 *redis.Client 供开发者手动调用原生 API。
//
// 用法:
//
//	rc := cache.GetGlobalCache()          // 或 builder.EnableCache 后注入
//	rc.Incr(ctx, "visit:count")
//	rc.HSet(ctx, "user:1", "name", "connor")
//	// 需要原生 API 时:
//	raw := rc.Client()                     // *redis.Client
//	raw.ZAdd(ctx, "rank", redis.Z{...})
type RedisTemplate interface {
	// Client 返回底层原生 Redis 客户端(高级/原生操作入口)。
	Client() *redis.Client

	// ---- String ----
	// Incr 自增(不存在时从 0 开始);返回自增后的值。
	Incr(ctx context.Context, key string) (int64, error)
	// Decr 自减。
	Decr(ctx context.Context, key string) (int64, error)
	// GetString 获取字符串值。
	GetString(ctx context.Context, key string) (string, error)

	// ---- 通用 ----
	// Del 删除一个或多个 key;返回删除数量。
	Del(ctx context.Context, keys ...string) (int64, error)
	// Expire 设置过期时间;key 不存在返回 false。
	Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// SetNX 仅当 key 不存在时写入(分布式锁/幂等标记);成功返回 true。
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)

	// ---- Hash ----
	// HSet 设置哈希字段。
	HSet(ctx context.Context, key, field string, value interface{}) error
	// HGet 获取哈希字段。
	HGet(ctx context.Context, key, field string) (string, error)
	// HDel 删除哈希字段;返回删除数量。
	HDel(ctx context.Context, key string, fields ...string) (int64, error)
	// HGetAll 获取整个哈希。
	HGetAll(ctx context.Context, key string) (map[string]string, error)

	// ---- List(简单队列) ----
	// LPush 从头部压入;返回列表长度。
	LPush(ctx context.Context, key string, values ...interface{}) (int64, error)
	// RPop 从尾部弹出(配合 LPush 组成 FIFO 队列)。
	RPop(ctx context.Context, key string) (string, error)
}

// ---- 实现 ----

// Client 返回底层原生 Redis 客户端。
func (rc *RedisCache) Client() *redis.Client {
	if rc == nil {
		return nil
	}
	return rc.rdb
}

// Incr 自增。
func (rc *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	if err := rc.requireClient(); err != nil {
		return 0, err
	}
	return rc.rdb.Incr(ctx, key).Result()
}

// Decr 自减。
func (rc *RedisCache) Decr(ctx context.Context, key string) (int64, error) {
	if err := rc.requireClient(); err != nil {
		return 0, err
	}
	return rc.rdb.Decr(ctx, key).Result()
}

// GetString 获取字符串值;key 不存在返回 redis.Nil。
func (rc *RedisCache) GetString(ctx context.Context, key string) (string, error) {
	if err := rc.requireClient(); err != nil {
		return "", err
	}
	return rc.rdb.Get(ctx, key).Result()
}

// Del 删除一个或多个 key。
func (rc *RedisCache) Del(ctx context.Context, keys ...string) (int64, error) {
	if err := rc.requireClient(); err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, nil
	}
	return rc.rdb.Del(ctx, keys...).Result()
}

// Expire 设置过期时间。
func (rc *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if err := rc.requireClient(); err != nil {
		return false, err
	}
	return rc.rdb.Expire(ctx, key, ttl).Result()
}

// SetNX 仅当 key 不存在时写入。
func (rc *RedisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if err := rc.requireClient(); err != nil {
		return false, err
	}
	return rc.rdb.SetNX(ctx, key, value, ttl).Result()
}

// HSet 设置哈希字段。
func (rc *RedisCache) HSet(ctx context.Context, key, field string, value interface{}) error {
	if err := rc.requireClient(); err != nil {
		return err
	}
	return rc.rdb.HSet(ctx, key, field, value).Err()
}

// HGet 获取哈希字段。
func (rc *RedisCache) HGet(ctx context.Context, key, field string) (string, error) {
	if err := rc.requireClient(); err != nil {
		return "", err
	}
	return rc.rdb.HGet(ctx, key, field).Result()
}

// HDel 删除哈希字段。
func (rc *RedisCache) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	if err := rc.requireClient(); err != nil {
		return 0, err
	}
	if len(fields) == 0 {
		return 0, nil
	}
	return rc.rdb.HDel(ctx, key, fields...).Result()
}

// HGetAll 获取整个哈希。
func (rc *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if err := rc.requireClient(); err != nil {
		return nil, err
	}
	return rc.rdb.HGetAll(ctx, key).Result()
}

// LPush 从头部压入。
func (rc *RedisCache) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	if err := rc.requireClient(); err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}
	return rc.rdb.LPush(ctx, key, values...).Result()
}

// RPop 从尾部弹出。
func (rc *RedisCache) RPop(ctx context.Context, key string) (string, error) {
	if err := rc.requireClient(); err != nil {
		return "", err
	}
	return rc.rdb.RPop(ctx, key).Result()
}

// requireClient 校验底层客户端可用。
func (rc *RedisCache) requireClient() error {
	if rc == nil || rc.rdb == nil {
		return errors.New("redis client is nil: call EnableCache first")
	}
	return nil
}
