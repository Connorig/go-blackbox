package rabbitmq

import (
	"encoding/json"
	"fmt"
	"log"
)

/**
* @Author: Connor
* @Date:   23.4.7 17:52
* @Description:
 */

// MapTest 是 RabbitMQ 演示消费者解析的消息结构。
type MapTest struct {
	Name  string   `json:"name"`
	Age   int      `json:"age"`
	Child MapChild `json:"child"`
}

// MapChild 是演示消息中的嵌套子对象。
type MapChild struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// StartQueue 保存演示入口创建的 RabbitMQ 客户端。
var StartQueue *RabbitMQ

// TestReceive 是演示消费者，用于验证消息解析和最终失败处理。
type TestReceive struct {
}

// Consumer 解析单条演示消息；JSON 无效时返回包装错误并交由重试流程处理。
func (t *TestReceive) Consumer(messageBody []byte) error {
	var mapTest MapTest
	if err := json.Unmarshal(messageBody, &mapTest); err != nil {
		return fmt.Errorf("decode RabbitMQ demo message: %w", err)
	}
	log.Printf("RabbitMQ demo message consumed, name=%s, age=%d", mapTest.Name, mapTest.Age)
	return nil
}

// FailAction 返回消息超过重试上限后的最终失败错误。
// 错误只记录消息长度，不输出可能包含敏感字段的完整消息体。
func (t *TestReceive) FailAction(consumeErr error, messageBody []byte) error {
	if consumeErr == nil {
		return fmt.Errorf("RabbitMQ demo message failed without original error, message_length=%d", len(messageBody))
	}
	return fmt.Errorf("RabbitMQ demo message exceeded retry limit, message_length=%d: %w", len(messageBody), consumeErr)
}

// MqStart 创建演示 RabbitMQ 客户端并开始消费固定测试队列。
// 该函数依赖本地 RabbitMQ 服务，仅用于显式运行的集成演示。
func MqStart() (err error) {
	configQueue := "test.001.queue"
	configDns := "amqp://guest:guest@127.0.0.1:5673/"

	rec := TestReceive{}

	exchange := QueueExchange{
		QuName: configQueue,
		RtKey:  configQueue,
		ExName: DefaultExchangeName,
		ExType: DefaultExchangeType,
		Dns:    configDns,
	}

	// 初始化发送端
	mq := NewMq(exchange)
	StartQueue = &mq
	// 获取连接
	StartQueue.MqConnect()

	// 初始化接收端&开始监听
	err = ReceiveMsg(exchange, &rec)
	return
}
