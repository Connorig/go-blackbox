// Package kafka 提供 Kafka 消息队列集成(对标 Spring Kafka 的 KafkaTemplate):
// 生产/消费封装 + 原生客户端暴露。基于纯 Go 的 segmentio/kafka-go。
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// Config Kafka 客户端配置。
type Config struct {
	// Brokers 节点列表(如 127.0.0.1:9092)。
	Brokers []string
	// Topic 默认主题(可空,操作时显式指定)。
	Topic string
	// GroupID 消费组(Consumer 使用)。
	GroupID string
	// Timeout 网络超时(默认 10s)。
	Timeout time.Duration
}

// normalize 补齐默认值(不填充 brokers:空列表由调用方校验)。
func (c Config) normalize() Config {
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	return c
}

// Producer 生产端(对标 KafkaTemplate.send)。
type Producer struct {
	writer *kafka.Writer
	topic  string
}

// NewProducer 创建生产者。
func NewProducer(config Config) (*Producer, error) {
	cfg := config.normalize()
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka: brokers are required")
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.Hash{}, // 同 key 同分区(保序)
		RequiredAcks: kafka.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
	}
	return &Producer{writer: writer, topic: cfg.Topic}, nil
}

// Send 发送消息(key 为 nil 时按哈希均衡)。
func (p *Producer) Send(ctx context.Context, topic, key string, value []byte) error {
	if p == nil || p.writer == nil {
		return errors.New("kafka: producer is nil")
	}
	if topic == "" {
		topic = p.topic
	}
	if topic == "" {
		return errors.New("kafka: topic is required")
	}
	message := kafka.Message{Topic: topic, Value: value}
	if key != "" {
		message.Key = []byte(key)
	}
	return p.writer.WriteMessages(ctx, message)
}

// SendJSON 发送 JSON 消息。
func (p *Producer) SendJSON(ctx context.Context, topic, key string, value interface{}) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("kafka: marshal message: %w", err)
	}
	return p.Send(ctx, topic, key, body)
}

// Close 关闭生产者(刷盘)。
func (p *Producer) Close() error {
	if p != nil && p.writer != nil {
		return p.writer.Close()
	}
	return nil
}

// Writer 返回原生 Writer(高级操作入口)。
func (p *Producer) Writer() *kafka.Writer {
	if p == nil {
		return nil
	}
	return p.writer
}

// Message 消费消息。
type Message struct {
	Topic     string
	Key       string
	Value     []byte
	Partition int
	Offset    int64
	Time      time.Time
	Headers   map[string]string
}

// Handler 消息处理函数;返回 error 时消息被标记失败(不提交偏移)。
type Handler func(ctx context.Context, message Message) error

// Consumer 消费端(对标 @KafkaListener)。
type Consumer struct {
	reader  *kafka.Reader
	topic   string
	groupID string
}

// NewConsumer 创建消费者(同一消费组内分区均衡)。
func NewConsumer(config Config) (*Consumer, error) {
	cfg := config.normalize()
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka: brokers are required")
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	return &Consumer{reader: reader, topic: cfg.Topic, groupID: cfg.GroupID}, nil
}

// Consume 阻塞消费:循环拉取消息并调用 handler;ctx 取消时返回 nil。
// handler 返回 nil 后自动提交偏移(At-Least-Once 语义)。
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	if c == nil || c.reader == nil {
		return errors.New("kafka: consumer is nil")
	}
	if handler == nil {
		return errors.New("kafka: handler is nil")
	}
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // 正常停止
			}
			return fmt.Errorf("kafka: fetch message: %w", err)
		}
		headers := make(map[string]string, len(message.Headers))
		for _, header := range message.Headers {
			headers[header.Key] = string(header.Value)
		}
		msg := Message{
			Topic:     message.Topic,
			Key:       string(message.Key),
			Value:     message.Value,
			Partition: message.Partition,
			Offset:    message.Offset,
			Time:      message.Time,
			Headers:   headers,
		}
		if err := handler(ctx, msg); err != nil {
			return fmt.Errorf("kafka: handler failed: %w", err)
		}
		if err := c.reader.CommitMessages(ctx, message); err != nil {
			return fmt.Errorf("kafka: commit offset: %w", err)
		}
	}
}

// Close 关闭消费者。
func (c *Consumer) Close() error {
	if c != nil && c.reader != nil {
		return c.reader.Close()
	}
	return nil
}

// Reader 返回原生 Reader(高级操作入口)。
func (c *Consumer) Reader() *kafka.Reader {
	if c == nil {
		return nil
	}
	return c.reader
}
