package rabbitmq

import (
	"os"
	"testing"
)

// TestQueue2 验证生产者连接与发送；需要真实 RabbitMQ 地址，默认跳过。
// 设置 GO_BLACKBOX_RABBITMQ_DNS（如 amqp://guest:guest@127.0.0.1:5672/）后才会执行。
func TestQueue2(t *testing.T) {
	dns := os.Getenv("GO_BLACKBOX_RABBITMQ_DNS")
	if dns == "" {
		t.Skip("RabbitMQ integration test requires GO_BLACKBOX_RABBITMQ_DNS environment variable")
	}

	exchange := QueueExchange{
		QuName: "test.001.queue",
		RtKey:  "test.001.queue",
		ExName: DefaultExchangeName,
		ExType: DefaultExchangeType,
		Dns:    dns,
	}
	mq := NewMq(exchange)
	if err := mq.MqConnect(); err != nil {
		t.Fatalf("connect RabbitMQ failed: %v", err)
	}
	t.Cleanup(func() {
		if err := mq.CloseMqConnect(); err != nil {
			t.Errorf("close RabbitMQ connection failed: %v", err)
		}
	})

	message := MapTest{
		Name: "father",
		Age:  10,
		Child: MapChild{
			Name: "child",
			Age:  1,
		},
	}
	if err := mq.SendMsg(message); err != nil {
		t.Fatalf("send message failed: %v", err)
	}
}
