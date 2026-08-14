package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// 消费重试上限。
const defaultMaxRetries = 3

// Consumer 消费指定队列并处理消息。
// 断线后自动等待连接重连并恢复消费；Context 取消后优雅停止。
type Consumer struct {
	conn       *Connection
	queue      QueueExchange
	receiver   Receiver
	maxRetries int
}

// ConsumerOption 是 Consumer 的可选配置。
type ConsumerOption func(*Consumer)

// WithConsumerMaxRetries 配置消息处理失败的重试次数上限（默认 3）。
func WithConsumerMaxRetries(maxRetries int) ConsumerOption {
	return func(consumer *Consumer) {
		if maxRetries >= 0 {
			consumer.maxRetries = maxRetries
		}
	}
}

// NewConsumer 创建消费者；参数校验失败返回错误。
func NewConsumer(conn *Connection, queue QueueExchange, receiver Receiver, options ...ConsumerOption) (*Consumer, error) {
	if conn == nil {
		return nil, errors.New("rabbitmq consumer: connection is nil")
	}
	if receiver == nil {
		return nil, errors.New("rabbitmq consumer: receiver is nil")
	}
	if queue.QuName == "" {
		return nil, errors.New("rabbitmq consumer: queue name is empty")
	}
	consumer := &Consumer{
		conn:       conn,
		queue:      queue,
		receiver:   receiver,
		maxRetries: defaultMaxRetries,
	}
	for _, option := range options {
		option(consumer)
	}
	return consumer, nil
}

// Start 消费队列并阻塞，直到 Context 取消或连接关闭。
// 消费循环内部处理断线重连（依赖 Connection 状态机），消息处理带 panic 恢复。
func (c *Consumer) Start(ctx context.Context) error {
	if c == nil {
		return errors.New("rabbitmq consumer: consumer is nil")
	}
	if ctx == nil {
		return errors.New("rabbitmq consumer: context is nil")
	}
	if err := c.conn.ensureConnected(ctx); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		err := c.runOnce(ctx)
		if err == nil {
			return nil
		}
		if c.conn.IsClosed() {
			return fmt.Errorf("rabbitmq consumer stopped: %w", err)
		}
		// 断线/信道异常：等待重连后重试
		log.Printf("rabbitmq consumer loop interrupted: %v, waiting for reconnect", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(c.conn.reconnectInterval):
		}
	}
}

// runOnce 执行一轮消费循环；信道或连接异常时返回错误。
func (c *Consumer) runOnce(ctx context.Context) error {
	channel, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer func() {
		_ = channel.Close()
	}()

	if err := declareQueueComponents(channel, c.queue); err != nil {
		return err
	}

	messages, err := channel.Consume(c.queue.QuName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume queue %q: %w", c.queue.QuName, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-messages:
			if !ok {
				return fmt.Errorf("consumer channel closed for queue %q", c.queue.QuName)
			}
			c.process(ctx, message)
		}
	}
}

// process 处理单条消息：panic 恢复 + 失败重试 + 单条确认。
func (c *Consumer) process(ctx context.Context, message amqp.Delivery) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("rabbitmq consumer panic recovered: %v", recovered)
			_ = message.Ack(false)
		}
	}()

	retryNums, ok := message.Headers["retry_nums"].(int32)
	if !ok {
		retryNums = 0
	}

	if err := c.receiver.Consumer(message.Body); err != nil {
		if retryNums < int32(c.maxRetries) {
			// 进入延时重试队列，到期后回到原队列
			log.Printf("rabbitmq message processing failed, entering retry queue: %v", err)
			c.retryMessage(ctx, message.Body, retryNums)
		} else {
			// 超过重试上限：交给最终失败处理
			if failErr := c.receiver.FailAction(err, message.Body); failErr != nil {
				log.Printf("rabbitmq final failure handler error: %v", failErr)
			}
		}
		// 已转重试/失败处理，确认并丢弃当前消息
		_ = message.Ack(false)
		return
	}
	// 单条确认，避免误确认其他消息
	if err := message.Ack(false); err != nil {
		log.Printf("rabbitmq ack failed: %v", err)
	}
}

// retryMessage 把失败消息送入重试队列（携带递增的 retry_nums 头）。
func (c *Consumer) retryMessage(ctx context.Context, body []byte, retryNums int32) {
	queue := c.queue
	oldRoutingKey := queue.RtKey
	oldExchangeName := queue.ExName
	if oldRoutingKey == "" || oldExchangeName == "" {
		oldRoutingKey = queue.QuName
	}

	// 重试队列使用独立名称，避免与主队列混淆
	queue.QuName = retryQueueName(queue.QuName)
	queue.RtKey = retryQueueName(queue.RtKey)
	if queue.RtKey == retryQueueName("") {
		queue.RtKey = queue.QuName
	}

	producer := NewProducer(c.conn, queue)
	if err := producer.PublishRetry(ctx, string(body), retryNums, oldRoutingKey, oldExchangeName); err != nil {
		log.Printf("rabbitmq publish retry message failed: %v", err)
	}
}

// declareQueueComponents 声明交换机、队列与绑定（幂等）。
func declareQueueComponents(channel *amqp.Channel, queue QueueExchange) error {
	if queue.ExName != "" {
		exchangeType := queue.ExType
		if exchangeType == "" {
			exchangeType = DefaultExchangeType
		}
		if err := channel.ExchangeDeclare(queue.ExName, exchangeType, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare exchange %q: %w", queue.ExName, err)
		}
	}
	if _, err := channel.QueueDeclare(queue.QuName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %q: %w", queue.QuName, err)
	}
	if queue.RtKey != "" && queue.ExName != "" {
		if err := channel.QueueBind(queue.QuName, queue.RtKey, queue.ExName, false, nil); err != nil {
			return fmt.Errorf("bind queue %q to %q: %w", queue.QuName, queue.ExName, err)
		}
	}
	return nil
}
