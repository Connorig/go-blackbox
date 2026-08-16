package mongodb

// 全局便捷入口:EnableMongoDB 注册后自动设置,业务直接 mongodb.Get() 获取。

var global *Client

// SetGlobal 设置全局实例(EnableMongoDB 时自动调用)。
func SetGlobal(instance *Client) { global = instance }

// Get 获取全局 MongoDB 客户端;未启用时返回 nil。
func Get() *Client { return global }
