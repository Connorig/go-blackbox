package oplog

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisClient 连接测试 Redis;未设置环境变量时跳过。
func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("GO_BLACKBOX_REDIS_ADDR")
	if addr == "" {
		t.Skip("Redis integration test requires GO_BLACKBOX_REDIS_ADDR environment variable")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 0})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping redis failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// TestRedisSinkWriteAndQuery 验证写入与查询往返(倒序 + 分页)。
func TestRedisSinkWriteAndQuery(t *testing.T) {
	client := redisClient(t)
	const key = "go-blackbox-test-oplog"
	_ = client.Del(context.Background(), key)

	sink := NewRedisListSink(client, key, 0)
	ctx := context.Background()
	now := time.Now()
	entries := []Entry{
		{Time: now.Add(-2 * time.Second), UserID: 1, Method: "GET", Path: "/a", Status: 200, RequestID: "req-1"},
		{Time: now.Add(-time.Second), UserID: 2, Method: "POST", Path: "/b", Status: 201, RequestID: "req-2"},
		{Time: now, UserID: 1, Method: "DELETE", Path: "/c", Status: 204, RequestID: "req-3"},
	}
	if err := sink.Write(ctx, entries); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// 查询:倒序(最新在前)
	queried, err := Query(ctx, client, key, 0, 10)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(queried) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(queried))
	}
	if queried[0].RequestID != "req-3" {
		t.Fatalf("latest entry must come first, got %s", queried[0].RequestID)
	}
	if queried[2].RequestID != "req-1" {
		t.Fatalf("oldest entry must come last, got %s", queried[2].RequestID)
	}
	if queried[1].UserID != 2 || queried[1].Method != "POST" {
		t.Fatalf("unexpected entry: %+v", queried[1])
	}

	// 分页
	page, err := Query(ctx, client, key, 1, 1)
	if err != nil {
		t.Fatalf("page query failed: %v", err)
	}
	if len(page) != 1 || page[0].RequestID != "req-2" {
		t.Fatalf("unexpected page: %+v", page)
	}

	total, err := Count(ctx, client, key)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 total, got %d", total)
	}
}

// TestRedisSinkLimitTrim 验证 limit 截断保留最近 N 条。
func TestRedisSinkLimitTrim(t *testing.T) {
	client := redisClient(t)
	const key = "go-blackbox-test-oplog-limit"
	_ = client.Del(context.Background(), key)

	sink := NewRedisListSink(client, key, 2)
	ctx := context.Background()
	for index := 1; index <= 5; index++ {
		entry := Entry{Time: time.Now(), UserID: int64(index), Path: "/x", Status: 200}
		if err := sink.Write(ctx, []Entry{entry}); err != nil {
			t.Fatalf("write %d failed: %v", index, err)
		}
	}
	total, err := Count(ctx, client, key)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("limit must trim to 2, got %d", total)
	}
	queried, err := Query(ctx, client, key, 0, 10)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(queried) != 2 || queried[0].UserID != 5 || queried[1].UserID != 4 {
		t.Fatalf("must keep the latest entries: %+v", queried)
	}
}

// TestQuerySkipsCorruptEntries 验证损坏条目被跳过。
func TestQuerySkipsCorruptEntries(t *testing.T) {
	client := redisClient(t)
	const key = "go-blackbox-test-oplog-corrupt"
	_ = client.Del(context.Background(), key)
	ctx := context.Background()

	// 先写一条正常,再手动插一条损坏数据,再写一条正常
	sink := NewRedisListSink(client, key, 0)
	_ = sink.Write(ctx, []Entry{{Time: time.Now(), UserID: 1, Path: "/ok1"}})
	if err := client.LPush(ctx, key, "{not-json").Err(); err != nil {
		t.Fatalf("push corrupt failed: %v", err)
	}
	_ = sink.Write(ctx, []Entry{{Time: time.Now(), UserID: 3, Path: "/ok3"}})

	queried, err := Query(ctx, client, key, 0, 10)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(queried) != 2 {
		t.Fatalf("corrupt entry must be skipped, got %d", len(queried))
	}
	if queried[0].UserID != 3 || queried[1].UserID != 1 {
		t.Fatalf("unexpected entries: %+v", queried)
	}
}

// TestRedisSinkValidation 验证 nil 安全。
func TestRedisSinkValidation(t *testing.T) {
	var sink *RedisListSink
	if err := sink.Write(context.Background(), []Entry{{}}); err == nil {
		t.Fatal("nil sink must return error")
	}
	if _, err := Query(context.Background(), nil, "k", 0, 10); err == nil {
		t.Fatal("nil client query must return error")
	}
	if _, err := Count(context.Background(), nil, "k"); err == nil {
		t.Fatal("nil client count must return error")
	}
	sink = NewRedisListSink(nil, "k", 0)
	if err := sink.Write(context.Background(), []Entry{{}}); err == nil {
		t.Fatal("sink with nil client must return error")
	}
}
