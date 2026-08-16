package kafka

// 全局便捷入口:业务装配 Producer/Consumer 后注册,直接 kafka.GetProducer() 获取。

var globalProducer *Producer
var globalConsumer *Consumer

// SetProducer 设置全局生产者(装配后调用)。
func SetProducer(producer *Producer) { globalProducer = producer }

// SetConsumer 设置全局消费者(装配后调用)。
func SetConsumer(consumer *Consumer) { globalConsumer = consumer }

// GetProducer 获取全局生产者;未注册返回 nil。
func GetProducer() *Producer { return globalProducer }

// GetConsumer 获取全局消费者;未注册返回 nil。
func GetConsumer() *Consumer { return globalConsumer }
