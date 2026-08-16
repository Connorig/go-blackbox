package appbox

import (
	"github.com/Connorig/go-blackbox/framework/cache"
	"github.com/Connorig/go-blackbox/framework/cron"
	"github.com/Connorig/go-blackbox/framework/database"
	"github.com/Connorig/go-blackbox/framework/es"
	"github.com/Connorig/go-blackbox/framework/httpclient"
	"github.com/Connorig/go-blackbox/framework/influx"
	"github.com/Connorig/go-blackbox/framework/kafka"
	"github.com/Connorig/go-blackbox/framework/mail"
	"github.com/Connorig/go-blackbox/framework/mongo"
	"github.com/Connorig/go-blackbox/framework/mq"
	"github.com/Connorig/go-blackbox/framework/mqtt"
	"github.com/Connorig/go-blackbox/framework/sms"
	"github.com/Connorig/go-blackbox/framework/storage"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// 资源门面(Facade):业务统一从这里获取已装配的资源实例,
// 无需记忆各模块的获取方式,也不用直接操作容器。
// 内部转发各模块的便捷入口(Enable*/New 成功时自动设置)。
//
// 业务自定义对象(service 等)仍通过 gbxioc.GetBean[T]() 获取。
//
//	// 业务代码示例(只 import appbox + 类型包):
//	db := appbox.DB()          // *gorm.DB
//	redis := appbox.Cache()    // *cache.RedisCache
//	producer := appbox.KafkaProducer()
//
// 未启用对应模块时返回 nil(不 panic),调用方自行判断或确保装配顺序。

// Datasource 获取数据库实例(事务/多实例操作入口);未启用返回 nil。
func Datasource() *datasource.Instance {
	instance, _ := datasource.Get()
	return instance
}

// NamedDatasource 获取具名数据库实例;未注册返回 nil。
func NamedDatasource(name string) *datasource.Instance {
	instance, _ := datasource.GetNamed(name)
	return instance
}

// DB 获取数据库 GORM 句柄(查询便捷入口);未启用返回 nil。
func DB() *gorm.DB {
	return GormDb()
}

// Cache 获取 Redis 缓存实例;未启用返回 nil。
func Cache() *cache.RedisCache {
	return cache.Get()
}

// MongoDB 获取 MongoDB 客户端;未启用返回 nil。
func MongoDB() *mongodb.Client {
	return mongodb.Get()
}

// MQ 获取 RabbitMQ 连接;未连接返回 nil。
func MQ() *rabbitmq.Connection {
	return rabbitmq.Get()
}

// KafkaProducer 获取 Kafka 生产者;未注册返回 nil。
func KafkaProducer() *kafka.Producer {
	return kafka.GetProducer()
}

// KafkaConsumer 获取 Kafka 消费者;未注册返回 nil。
func KafkaConsumer() *kafka.Consumer {
	return kafka.GetConsumer()
}

// Cron 获取定时任务调度器;未启用返回 nil。
func Cron() *cron.Cron {
	return cronjobs.GetCron()
}

// ES 获取 ElasticSearch 客户端;未初始化返回 nil。
func ES() *es.Client {
	return es.Get()
}

// Storage 获取对象存储客户端;未初始化返回 nil。
func Storage() *storage.Client {
	return storage.Get()
}

// Influx 获取 InfluxDB 客户端;未初始化返回 nil。
func Influx() *influx.Client {
	return influx.Get()
}

// SMS 获取短信客户端;未初始化返回 nil。
func SMS() *sms.Client {
	return sms.Get()
}

// Mail 获取邮件客户端;未初始化返回 nil。
func Mail() *email.Client {
	return email.Get()
}

// MQTT 获取全局 MQTT 客户端(设备网关采集);未连接返回 nil。
func MQTT() *mqtt.Client {
	return mqtt.Get()
}

// HTTPClient 获取全局 HTTP 请求工具客户端;未初始化返回 nil。
func HTTPClient() *httpclient.Client {
	return httpclient.Get()
}
