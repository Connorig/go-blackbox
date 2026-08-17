// Package ratelimit 提供 Redis 令牌桶分布式限流:
// 多实例共享配额,原子 Lua 脚本保证正确性。
// 场景:接口限流(跨实例)、短信发送频率、防刷。
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// tokenBucketScript Redis 令牌桶 Lua 脚本(原子):
// KEYS[1]=key ARGV[1]=rate(每秒) ARGV[2]=burst(容量) ARGV[3]=now(毫秒) ARGV[4]=requested
var tokenBucketScript = redis.NewScript(`
local key = KEYS[1]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local data = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts = tonumber(data[2])
if tokens == nil then
  tokens = burst
  ts = now
end
local delta = math.max(0, now - ts)
tokens = math.min(burst, tokens + delta * rate / 1000)
local allowed = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
end
redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
redis.call('PEXPIRE', key, math.max(1, math.ceil(burst / rate * 1000) * 2))
return allowed
`)

// Limiter 分布式限流器。
type Limiter struct {
	rdb    *redis.Client
	prefix string
}

// NewLimiter 创建限流器。prefix 为 key 前缀(如 "rl:api"),可为空。
func NewLimiter(rdb *redis.Client, prefix string) *Limiter {
	return &Limiter{rdb: rdb, prefix: prefix}
}

// Allow 请求 1 个令牌:放行返回 true;超额返回 false;错误返回 error。
// rate 为每秒补充令牌数,burst 为桶容量(允许突发)。
func (l *Limiter) Allow(ctx context.Context, key string, rate, burst int) (bool, error) {
	return l.AllowN(ctx, key, rate, burst, 1)
}

// AllowN 请求 n 个令牌。
func (l *Limiter) AllowN(ctx context.Context, key string, rate, burst, n int) (bool, error) {
	if l == nil || l.rdb == nil {
		return false, errors.New("rate limit: redis client is nil")
	}
	if ctx == nil {
		return false, errors.New("rate limit: context is nil")
	}
	if key == "" {
		return false, errors.New("rate limit: key is empty")
	}
	if rate <= 0 || burst <= 0 || n <= 0 {
		return false, fmt.Errorf("rate limit: invalid params rate=%d burst=%d n=%d", rate, burst, n)
	}
	redisKey := l.prefix + ":" + key
	if l.prefix == "" {
		redisKey = key
	}
	result, err := tokenBucketScript.Run(ctx, l.rdb, []string{redisKey}, rate, burst, time.Now().UnixMilli(), n).Int()
	if err != nil {
		return false, fmt.Errorf("rate limit %q: %w", key, err)
	}
	return result == 1, nil
}

// Reset 清空指定 key 的令牌状态(测试/手动解封)。
func (l *Limiter) Reset(ctx context.Context, key string) error {
	if l == nil || l.rdb == nil {
		return errors.New("rate limit reset: redis client is nil")
	}
	redisKey := l.prefix + ":" + key
	if l.prefix == "" {
		redisKey = key
	}
	if err := l.rdb.Del(ctx, redisKey).Err(); err != nil {
		return fmt.Errorf("rate limit reset %q: %w", key, err)
	}
	return nil
}
