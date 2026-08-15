package kafka

import (
	"context"
	"os"
	"testing"
	"time"
)

// brokers 返回测试 Kafka 地址;未配置时跳过。
func brokers(t *testing.T) []string {
	t.Helper()
	addr := os.Getenv("GO_BLACKBOX_KAFKA_ADDR")
	if addr == "" {
		t.Skip("kafka not configured: set GO_BLACKBOX_KAFKA_ADDR to run")
	}
	return []string{addr}
}

// TestNewProducerInvalid 非法配置报错。
func TestNewProducerInvalid(t *testing.T) {
	if _, err := NewProducer(Config{Brokers: nil}); err == nil {
		t.Fatal("nil brokers must fail")
	}
}

// TestProduceConsume 生产/消费闭环(需真实 Kafka)。
func TestProduceConsume(t *testing.T) {
	topic := "gbx-test-topic"
	groupID := "gbx-test-group"

	producer, err := NewProducer(Config{Brokers: brokers(t), Topic: topic, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("new producer failed: %v", err)
	}
	defer producer.Close()

	ctx := context.Background()
	if err := producer.SendJSON(ctx, topic, "order-1", map[string]interface{}{"order_id": "1001", "amount": 99}); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	consumer, err := NewConsumer(Config{Brokers: brokers(t), Topic: topic, GroupID: groupID, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("new consumer failed: %v", err)
	}
	defer consumer.Close()

	// 消费到消息后停止
	consumeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	received := make(chan Message, 1)
	go func() {
		_ = consumer.Consume(consumeCtx, func(ctx context.Context, message Message) error {
			received <- message
			return nil
		})
	}()

	select {
	case message := <-received:
		if message.Key != "order-1" {
			t.Fatalf("key = %q", message.Key)
		}
		if len(message.Value) == 0 {
			t.Fatal("value empty")
		}
	case <-consumeCtx.Done():
		t.Fatal("consume timeout")
	}
}
