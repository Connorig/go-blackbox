package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Producer 发送消息到指定交换机/队列。
// 每次发送使用独立信道，发送后立即关闭，避免信道泄漏。
type Producer struct {
	conn  *Connection
	queue QueueExchange
}

// NewProducer 创建生产者。
func NewProducer(conn *Connection, queue QueueExchange) *Producer {
	return &Producer{conn: conn, queue: queue}
}

// Publish 发送普通消息；body 会被 JSON 序列化并标记持久化。
func (p *Producer) Publish(ctx context.Context, body interface{}) error {
	if p == nil || p.conn == nil {
		return errors.New("rabbitmq producer: connection is nil")
	}
	if ctx == nil {
		return errors.New("rabbitmq producer: context is nil")
	}
	if err := p.conn.ensureConnected(ctx); err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal rabbitmq message: %w", err)
	}

	channel, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer func() {
		_ = channel.Close()
	}()

	if err := declareQueueComponents(channel, p.queue); err != nil {
		return err
	}

	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         payload,
		DeliveryMode: amqp.Persistent,
	}
	if p.queue.ExName != "" && p.queue.RtKey != "" {
		err = channel.PublishWithContext(ctx, p.queue.ExName, p.queue.RtKey, false, false, publishing)
	} else {
		err = channel.PublishWithContext(ctx, "", p.queue.QuName, false, false, publishing)
	}
	if err != nil {
		return fmt.Errorf("publish rabbitmq message to %q: %w", p.queue.QuName, err)
	}
	return nil
}

// PublishDelay 发送延时消息：消息先进入带 TTL 的延时队列，到期后经死信路由回到原队列。
func (p *Producer) PublishDelay(ctx context.Context, body string, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("publish delay: ttl must be positive")
	}
	channel, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer func() {
		_ = channel.Close()
	}()

	if p.queue.ExName != "" {
		if err := declareQueueComponents(channel, p.queue); err != nil {
			return err
		}
	}

	ttlMillis := int64(ttl / time.Millisecond)
	delayQueue := fmt.Sprintf("%s_delay_%d", p.queue.QuName, ttlMillis)
	delayRoutingKey := fmt.Sprintf("%s_delay_%d", p.queue.QuName, ttlMillis)
	arguments := map[string]interface{}{
		"x-dead-letter-routing-key": p.queue.RtKey,
		"x-dead-letter-exchange":    p.queue.ExName,
		"x-message-ttl":             ttlMillis,
	}
	if _, err := channel.QueueDeclare(delayQueue, true, false, false, false, arguments); err != nil {
		return fmt.Errorf("declare delay queue %q: %w", delayQueue, err)
	}
	if p.queue.ExName != "" {
		if err := channel.QueueBind(delayQueue, delayRoutingKey, p.queue.ExName, false, nil); err != nil {
			return fmt.Errorf("bind delay queue %q: %w", delayQueue, err)
		}
	}

	publishing := amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(body),
		Headers:     amqp.Table{"retry_nums": int32(0)},
	}
	exchange := p.queue.ExName
	routingKey := delayRoutingKey
	if exchange == "" {
		routingKey = delayQueue
	}
	if err := channel.PublishWithContext(ctx, exchange, routingKey, false, false, publishing); err != nil {
		return fmt.Errorf("publish delay message: %w", err)
	}
	return nil
}

// PublishRetry 发送重试消息：进入带死信配置的重试队列，TTL 到期后回到原队列。
// oldRoutingKey / oldExchangeName 是原队列的绑定信息，用于死信回投。
func (p *Producer) PublishRetry(ctx context.Context, body string, retryNums int32, oldRoutingKey, oldExchangeName string) error {
	channel, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer func() {
		_ = channel.Close()
	}()

	if p.queue.ExName != "" {
		if err := declareQueueComponents(channel, p.queue); err != nil {
			return err
		}
	}

	arguments := map[string]interface{}{
		"x-dead-letter-routing-key": oldRoutingKey,
		"x-dead-letter-exchange":    oldExchangeName,
		"x-message-ttl":             int64(20000), // 20 秒后回到原队列
	}
	if _, err := channel.QueueDeclare(p.queue.QuName, true, false, false, false, arguments); err != nil {
		return fmt.Errorf("declare retry queue %q: %w", p.queue.QuName, err)
	}
	if p.queue.RtKey != "" && p.queue.ExName != "" {
		if err := channel.QueueBind(p.queue.QuName, p.queue.RtKey, p.queue.ExName, false, nil); err != nil {
			return fmt.Errorf("bind retry queue %q: %w", p.queue.QuName, err)
		}
	}

	publishing := amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(body),
		Headers:     amqp.Table{"retry_nums": retryNums + 1},
	}
	exchange := p.queue.ExName
	routingKey := p.queue.RtKey
	if exchange == "" {
		routingKey = p.queue.QuName
	}
	if err := channel.PublishWithContext(ctx, exchange, routingKey, false, false, publishing); err != nil {
		return fmt.Errorf("publish retry message: %w", err)
	}
	log.Printf("rabbitmq retry message published, queue=%s retry_nums=%d", p.queue.QuName, retryNums+1)
	return nil
}

// retryQueueName 生成重试队列名称（兼容旧版 _retry_3 命名）。
func retryQueueName(queueName string) string {
	return queueName + "_retry_3"
}
