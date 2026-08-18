package oplog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisListSink 把审计日志写入 Redis List(LPUSH 头部插入,倒序保留最近 limit 条)。
// 多实例共享存储,零配置审计落地;配合 Query 提供最近操作日志查询。
// 大数据量/复杂检索场景请改用 ES 或数据库 Sink。
type RedisListSink struct {
	client *redis.Client
	key    string
	limit  int64
}

// NewRedisListSink 创建 Redis 审计存储。
// key 为存储键(多应用隔离用不同 key);limit 为保留条数(<=0 表示不限制,注意内存占用)。
func NewRedisListSink(client *redis.Client, key string, limit int64) *RedisListSink {
	return &RedisListSink{client: client, key: key, limit: limit}
}

// Write 批量写入条目(倒序 LPUSH 保持列表内时间倒序)。
// 写入失败返回错误;单条序列化失败整批返回错误(条目内部数据必须可 JSON 序列化)。
func (s *RedisListSink) Write(ctx context.Context, entries []Entry) error {
	if s == nil || s.client == nil {
		return errors.New("oplog redis sink: client is nil")
	}
	if ctx == nil {
		return errors.New("oplog redis sink: context is nil")
	}
	if len(entries) == 0 {
		return nil
	}
	// 倒序插入:列表头是最新日志
	for index := len(entries) - 1; index >= 0; index-- {
		payload, err := json.Marshal(entries[index])
		if err != nil {
			return fmt.Errorf("oplog redis sink: marshal entry: %w", err)
		}
		if err := s.client.LPush(ctx, s.key, payload).Err(); err != nil {
			return fmt.Errorf("oplog redis sink: push entry: %w", err)
		}
	}
	if s.limit > 0 {
		if err := s.client.LTrim(ctx, s.key, 0, s.limit-1).Err(); err != nil {
			return fmt.Errorf("oplog redis sink: trim to %d: %w", s.limit, err)
		}
	}
	return nil
}

// Query 查询最近审计日志(时间倒序,offset/count 分页)。
// 损坏条目(历史格式不兼容)自动跳过,不影响整体查询。
func Query(ctx context.Context, client *redis.Client, key string, offset, count int64) ([]Entry, error) {
	if client == nil {
		return nil, errors.New("oplog query: client is nil")
	}
	if ctx == nil {
		return nil, errors.New("oplog query: context is nil")
	}
	if offset < 0 {
		offset = 0
	}
	if count <= 0 {
		count = 20
	}
	values, err := client.LRange(ctx, key, offset, offset+count-1).Result()
	if err != nil {
		return nil, fmt.Errorf("oplog query: range %d-%d: %w", offset, offset+count-1, err)
	}
	entries := make([]Entry, 0, len(values))
	for _, value := range values {
		var entry Entry
		if err := json.Unmarshal([]byte(value), &entry); err != nil {
			continue // 跳过损坏条目
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Count 返回存储的审计日志总数。
func Count(ctx context.Context, client *redis.Client, key string) (int64, error) {
	if client == nil {
		return 0, errors.New("oplog count: client is nil")
	}
	total, err := client.LLen(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("oplog count: %w", err)
	}
	return total, nil
}
