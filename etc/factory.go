package etc

import (
	"context"
	"github.com/Domingor/go-blackbox/server/cache"
	"gorm.io/gorm"
	"reflect"
)

/**
* @Author: Connor
* @Date:   23.3.23 11:39
* @Description: 自定义容器，同于全局存取个个服务实例：iris.application\gorm.db\redis.client
 */

var beanMap map[reflect.Type]reflect.Value

// GlobalContext 全局存区容器
type GlobalContext struct {
	Ctx context.Context
}

func init() {
	// 初始化加载 容器
	beanMap = make(map[reflect.Type]reflect.Value)
	// 获取全局上下文
	background := context.Background()
	Set(&GlobalContext{Ctx: background})
}

// Set 将struct类型对象放入容器中，只能传入指针-struct类型数据
func Set(beans ...any) {
	for i := range beans {
		_type := reflect.TypeOf(beans[i])
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

// GetContext 获取上下文
func GetContext() *GlobalContext {
	get := Get((*GlobalContext)(nil))
	return get
}

// GetCache 获取redis实例
func GetCache() cache.Rediser {
	get := Get((*cache.RedisCache)(nil))
	return get
}
