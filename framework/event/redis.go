package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisBridge 通过 Redis Pub/Sub 桥接多个实例的进程内事件总线。
// 场景:多实例部署时,实例 A 发布的业务事件可被实例 B 的订阅者接收
// (如分布式缓存失效通知、跨实例业务联动)。
// 注意:事件负载必须可 JSON 序列化;跨实例收到的 data 类型为 map[string]interface{}。
type RedisBridge struct {
	client  *redis.Client
	channel string
	bus     *Bus
}

// NewRedisBridge 创建桥接器。
// client 为 go-redis 客户端;channel 为 Redis 频道名(多环境隔离用不同频道);
// bus 为本地事件总线(同步/异步均可,桥接按本地语义投递)。
func NewRedisBridge(client *redis.Client, channel string, bus *Bus) *RedisBridge {
	return &RedisBridge{client: client, channel: channel, bus: bus}
}

// Start 订阅 Redis 频道并把远端事件投递到本地总线,阻塞直到 ctx 取消。
func (b *RedisBridge) Start(ctx context.Context) error {
	if b == nil {
		return errors.New("eventbus bridge: bridge is nil")
	}
	if b.client == nil || b.bus == nil {
		return errors.New("eventbus bridge: client or bus is nil")
	}
	if ctx == nil {
		return errors.New("eventbus bridge: context is nil")
	}
	subscription := b.client.Subscribe(ctx, b.channel)
	defer subscription.Close()
	channel := subscription.Channel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-channel:
			if !ok {
				return errors.New("eventbus bridge: subscription channel closed")
			}
			event, err := decodeEvent([]byte(message.Payload))
			if err != nil {
				// 单条消息损坏不影响桥接
				continue
			}
			_ = b.bus.Publish(ctx, event)
		}
	}
}

// Publish 把本地事件发布到 Redis 频道(其他实例的桥接器会转发到各自本地总线)。
// 本地订阅者如需同时收到事件,请额外调用 bus.Publish。
func (b *RedisBridge) Publish(ctx context.Context, event Event) error {
	if b == nil {
		return errors.New("eventbus bridge: bridge is nil")
	}
	if b.client == nil {
		return errors.New("eventbus bridge: client is nil")
	}
	if event.Name == "" {
		return errors.New("eventbus bridge: event name is empty")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("eventbus bridge: marshal event %q: %w", event.Name, err)
	}
	if err := b.client.Publish(ctx, b.channel, payload).Err(); err != nil {
		return fmt.Errorf("eventbus bridge: publish event %q: %w", event.Name, err)
	}
	return nil
}

// decodeEvent 反序列化事件负载。
func decodeEvent(payload []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, err
	}
	return event, nil
}
