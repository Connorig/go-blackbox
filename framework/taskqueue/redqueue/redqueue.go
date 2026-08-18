// Package redqueue 提供基于 Redis 的可靠任务队列:
// 即时任务走 List,延迟任务走 ZSet(score=执行时间),多实例可并行消费。
// 与进程内 taskqueue 互补:进程内队列重启丢失,redqueue 持久化且支持多实例;
// 无需部署 MQ 的轻量场景下可作为可靠延迟队列使用。
package redqueue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Queue 是基于 Redis 的可靠任务队列。
type Queue struct {
	client    *redis.Client
	keyPrefix string
}

// NewQueue 创建队列;client 为 go-redis 客户端(可用 cache.RedisCache.Client())。
// keyPrefix 用于多队列隔离(如 "order-tasks")。
func NewQueue(client *redis.Client, keyPrefix string) *Queue {
	return &Queue{client: client, keyPrefix: keyPrefix}
}

// listKey 即时任务队列 key。
func (q *Queue) listKey() string { return q.keyPrefix + ":list" }

// zsetKey 延迟任务 ZSet key。
func (q *Queue) zsetKey() string { return q.keyPrefix + ":zset" }

// Submit 提交任务:delay<=0 走即时队列,否则走延迟队列(执行时间 = now+delay)。
func (q *Queue) Submit(ctx context.Context, payload []byte, delay time.Duration) error {
	if q == nil || q.client == nil {
		return errors.New("redqueue: client is nil")
	}
	if ctx == nil {
		return errors.New("redqueue: context is nil")
	}
	if len(payload) == 0 {
		return errors.New("redqueue: payload is empty")
	}
	if delay <= 0 {
		if err := q.client.LPush(ctx, q.listKey(), payload).Err(); err != nil {
			return fmt.Errorf("redqueue: enqueue immediate task: %w", err)
		}
		return nil
	}
	score := float64(time.Now().Add(delay).UnixMilli())
	if err := q.client.ZAdd(ctx, q.zsetKey(), redis.Z{Score: score, Member: payload}).Err(); err != nil {
		return fmt.Errorf("redqueue: enqueue delayed task: %w", err)
	}
	return nil
}

// Pending 返回待处理任务总数(即时队列 + 延迟队列)。
func (q *Queue) Pending(ctx context.Context) (int64, error) {
	if q == nil || q.client == nil {
		return 0, errors.New("redqueue: client is nil")
	}
	listLen, err := q.client.LLen(ctx, q.listKey()).Result()
	if err != nil {
		return 0, fmt.Errorf("redqueue: list length: %w", err)
	}
	zsetLen, err := q.client.ZCard(ctx, q.zsetKey()).Result()
	if err != nil {
		return 0, fmt.Errorf("redqueue: zset length: %w", err)
	}
	return listLen + zsetLen, nil
}

// Consume 阻塞消费任务:延迟任务到期后原子搬入即时队列,再阻塞取任务执行。
// handler 返回错误时任务重新入队(最多无限重试;业务可自行判断后返回 nil 放弃)。
// ctx 取消时优雅退出(处理中的任务完成后返回)。
func (q *Queue) Consume(ctx context.Context, handler func(ctx context.Context, payload []byte) error) error {
	if q == nil || q.client == nil {
		return errors.New("redqueue: client is nil")
	}
	if ctx == nil {
		return errors.New("redqueue: context is nil")
	}
	if handler == nil {
		return errors.New("redqueue: handler is nil")
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// 1) 延迟任务到期搬移(Lua 原子)
		if err := q.moveDueTasks(ctx); err != nil {
			return err
		}

		// 2) 阻塞取即时任务(BRPop 超时 1s,期间可响应取消)
		results, err := q.client.BRPop(ctx, time.Second, q.listKey()).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				select {
				case <-ctx.Done():
					return nil
				default:
					continue
				}
			}
			return fmt.Errorf("redqueue: blocking pop: %w", err)
		}
		if len(results) < 2 {
			continue
		}
		payload := []byte(results[1])

		if err := handler(ctx, payload); err != nil {
			// 处理失败:重新入队(延迟 1s,避免紧循环)
			if requeueErr := q.Submit(ctx, payload, time.Second); requeueErr != nil {
				return fmt.Errorf("redqueue: handler failed (%v) and requeue failed: %w", err, requeueErr)
			}
		}
	}
}

// moveDueTasks 原子搬移到期延迟任务到即时队列(Lua)。
func (q *Queue) moveDueTasks(ctx context.Context) error {
	const luaScript = `
local items = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, 100)
if #items == 0 then return 0 end
redis.call('ZREM', KEYS[1], unpack(items))
for i = 1, #items do redis.call('LPUSH', KEYS[2], items[i]) end
return #items`
	if err := q.client.Eval(ctx, luaScript, []string{q.zsetKey(), q.listKey()}, time.Now().UnixMilli()).Err(); err != nil {
		return fmt.Errorf("redqueue: move due tasks: %w", err)
	}
	return nil
}
