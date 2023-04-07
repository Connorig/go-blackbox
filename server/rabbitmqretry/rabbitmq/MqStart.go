package rabbitmq

import (
	"encoding/json"
	"fmt"
)

/**
* @Author: Connor
* @Date:   23.4.7 17:52
* @Description:
 */

type MapTest struct {
	Name  string   `json:"name"`
	Age   int      `json:"age"`
	Child MapChild `json:"child"`
}

type MapChild struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

var StartQueue *RabbitMQ

type TestReceive struct {
}

func (t *TestReceive) Consumer(byte []byte) error {

	var mapTest MapTest
	err := json.Unmarshal(byte, &mapTest)
	if err != nil {
		fmt.Sprintf("%s", err)
	}
	fmt.Printf("consumer %s\n", byte)

	fmt.Println(mapTest)

	return nil
}
func (t *TestReceive) FailAction(err error, byte []byte) error {
	fmt.Printf("oops!")
	fmt.Errorf("%s", err)
	fmt.Errorf("%s", byte)
	return nil
}

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
