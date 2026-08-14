package simpleioc

import (
	"context"
	"errors"
	"reflect"

	"github.com/Connorig/go-blackbox/server/cache"
	"github.com/Connorig/go-blackbox/server/mongodb"
	"github.com/Connorig/go-blackbox/server/shutdown"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// defaultContainer 是进程级默认容器，泛型 API（api.go）均为其薄封装。
var defaultContainer = NewContainer()

// GlobalContext 自定义封装全局上下文
type GlobalContext struct {
	// 上下文实例
	Ctx context.Context
}

// init 注册进程级全局上下文，保持既有调用方 GetContext 行为。
// 定时任务实例由 InitCronJob 显式注册，不再在包初始化时隐式创建。
func init() {
	_ = RegisterInstance(&GlobalContext{Ctx: shutdown.Context()})
}

// Set 将 struct 类型对象放入容器中（仅接受 *struct）。
// Deprecated: 新代码使用 Register / RegisterInstance。
// 保留旧语义：重复注册忽略；非结构体指针 panic。
func Set(beans ...any) {
	for i := range beans {
		bean := beans[i]
		beanType := reflect.TypeOf(bean)
		if beanType == nil || beanType.Kind() != reflect.Ptr || beanType.Elem().Kind() != reflect.Struct {
			panic("it is not struct pointer")
		}
		if err := defaultContainer.register(beanType, "", ScopeSingleton, nil, bean); err != nil {
			if errors.Is(err, ErrInvalidType) {
				panic("it is not struct pointer")
			}
			// 重复注册保持旧语义：静默忽略
		}
	}
}

// Get 从容器中获取与参数类型一致的 *struct 对象。
// Deprecated: 新代码使用泛型 Get[T]() 并处理错误。
// 保留旧语义：未注册或类型不合法时返回原参数值。
func Get[T any](bean T) T {
	if t := reflect.TypeOf(bean); !(t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct) {
		return bean
	}
	instance, err := compatGet(reflect.TypeOf(bean))
	if err != nil {
		return bean
	}
	return instance.(T)
}

// GetDb 获取数据库实例
// Deprecated: 使用 Get[*gorm.DB]()。
func GetDb() *gorm.DB {
	instance, err := compatGet(reflect.TypeOf((*gorm.DB)(nil)))
	if err != nil {
		return nil
	}
	return instance.(*gorm.DB)
}

// GetContext 获取全局上下文
// Deprecated: 使用 Get[*GlobalContext]()。
func GetContext() *GlobalContext {
	instance, err := compatGet(reflect.TypeOf((*GlobalContext)(nil)))
	if err != nil {
		return nil
	}
	return instance.(*GlobalContext)
}

// GetCache 获取redis实例
// Deprecated: 使用 Get[*cache.RedisCache]()。
func GetCache() cache.Rediser {
	instance, err := compatGet(reflect.TypeOf((*cache.RedisCache)(nil)))
	if err != nil {
		return nil
	}
	return instance.(*cache.RedisCache)
}

// GetCronJobInstance 获取定时任务实例
// Deprecated: 使用 Get[*cron.Cron]()。
func GetCronJobInstance() *cron.Cron {
	instance, err := compatGet(reflect.TypeOf((*cron.Cron)(nil)))
	if err != nil {
		return nil
	}
	return instance.(*cron.Cron)
}

// GetMongoDb 获取MongoDbClient
// Deprecated: 使用 Get[*mongodb.Client]()。
func GetMongoDb() *mongodb.Client {
	instance, err := compatGet(reflect.TypeOf((*mongodb.Client)(nil)))
	if err != nil {
		return nil
	}
	return instance.(*mongodb.Client)
}
