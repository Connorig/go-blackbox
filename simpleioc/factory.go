package simpleioc

import (
	"context"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/cronjobs"
	"github.com/Domingor/go-blackbox/server/mongodb"
	"github.com/Domingor/go-blackbox/server/shutdown"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"reflect"
)

/**
* @Author: Connor
* @Date:   23.3.23 11:39
* @Description: 自定义容器，用于全局存取服务实例：iris.application\gorm.db\redis.client
 */

// 存储服务实例对象，单例模式。用于全局使用
var beanMap map[reflect.Type]reflect.Value

// GlobalContext 自定义封装全局上下文

type GlobalContext struct {
	// 上下文实例
	Ctx context.Context
}

// 初始化IOC容器
func init() {

	// 初始化加载IOC容器
	beanMap = make(map[reflect.Type]reflect.Value)

	// 获取全局上下文,设置全局上下文到容器
	Set(&GlobalContext{Ctx: shutdown.Context()})

	// 添加定时任务对象到容器-单例模式（用于启动定时任务）

	Set(cronjobs.CronInstance())
}

// Set 将struct类型对象放入容器中，只能传入指针 *struct类型数据
func Set(beans ...any) {
	// 根据指针类型存储
	for i := range beans {

		_type := reflect.TypeOf(beans[i])

		// 判断类型指针
		if !(_type.Kind() == reflect.Ptr && _type.Elem().Kind() == reflect.Struct) {
			panic("it is not struct pointer")
		}

		if _, ok := beanMap[reflect.ValueOf(beans[i]).Type()]; !ok {
			beanMap[reflect.ValueOf(beans[i]).Type()] = reflect.ValueOf(beans[i])
		}
	}
}

// Get 从容器中获取与方法参数类型一致的指针-struct对象
func Get[T any](bean T) T {
	if t := reflect.TypeOf(bean); !(t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct) {
		return bean
	}

	if beanPtr, ok := beanMap[reflect.TypeOf(bean)]; ok {
		return beanPtr.Interface().(T)
	}

	return bean
}

// GetDb 获取数据库实例
func GetDb() *gorm.DB {
	// (* T)(nil) 它返回nil指针或没有指针，但仍然为struct的所有字段分配内存。

	get := Get((*gorm.DB)(nil))
	return get
}

// GetContext 获取全局上下文
func GetContext() *GlobalContext {
	// 传入一个nil指针，类型为 GlobalContext
	get := Get((*GlobalContext)(nil))
	return get
}

// GetCache 获取redis实例
func GetCache() cache.Rediser {

	get := Get((*cache.RedisCache)(nil))
	return get
}

// GetCronJobInstance 获取定时任务实例
func GetCronJobInstance() *cron.Cron {

	get := Get((*cron.Cron)(nil))
	return get
}

// GetMongoDb 获取MongoDbClient
func GetMongoDb() *mongodb.Client {

	get := Get((*mongodb.Client)(nil))
	return get
}
