package eventbus

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestBridgeValidation 验证桥接参数校验。
func TestBridgeValidation(t *testing.T) {
	var bridge *RedisBridge
	if err := bridge.Start(context.Background()); err == nil {
		t.Fatal("nil bridge must return error")
	}
	bridge = NewRedisBridge(nil, "ch", New(false))
	if err := bridge.Start(context.Background()); err == nil {
		t.Fatal("nil client must return error")
	}
	if err := bridge.Publish(context.Background(), Event{Name: "x"}); err == nil {
		t.Fatal("publish with nil client must return error")
	}
	bridge = NewRedisBridge(nil, "ch", New(false))
	if err := bridge.Publish(context.Background(), Event{Name: ""}); err == nil {
		t.Fatal("empty event name must return error")
	}
}

// TestBridgePublishDecode 验证事件序列化往返。
func TestBridgePublishDecode(t *testing.T) {
	original := Event{Name: "order.created", Data: map[string]int{"orderId": 42}}
	payload, err := jsonMarshalEvent(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	decoded, err := decodeEvent(payload)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.Name != original.Name {
		t.Fatalf("unexpected name: %s", decoded.Name)
	}
	data, ok := decoded.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data must decode to map, got %T", decoded.Data)
	}
	if int(data["orderId"].(float64)) != 42 {
		t.Fatalf("unexpected data: %v", decoded.Data)
	}
}

// jsonMarshalEvent 测试辅助:与 RedisBridge.Publish 一致的序列化。
func jsonMarshalEvent(event Event) ([]byte, error) {
	return json.Marshal(event)
}

// TestBridgeCrossInstanceDelivery 验证跨实例事件投递(需真实 Redis)。
func TestBridgeCrossInstanceDelivery(t *testing.T) {
	addr := os.Getenv("GO_BLACKBOX_REDIS_ADDR")
	if addr == "" {
		t.Skip("Redis integration test requires GO_BLACKBOX_REDIS_ADDR environment variable")
	}

	clientA := redis.NewClient(&redis.Options{Addr: addr, DB: 0})
	clientB := redis.NewClient(&redis.Options{Addr: addr, DB: 0})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
	})

	const channel = "go-blackbox-test-eventbus"
	// 实例 B 的本地总线
	busB := New(false)
	received := make(chan Event, 1)
	busB.Subscribe("order.created", func(_ context.Context, event Event) error {
		received <- event
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// 实例 B 启动桥接(订阅 Redis)
	go func() {
		_ = NewRedisBridge(clientB, channel, busB).Start(ctx)
	}()

	// 实例 A 发布到 Redis
	bridgeA := NewRedisBridge(clientA, channel, New(false))
	deadline := time.Now().Add(3 * time.Second)
	var publishErr error
	for time.Now().Before(deadline) {
		publishErr = bridgeA.Publish(ctx, Event{Name: "order.created", Data: map[string]string{"source": "instance-a"}})
		if publishErr == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if publishErr != nil {
		t.Fatalf("publish failed: %v", publishErr)
	}

	select {
	case event := <-received:
		if event.Name != "order.created" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cross-instance event")
	}
}
