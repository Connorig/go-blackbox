package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// lockPrefix 是分布式锁 key 的统一前缀。
const lockPrefix = "lock:"

// Lock 表示一个已持有的分布式锁。
// 释放或续期时都会校验持有者 token，防止误删他人持有的锁。
type Lock struct {
	rdb   *redis.Client
	key   string
	token string
	ttl   time.Duration
}

// TryLock 尝试获取分布式锁，立即返回。
// 获取成功返回非 nil Lock；key 已被占用时返回 (nil, nil)；错误时返回错误。
func (rc *RedisCache) TryLock(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	if rc == nil || rc.rdb == nil {
		return nil, errors.New("acquire lock: redis client is nil")
	}
	if ctx == nil {
		return nil, errors.New("acquire lock: context is nil")
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	token, err := newLockToken()
	if err != nil {
		return nil, fmt.Errorf("acquire lock %q: %w", key, err)
	}
	lockKey := lockPrefix + key
	acquired, err := rc.rdb.SetNX(ctx, lockKey, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("acquire lock %q: %w", key, err)
	}
	if !acquired {
		return nil, nil
	}
	return &Lock{rdb: rc.rdb, key: lockKey, token: token, ttl: ttl}, nil
}

// Lock 阻塞获取分布式锁，直到成功、超时或 Context 取消。
// waitTimeout 非正数时使用 5 秒默认值。
func (rc *RedisCache) Lock(ctx context.Context, key string, ttl, waitTimeout time.Duration) (*Lock, error) {
	if waitTimeout <= 0 {
		waitTimeout = 5 * time.Second
	}
	deadline := time.Now().Add(waitTimeout)
	for {
		lock, err := rc.TryLock(ctx, key, ttl)
		if err != nil {
			return nil, err
		}
		if lock != nil {
			return lock, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("acquire lock %q: timeout after %s", key, waitTimeout)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("acquire lock %q: %w", key, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Unlock 释放锁；只有持有者 token 匹配时才删除（防误删）。
// 重复释放安全。
func (l *Lock) Unlock(ctx context.Context) error {
	if l == nil || l.rdb == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("release lock: context is nil")
	}
	// Lua 脚本保证「校验 + 删除」原子执行
	script := redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
end
return 0`)
	if err := script.Run(ctx, l.rdb, []string{l.key}, l.token).Err(); err != nil {
		return fmt.Errorf("release lock %q: %w", l.key, err)
	}
	return nil
}

// Renew 续期锁；仅持有者 token 匹配时更新过期时间。
func (l *Lock) Renew(ctx context.Context, ttl time.Duration) error {
	if l == nil || l.rdb == nil {
		return errors.New("renew lock: lock is nil")
	}
	if ctx == nil {
		return errors.New("renew lock: context is nil")
	}
	if ttl <= 0 {
		ttl = l.ttl
	}
	script := redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0`)
	if err := script.Run(ctx, l.rdb, []string{l.key}, l.token, ttl.Milliseconds()).Err(); err != nil {
		return fmt.Errorf("renew lock %q: %w", l.key, err)
	}
	return nil
}

// newLockToken 生成锁持有者随机标识。
func newLockToken() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
