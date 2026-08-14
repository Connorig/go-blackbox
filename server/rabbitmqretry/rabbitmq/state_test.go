package rabbitmq

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestDialRejectsEmptyDNS 验证空连接串被拒绝。
func TestDialRejectsEmptyDNS(t *testing.T) {
	if _, err := Dial(""); err == nil {
		t.Fatal("empty dns must return an error")
	}
}

// TestConnectionNilSafety 验证 nil 连接上的方法调用安全。
func TestConnectionNilSafety(t *testing.T) {
	var conn *Connection
	if conn.State() != StateDisconnected {
		t.Fatal("nil connection state must be disconnected")
	}
	if !conn.IsClosed() {
		t.Fatal("nil connection must be considered closed")
	}
	if _, err := conn.Channel(); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close on nil connection must be no-op: %v", err)
	}
}

// TestConnectionCloseIdempotent 验证关闭幂等。
func TestConnectionCloseIdempotent(t *testing.T) {
	conn := &Connection{closed: true, state: StateClosed}
	if err := conn.Close(); err != nil {
		t.Fatalf("close must be idempotent: %v", err)
	}
	if conn.State() != StateClosed {
		t.Fatal("state must stay closed")
	}
}

// TestConnectionChannelWithoutConnection 验证未连接时 Channel 返回明确错误。
func TestConnectionChannelWithoutConnection(t *testing.T) {
	conn := &Connection{state: StateDisconnected}
	if _, err := conn.Channel(); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected, got: %v", err)
	}
}

// TestConnectionConnectStateFlow 验证状态机流转：connecting → connected / 失败回 disconnected。
func TestConnectionConnectStateFlow(t *testing.T) {
	conn := &Connection{state: StateDisconnected, reconnectInterval: time.Second}
	// 无效 dns 应失败并回到 disconnected
	conn.dns = "amqp://invalid:invalid@127.0.0.1:1/"
	if err := conn.connect(); err == nil {
		t.Fatal("connect to invalid address must fail")
	}
	if conn.State() != StateDisconnected {
		t.Fatalf("failed connect must return to disconnected, got %s", conn.State())
	}
}

// TestStateString 验证状态可读名称。
func TestStateString(t *testing.T) {
	cases := map[State]string{
		StateDisconnected: "disconnected",
		StateConnecting:   "connecting",
		StateConnected:    "connected",
		StateClosed:       "closed",
	}
	for state, expected := range cases {
		if state.String() != expected {
			t.Fatalf("unexpected state string for %d: %s", state, state.String())
		}
	}
}

// TestNewConsumerValidation 验证消费者参数校验。
func TestNewConsumerValidation(t *testing.T) {
	if _, err := NewConsumer(nil, QueueExchange{QuName: "q"}, nil); err == nil {
		t.Fatal("nil connection must be rejected")
	}
	conn := &Connection{state: StateDisconnected}
	if _, err := NewConsumer(conn, QueueExchange{QuName: "q"}, nil); err == nil {
		t.Fatal("nil receiver must be rejected")
	}
	if _, err := NewConsumer(conn, QueueExchange{}, &testReceiver{}); err == nil {
		t.Fatal("empty queue name must be rejected")
	}
}

// TestConsumerStartValidation 验证 Start 参数校验。
func TestConsumerStartValidation(t *testing.T) {
	consumer := &Consumer{}
	if err := consumer.Start(context.Background()); err == nil {
		t.Fatal("consumer without connection must return an error")
	}
	consumer = &Consumer{conn: &Connection{state: StateDisconnected}}
	if err := consumer.Start(nil); err == nil {
		t.Fatal("nil context must return an error")
	}
}

// TestProducerPublishWithoutConnection 验证未连接时发送返回明确错误。
func TestProducerPublishWithoutConnection(t *testing.T) {
	producer := NewProducer(nil, QueueExchange{QuName: "q"})
	if err := producer.Publish(context.Background(), map[string]string{"k": "v"}); err == nil {
		t.Fatal("publish on nil connection must return an error")
	}
	producer = NewProducer(&Connection{state: StateDisconnected}, QueueExchange{QuName: "q"})
	if err := producer.Publish(nil, "body"); err == nil {
		t.Fatal("publish with nil context must return an error")
	}
}

// TestPublishDelayRequiresTTL 验证延时消息必须提供正数 TTL。
func TestPublishDelayRequiresTTL(t *testing.T) {
	producer := NewProducer(&Connection{state: StateDisconnected}, QueueExchange{QuName: "q"})
	if err := producer.PublishDelay(context.Background(), "body", 0); err == nil {
		t.Fatal("non-positive ttl must return an error")
	}
}

// TestRabbitMQIntegration 验证真实 RabbitMQ 的发布与消费链路。
// 设置 GO_BLACKBOX_RABBITMQ_DNS（如 amqp://guest:***@127.0.0.1:5672/）后才会执行。
func TestRabbitMQIntegration(t *testing.T) {
	dns := os.Getenv("GO_BLACKBOX_RABBITMQ_DNS")
	if dns == "" {
		t.Skip("RabbitMQ integration test requires GO_BLACKBOX_RABBITMQ_DNS environment variable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := Dial(dns, WithReconnect(2*time.Second, 3))
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if conn.State() != StateConnected {
		t.Fatalf("unexpected state after dial: %s", conn.State())
	}

	queue := QueueExchange{
		QuName: "go-blackbox.test.queue",
		RtKey:  "go-blackbox.test.queue",
		ExName: DefaultExchangeName,
		ExType: DefaultExchangeType,
		Dns:    dns,
	}

	// 发布一条消息
	producer := NewProducer(conn, queue)
	if err := producer.Publish(ctx, map[string]string{"source": "integration-test"}); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// 消费验证
	received := make(chan string, 1)
	receiver := &channelReceiver{received: received}
	consumer, err := NewConsumer(conn, queue, receiver)
	if err != nil {
		t.Fatalf("create consumer failed: %v", err)
	}

	consumeCtx, consumeCancel := context.WithTimeout(ctx, 10*time.Second)
	defer consumeCancel()
	go func() {
		_ = consumer.Start(consumeCtx)
	}()

	select {
	case body := <-received:
		if body == "" {
			t.Fatal("received empty message")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for message consumption")
	}
}

// testReceiver 是参数校验测试使用的空实现。
type testReceiver struct{}

// Consumer 空实现。
func (testReceiver) Consumer([]byte) error { return nil }

// FailAction 空实现。
func (testReceiver) FailAction(error, []byte) error { return nil }

// channelReceiver 通过 channel 透传消费内容。
type channelReceiver struct {
	received chan string
}

// Consumer 解析消息并发送到 channel。
func (r *channelReceiver) Consumer(body []byte) error {
	r.received <- string(body)
	return nil
}

// FailAction 空实现。
func (r *channelReceiver) FailAction(err error, _ []byte) error { return err }
