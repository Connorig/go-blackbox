package cache

// 全局便捷入口:EnableCache 注册后自动设置,业务直接 cache.Get() 获取,
// 无需每次 gbxioc.GetBean[cache.RedisCache]()。GetBean 保留给业务自定义对象。

var global *RedisCache

// SetGlobal 设置全局实例(EnableCache 时自动调用;测试可手动设置)。
func SetGlobal(instance *RedisCache) { global = instance }

// Get 获取全局 RedisCache 实例;未启用缓存时返回 nil。
// 业务直接调用,等价于 gbxioc.GetBean[cache.RedisCache]()。
func Get() *RedisCache { return global }
