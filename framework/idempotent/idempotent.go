// Package idempotent 提供基于 Redis 的业务幂等防护:
// 首次请求执行业务并占用幂等标记,重复请求(回调重试/用户连点/消息重投)直接拒绝。
// 与分布式锁(cache.Lock)的区别:锁是执行期间互斥(用完释放);
// 幂等标记是结果性占用(执行成功后仍保留,TTL 内重复请求一律拒绝)。
package idempotent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Guard 幂等守卫:基于 Redis SETNX 的标记占用。
type Guard struct {
	rdb    *redis.Client
	prefix string
}

// NewGuard 创建幂等守卫。prefix 为 key 统一前缀(如 "idem:gift"),可为空。
func NewGuard(rdb *redis.Client, prefix string) *Guard {
	return &Guard{rdb: rdb, prefix: prefix}
}

// Check 检查业务键是否首次执行:
// 首次执行返回 true 并占用标记;重复请求返回 false(业务应直接拒绝或返回缓存结果)。
// ttl 控制标记存活期(支付回调 24h、防重复提交 10min 等),非正数默认 10 分钟。
func (g *Guard) Check(ctx context.Context, bizKey string, ttl time.Duration) (bool, error) {
	if g == nil || g.rdb == nil {
		return false, errors.New("idempotent check: redis client is nil")
	}
	if ctx == nil {
		return false, errors.New("idempotent check: context is nil")
	}
	if bizKey == "" {
		return false, errors.New("idempotent check: business key is empty")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	acquired, err := g.rdb.SetNX(ctx, g.key(bizKey), time.Now().Format(time.RFC3339), ttl).Result()
	if err != nil {
		return false, fmt.Errorf("idempotent check %q: %w", bizKey, err)
	}
	return acquired, nil
}

// Release 手动释放幂等标记(业务执行失败时回滚,允许重试)。
// 业务成功后不应调用(标记保留用于拒绝重复请求)。
func (g *Guard) Release(ctx context.Context, bizKey string) error {
	if g == nil || g.rdb == nil {
		return errors.New("idempotent release: redis client is nil")
	}
	if ctx == nil {
		return errors.New("idempotent release: context is nil")
	}
	if err := g.rdb.Del(ctx, g.key(bizKey)).Err(); err != nil {
		return fmt.Errorf("idempotent release %q: %w", bizKey, err)
	}
	return nil
}

// Status 查询幂等标记是否存在(存在 = 已执行过)。
func (g *Guard) Status(ctx context.Context, bizKey string) (bool, error) {
	if g == nil || g.rdb == nil {
		return false, errors.New("idempotent status: redis client is nil")
	}
	if ctx == nil {
		return false, errors.New("idempotent status: context is nil")
	}
	count, err := g.rdb.Exists(ctx, g.key(bizKey)).Result()
	if err != nil {
		return false, fmt.Errorf("idempotent status %q: %w", bizKey, err)
	}
	return count > 0, nil
}

// key 组装 Redis key。
func (g *Guard) key(bizKey string) string {
	if g.prefix == "" {
		return bizKey
	}
	return g.prefix + ":" + bizKey
}
