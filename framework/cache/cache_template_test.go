package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

// redisAddr 返回测试 Redis 地址;未配置时跳过(CI 无 Redis 环境)。
func redisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("GO_BLACKBOX_REDIS_ADDR")
	if addr == "" {
		t.Skip("redis not configured: set GO_BLACKBOX_REDIS_ADDR to run")
	}
	return addr
}

// testCache 连接测试 Redis(独立 db 避免污染)。
func testCache(t *testing.T) *RedisCache {
	t.Helper()
	ctx := context.Background()
	rc, err := Init(ctx, RedisOptions{Addr: redisAddr(t), DB: 15})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	// 清理测试键
	keys, _ := rc.rdb.Keys(ctx, "gbx-test:*").Result()
	if len(keys) > 0 {
		_ = rc.rdb.Del(ctx, keys...).Err()
	}
	return rc
}

// TestTemplateStringOps Incr/Decr/GetString。
func TestTemplateStringOps(t *testing.T) {
	rc := testCache(t)
	ctx := context.Background()
	key := "gbx-test:counter"

	value, err := rc.Incr(ctx, key)
	if err != nil || value != 1 {
		t.Fatalf("incr = %d, %v", value, err)
	}
	value, _ = rc.Incr(ctx, key)
	if value != 2 {
		t.Fatalf("second incr = %d", value)
	}
	value, _ = rc.Decr(ctx, key)
	if value != 1 {
		t.Fatalf("decr = %d", value)
	}
	if rc.Client() == nil {
		t.Fatal("Client() must return native client")
	}
	raw, err := rc.GetString(ctx, key)
	if err != nil || raw != "1" {
		t.Fatalf("get = %q, %v", raw, err)
	}
}

// TestTemplateSetNXExpireDel SetNX/Expire/Del。
func TestTemplateSetNXExpireDel(t *testing.T) {
	rc := testCache(t)
	ctx := context.Background()
	key := "gbx-test:setnx"

	ok, err := rc.SetNX(ctx, key, "v1", time.Minute)
	if err != nil || !ok {
		t.Fatalf("setnx first = %v, %v", ok, err)
	}
	ok, _ = rc.SetNX(ctx, key, "v2", time.Minute)
	if ok {
		t.Fatal("setnx second must fail")
	}
	expired, err := rc.Expire(ctx, key, time.Hour)
	if err != nil || !expired {
		t.Fatalf("expire = %v, %v", expired, err)
	}
	deleted, err := rc.Del(ctx, key)
	if err != nil || deleted != 1 {
		t.Fatalf("del = %d, %v", deleted, err)
	}
}

// TestTemplateHashOps HSet/HGet/HGetAll/HDel。
func TestTemplateHashOps(t *testing.T) {
	rc := testCache(t)
	ctx := context.Background()
	key := "gbx-test:user:1"

	if err := rc.HSet(ctx, key, "name", "connor"); err != nil {
		t.Fatalf("hset failed: %v", err)
	}
	if err := rc.HSet(ctx, key, "age", 18); err != nil {
		t.Fatalf("hset failed: %v", err)
	}
	name, err := rc.HGet(ctx, key, "name")
	if err != nil || name != "connor" {
		t.Fatalf("hget = %q, %v", name, err)
	}
	all, err := rc.HGetAll(ctx, key)
	if err != nil || all["name"] != "connor" || all["age"] != "18" {
		t.Fatalf("hgetall = %v, %v", all, err)
	}
	deleted, err := rc.HDel(ctx, key, "age")
	if err != nil || deleted != 1 {
		t.Fatalf("hdel = %d, %v", deleted, err)
	}
	_, _ = rc.Del(ctx, key)
}

// TestTemplateListOps LPush/RPop FIFO 队列。
func TestTemplateListOps(t *testing.T) {
	rc := testCache(t)
	ctx := context.Background()
	key := "gbx-test:queue"

	if _, err := rc.LPush(ctx, key, "a", "b", "c"); err != nil {
		t.Fatalf("lpush failed: %v", err)
	}
	// FIFO:LPush 头部压入 + RPop 尾部弹出 → a 先出
	first, err := rc.RPop(ctx, key)
	if err != nil || first != "a" {
		t.Fatalf("rpop first = %q, %v", first, err)
	}
	_, _ = rc.Del(ctx, key)
}

// TestTemplateNilClient 未初始化时明确报错。
func TestTemplateNilClient(t *testing.T) {
	ctx := context.Background()
	var rc *RedisCache
	if _, err := rc.Incr(ctx, "x"); err == nil {
		t.Fatal("nil client must fail")
	}
	if rc.Client() != nil {
		t.Fatal("nil client must return nil")
	}
}
