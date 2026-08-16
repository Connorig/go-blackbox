package rabbitmq

// 全局便捷入口:Dial 成功后自动设置,业务直接 mq.Get() 获取连接。

var global *Connection

// SetGlobal 设置全局连接(Dial 成功后自动调用;测试可手动设置)。
func SetGlobal(connection *Connection) { global = connection }

// Get 获取全局 RabbitMQ 连接;未连接时返回 nil。
func Get() *Connection { return global }
