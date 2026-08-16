package mqtt

import (
	"os"
	"testing"
	"time"
)

// TestNewClientInvalid 非法配置报错。
func TestNewClientInvalid(t *testing.T) {
	if _, err := NewClient(Config{}, nil); err == nil {
		t.Fatal("empty broker must fail")
	}
	if _, err := NewClient(Config{Broker: "tcp://127.0.0.1:1", Timeout: 2 * time.Second}, nil); err == nil {
		t.Fatal("unreachable broker must fail")
	}
}

// TestGlobalGetter 全局便捷入口。
func TestGlobalGetter(t *testing.T) {
	if Get() != nil {
		t.Fatal("global must be nil before connect")
	}
}

// TestSubscribePublish 订阅/发布闭环(需真实 MQTT broker)。
func TestSubscribePublish(t *testing.T) {
	broker := os.Getenv("GO_BLACKBOX_MQTT_BROKER")
	if broker == "" {
		t.Skip("mqtt not configured: set GO_BLACKBOX_MQTT_BROKER to run")
	}
	received := make(chan []byte, 1)
	client, err := NewClient(Config{Broker: broker, Timeout: 10 * time.Second}, func(topic string, payload []byte) {
		select {
		case received <- payload:
		default:
		}
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	topic := "gbx/test/hello"
	if err := client.Subscribe(topic, 1, func(topic string, payload []byte) {
		select {
		case received <- payload:
		default:
		}
	}); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	if err := client.Publish(topic, 1, false, []byte("hello-mqtt")); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	select {
	case payload := <-received:
		if string(payload) != "hello-mqtt" {
			t.Fatalf("payload = %q", payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("message not received")
	}
}
