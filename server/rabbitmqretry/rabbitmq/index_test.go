package rabbitmq

import (
	"testing"
	"time"
)

func TestQueue2(t *testing.T) {

	// 创建mq
	go MqStart()
	// 等待创建Mq操作对象，并监听队列
	time.Sleep(time.Second * 1)

	t.Log(StartQueue)

	go func() {
		for i := 0; i < 2000; i++ {

			maps := MapTest{
				Name: "father",
				Age:  (i + 1) * 10,
				Child: MapChild{
					Name: "child",
					Age:  i,
				},
			}
			//发送消息
			err := StartQueue.sendMsg(maps)

			StartQueue.sendMsg(maps)
			if err != nil {
				t.Errorf("发送消息失败 %s", err)
			}
		}
	}()

	time.Sleep(time.Minute * 5)
}

//func TestName(t *testing.T) {
//

//
//	go func() {
//
//	}()
//
//	time.Sleep(time.Minute * 5)
//}
//
////测试发送
//func SendTest(queueName string, body interface{}) {
//
//	//exchange := QueueExchange{
//	//	QuName: queueName,
//	//	RtKey:  queueName,
//	//	ExName: "",
//	//	ExType: "",
//	//	Dns:    "",
//	//}
//
//}
//
//import (
//	"fmt"
//	"testing"
//	"time"
//)
//
///**
//* @Author: Connor
//* @Date:   23.4.7 15:10
//* @Description:
// */
//
//func TestName(t *testing.T) {
//
//	go func() {
//
//		for i := 0; i < 2000; i++ {
//			time.Sleep(time.Second)
//			body := fmt.Sprintf("%d =>", i)
//
//			t.Log("producer", body)
//
//			_ = Send(QueueExchange{
//				"a_test_0001",
//				"a_test_0001",
//				"hello_go",
//				"direct",
//				"amqp://guest:guest@127.0.0.1:5673/",
//			}, body)
//		}
//	}()
//
//	go func() {
//		processTask := &TestListener{}
//
//		err := Recv(QueueExchange{
//			"a_test_0001",
//			"a_test_0001",
//			"hello_go",
//			"direct",
//			"amqp://guest:guest@127.0.0.1:5673/",
//		},
//			processTask, 1, 1)
//		if err != nil {
//			fmt.Println(err)
//		}
//	}()
//
//	time.Sleep(time.Minute * 10)
//}
//
//type TestListener struct {
//}
//
//func (t *TestListener) Consumer(byte []byte) error {
//
//	fmt.Printf("consumer %s\n", byte)
//	return nil
//}
//func (t *TestListener) FailAction(err error, byte []byte) error {
//	fmt.Printf("oops!")
//	fmt.Errorf("%s", err)
//	fmt.Errorf("%s", byte)
//	return nil
//}
