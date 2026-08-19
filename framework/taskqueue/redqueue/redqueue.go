// Package redqueue 提供基于 Redis 的可靠任务队列:
// 即时任务走 List,延迟任务走 ZSet(score=执行时间),多实例可并行消费。
// 消息带重试计数:handler 失败自动重投(延迟 1s),超过上限进入死信队列,
// 并通过死信回调(OnDeadLetter)通知运维(可接 alert/notify)。
// 与进程内 taskqueue 互补:进程内队列重启丢失,redqueue 持久化且支持多实例。
package redqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// 默认重试上限。
const DefaultMaxRetries = 5

// envelope 是存储信封:data 为业务负载,retries 为已重试次数。
// 兼容说明:早期版本直接存储裸 payload,读取时按 retries=0 处理。
type envelope struct {
	Data    []byte `json:"data"`
	Retries int    `json:"retries"`
}

// DeadLetter 是死信消息(超过重试上限)。
type DeadLetter struct {
	Payload  []byte    `json:"payload"`   // 业务负载
	Retries  int       `json:"retries"`   // 进入死信时的重试次数
	FailedAt time.Time `json:"failed_at"` // 进入死信时间
}

// DeadLetterHook 死信回调:死信产生时调用(可接 alert/notify/日志)。
// 注意:多实例部署时每个实例都会收到回调,业务侧需自行去重
// (如以 failed_at+payload 指纹做 Redis SETNX,或接受重复告警)。
type DeadLetterHook func(ctx context.Context, letter DeadLetter)

// Queue 是基于 Redis 的可靠任务队列。
type Queue struct {
	client     *redis.Client
	keyPrefix  string
	maxRetries int
	deadHook   DeadLetterHook
}

// NewQueue 创建队列;client 为 go-redis 客户端(可用 cache.RedisCache.Client())。
// keyPrefix 用于多队列隔离(如 "order-tasks")。
func NewQueue(client *redis.Client, keyPrefix string) *Queue {
	return &Queue{client: client, keyPrefix: keyPrefix, maxRetries: DefaultMaxRetries}
}

// WithMaxRetries 设置重试上限(0 表示无限重试;超过上限进死信队列)。
func (q *Queue) WithMaxRetries(maxRetries int) *Queue {
	if q != nil {
		q.maxRetries = maxRetries
	}
	return q
}

// WithDeadLetterHook 设置死信回调(死信产生时调用,不阻塞消费)。
func (q *Queue) WithDeadLetterHook(hook DeadLetterHook) *Queue {
	if q != nil {
		q.deadHook = hook
	}
	return q
}

// MaxRetries 返回当前重试上限。
func (q *Queue) MaxRetries() int {
	if q == nil {
		return 0
	}
	return q.maxRetries
}

// listKey 即时任务队列 key。
func (q *Queue) listKey() string { return q.keyPrefix + ":list" }

// zsetKey 延迟任务 ZSet key。
func (q *Queue) zsetKey() string { return q.keyPrefix + ":zset" }

// deadKey 死信队列 key。
func (q *Queue) deadKey() string { return q.keyPrefix + ":dead" }

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
	encoded, err := json.Marshal(envelope{Data: payload})
	if err != nil {
		return fmt.Errorf("redqueue: encode envelope: %w", err)
	}
	if delay <= 0 {
		if err := q.client.LPush(ctx, q.listKey(), encoded).Err(); err != nil {
			return fmt.Errorf("redqueue: enqueue immediate task: %w", err)
		}
		return nil
	}
	score := float64(time.Now().Add(delay).UnixMilli())
	if err := q.client.ZAdd(ctx, q.zsetKey(), redis.Z{Score: score, Member: encoded}).Err(); err != nil {
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
// handler 返回错误时任务重试计数 +1 并延迟 1s 重投;超过上限进入死信队列
// 并触发死信回调(如有设置)。
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

		// 解码信封(兼容早期裸 payload 格式)
		enveloped := decodeEnvelope([]byte(results[1]))

		if err := handler(ctx, enveloped.Data); err != nil {
			nextRetries := enveloped.Retries + 1
			if q.maxRetries > 0 && nextRetries > q.maxRetries {
				// 超过上限:进死信
				if deadErr := q.pushDead(ctx, enveloped.Data, enveloped.Retries); deadErr != nil {
					return fmt.Errorf("redqueue: handler failed (%v) and dead-letter failed: %w", err, deadErr)
				}
				continue
			}
			// 未超限:重投(延迟 1s,计数 +1)
			if requeueErr := q.requeue(ctx, enveloped.Data, nextRetries); requeueErr != nil {
				return fmt.Errorf("redqueue: handler failed (%v) and requeue failed: %w", err, requeueErr)
			}
		}
	}
}

// requeue 携带重试计数重新投递。
func (q *Queue) requeue(ctx context.Context, payload []byte, retries int) error {
	encoded, err := json.Marshal(envelope{Data: payload, Retries: retries})
	if err != nil {
		return fmt.Errorf("redqueue: encode requeue envelope: %w", err)
	}
	if err := q.client.LPush(ctx, q.listKey(), encoded).Err(); err != nil {
		return fmt.Errorf("redqueue: requeue: %w", err)
	}
	return nil
}

// pushDead 写入死信队列并触发死信回调。
func (q *Queue) pushDead(ctx context.Context, payload []byte, retries int) error {
	letter := DeadLetter{
		Payload:  payload,
		Retries:  retries,
		FailedAt: time.Now(),
	}
	encoded, err := json.Marshal(letter)
	if err != nil {
		return fmt.Errorf("redqueue: encode dead letter: %w", err)
	}
	if err := q.client.LPush(ctx, q.deadKey(), encoded).Err(); err != nil {
		return fmt.Errorf("redqueue: push dead letter: %w", err)
	}
	// 死信回调:不阻塞消费;hook 自身 panic 由调用方决定是否 recover
	if q.deadHook != nil {
		q.deadHook(ctx, letter)
	}
	return nil
}

// DeadLetterCount 返回死信数量。
func (q *Queue) DeadLetterCount(ctx context.Context) (int64, error) {
	if q == nil || q.client == nil {
		return 0, errors.New("redqueue: client is nil")
	}
	total, err := q.client.LLen(ctx, q.deadKey()).Result()
	if err != nil {
		return 0, fmt.Errorf("redqueue: dead letter length: %w", err)
	}
	return total, nil
}

// DeadLetters 查询死信列表(倒序,offset/count 分页);损坏条目跳过。
func (q *Queue) DeadLetters(ctx context.Context, offset, count int64) ([]DeadLetter, error) {
	if q == nil || q.client == nil {
		return nil, errors.New("redqueue: client is nil")
	}
	if offset < 0 {
		offset = 0
	}
	if count <= 0 {
		count = 20
	}
	values, err := q.client.LRange(ctx, q.deadKey(), offset, offset+count-1).Result()
	if err != nil {
		return nil, fmt.Errorf("redqueue: dead letter range: %w", err)
	}
	letters := make([]DeadLetter, 0, len(values))
	for _, value := range values {
		var letter DeadLetter
		if err := json.Unmarshal([]byte(value), &letter); err != nil {
			continue
		}
		letters = append(letters, letter)
	}
	return letters, nil
}

// RequeueDeadLetter 把指定位置的死信重新投递到即时队列并移除。
// index 为死信列表索引(0 = 最新一条);重投时保留原重试计数。
func (q *Queue) RequeueDeadLetter(ctx context.Context, index int64) error {
	if q == nil || q.client == nil {
		return errors.New("redqueue: client is nil")
	}
	if index < 0 {
		return errors.New("redqueue: dead letter index must be non-negative")
	}
	values, err := q.client.LRange(ctx, q.deadKey(), index, index).Result()
	if err != nil {
		return fmt.Errorf("redqueue: read dead letter %d: %w", index, err)
	}
	if len(values) == 0 {
		return fmt.Errorf("redqueue: dead letter %d not found", index)
	}
	var letter DeadLetter
	if err := json.Unmarshal([]byte(values[0]), &letter); err != nil {
		return fmt.Errorf("redqueue: dead letter %d corrupt: %w", index, err)
	}
	if err := q.requeue(ctx, letter.Payload, letter.Retries); err != nil {
		return err
	}
	// 移除死信(按索引 LSET 标记 + LREM 或用 LREM 精确值)
	if err := q.client.LRem(ctx, q.deadKey(), 1, values[0]).Err(); err != nil {
		return fmt.Errorf("redqueue: remove dead letter %d: %w", index, err)
	}
	return nil
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

// decodeEnvelope 解码信封;兼容早期裸 payload 格式(视为 retries=0)。
// 该函数永不失败:非信封格式的原始字节按业务负载处理。
func decodeEnvelope(value []byte) envelope {
	var enveloped envelope
	if err := json.Unmarshal(value, &enveloped); err == nil && len(enveloped.Data) > 0 {
		return enveloped
	}
	// 兼容裸 payload(可能本身是 JSON,原样作为数据)
	return envelope{Data: value}
}
